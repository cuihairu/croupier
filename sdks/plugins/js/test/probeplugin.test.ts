/** Unit tests for the probe contract mapping in src/probeplugin.ts. */

import {
  COOKIE_KEY,
  COOKIE_VALUE,
  PROTOCOL_VERSION,
  PROTOCOL_VERSIONS_KEY,
  bindTcp,
  controllerHandlers,
  failResult,
  healthHandlers,
  makeProbeCall,
  nowMs,
  outcomeToProto,
  passResult,
  probeHandlers,
} from "../src";
import * as grpc from "@grpc/grpc-js";
import * as path from "node:path";
import { loadProtos } from "../src";

describe("result constructors", () => {
  it("passResult defaults status/latency and clears error", () => {
    expect(passResult()).toEqual({
      ok: true,
      status: 0,
      latencyMs: 0,
      error: "",
      detail: "",
      atUnixMs: undefined,
    });
  });

  it("passResult carries status, latency and detail", () => {
    const got = passResult({ status: 200, latencyMs: 12, detail: "d" });
    expect(got.ok).toBe(true);
    expect(got.status).toBe(200);
    expect(got.latencyMs).toBe(12);
    expect(got.detail).toBe("d");
  });

  it("failResult requires the error and reports ok=false", () => {
    const got = failResult("synthetic probe failure", { status: 500 });
    expect(got.ok).toBe(false);
    expect(got.error).toBe("synthetic probe failure");
    expect(got.status).toBe(500);
  });
});

describe("outcomeToProto", () => {
  it("defaults atUnixMs to now", () => {
    const before = nowMs();
    const got = outcomeToProto(passResult());
    const after = nowMs();
    expect(got.atUnixMs).toBeGreaterThanOrEqual(before);
    expect(got.atUnixMs).toBeLessThanOrEqual(after);
  });

  it("keeps an explicit atUnixMs", () => {
    expect(outcomeToProto(passResult({ atUnixMs: 1_700_000_000_000 })).atUnixMs).toBe(
      1_700_000_000_000,
    );
  });

  it("fills every wire field even for a bare outcome", () => {
    expect(outcomeToProto({ ok: false, error: "boom" })).toEqual({
      ok: false,
      status: 0,
      latencyMs: 0,
      error: "boom",
      detail: "",
      atUnixMs: expect.any(Number),
    });
  });
});

describe("makeProbeCall", () => {
  const past = new Date(Date.now() - 1000);
  const future = new Date(Date.now() + 60_000);

  it("maps snake_case wire fields onto the camelCase call view", () => {
    const call = makeProbeCall(
      { target: "ok-target", timeoutMs: 1500, payloadJson: '{"sleep_ms":5}' },
      { getDeadline: () => future },
    );
    expect(call.target).toBe("ok-target");
    expect(call.timeoutMs).toBe(1500);
    expect(call.payloadJson).toBe('{"sleep_ms":5}');
    expect(call.isCancelled()).toBe(false);
  });

  it("defaults missing wire fields", () => {
    const call = makeProbeCall(
      {} as { target: string; timeoutMs: number; payloadJson: string },
      {},
    );
    expect(call.target).toBe("");
    expect(call.timeoutMs).toBe(0);
    expect(call.payloadJson).toBe("");
  });

  it("reports cancelled once the host deadline is past", () => {
    const call = makeProbeCall(
      { target: "t", timeoutMs: 0, payloadJson: "" },
      { getDeadline: () => past },
    );
    expect(call.isCancelled()).toBe(true);
    expect(call.remainingMs()).toBe(0);
  });

  it("survives a call surface without a deadline (stays live)", () => {
    const call = makeProbeCall(
      { target: "t", timeoutMs: 0, payloadJson: "" },
      {},
    );
    expect(call.isCancelled()).toBe(false);
    expect(call.remainingMs()).toBe(Number.POSITIVE_INFINITY);
  });

  it("honours an explicit cancelled flag", () => {
    const call = makeProbeCall(
      { target: "t", timeoutMs: 0, payloadJson: "" },
      { getDeadline: () => future, cancelled: true },
    );
    expect(call.isCancelled()).toBe(true);
  });
});

describe("handler maps", () => {
  const proto = loadProtos();
  const noopCallback = (): void => undefined;

  it("probeHandlers.Describe returns the plugin self-description", () => {
    const handlers = probeHandlers({
      describe: () => ({ name: "probe-echo", version: "1.0.0" }),
      check: () => passResult(),
    });
    let response: { name?: string; version?: string } | null = null;
    (handlers.Describe as (call: unknown, cb: (err: null, res: { name?: string; version?: string }) => void) => void)(
      {},
      (_err, res) => {
        response = res;
      },
    );
    expect(response).toEqual({ name: "probe-echo", version: "1.0.0" });
    expect(proto.probeService).toBeDefined();
  });

  it("probeHandlers.Check maps an async outcome onto the wire result", async () => {
    const handlers = probeHandlers({
      describe: () => ({ name: "probe-echo", version: "1.0.0" }),
      check: async () => failResult("synthetic probe failure", { status: 500, detail: "d" }),
    });
    await new Promise<void>((resolve) => {
      (
        handlers.Check as (
          call: grpc.ServerUnaryCall<unknown, unknown>,
          cb: (err: null, res: Record<string, unknown>) => void,
        ) => void
      )(
        { request: { target: "fail-target", timeoutMs: 10, payloadJson: "" } } as unknown as grpc.ServerUnaryCall<unknown, unknown>,
        (_err, res) => {
          expect(res.ok).toBe(false);
          expect(res.status).toBe(500);
          expect(res.detail).toBe("d");
          resolve();
        },
      );
    });
  });

  it("healthHandlers.Check answers SERVING", () => {
    const handlers = healthHandlers();
    (handlers.Check as (call: unknown, cb: (err: null, res: { status: number }) => void) => void)(
      {},
      (_err, res) => {
        expect(res.status).toBe(proto.servingStatus);
      },
    );
  });

  it("controllerHandlers.Shutdown replies first, then stops", async () => {
    let stopped = 0;
    const handlers = controllerHandlers(() => {
      stopped += 1;
    });
    await new Promise<void>((resolve) => {
      (handlers.Shutdown as (call: unknown, cb: (err: null, res: unknown) => void) => void)(
        {},
        () => resolve(),
      );
    });
    // The reply must already be out before the teardown starts.
    expect(stopped).toBe(0);
    await new Promise((resolve) => {
      setTimeout(resolve, 120);
    });
    expect(stopped).toBe(1);
    expect(noopCallback).toBeDefined();
  });

  it("keeps the contract constants in sync with the Python SDK", () => {
    expect(COOKIE_KEY).toBe("CROUPIER_SYNTHETICS_PLUGIN");
    expect(COOKIE_VALUE).toBe("croupier-synthetics-probe-plugin");
    expect(PROTOCOL_VERSION).toBe(1);
    expect(PROTOCOL_VERSIONS_KEY).toBe("PLUGIN_PROTOCOL_VERSIONS");
  });

  it("loads all four service definitions and the SERVING status", () => {
    const modules = loadProtos(path.join(__dirname, "..", "protos"));
    expect(Object.keys(modules.probeService).sort()).toEqual(["Check", "Describe"]);
    expect(Object.keys(modules.stdioService)).toEqual(["StreamStdio"]);
    expect(Object.keys(modules.controllerService)).toEqual(["Shutdown"]);
    expect(Object.keys(modules.healthService).sort()).toEqual(["Check", "Watch"]);
    expect(modules.servingStatus).toBe(1);
  });
});

describe("bindTcp", () => {
  it("binds loopback and reports the bound port", async () => {
    const server = new grpc.Server();
    try {
      const bound = await bindTcp(server, "127.0.0.1:0");
      expect(bound.network).toBe("tcp");
      const port = Number(bound.addr.split(":")[1]);
      expect(port).toBeGreaterThan(0);
    } finally {
      server.forceShutdown();
    }
  });
});
