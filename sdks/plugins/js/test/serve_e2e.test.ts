/**
 * In-process end-to-end test of the full go-plugin local protocol: cookie
 * gate, handshake line, health service, probe service, controller shutdown
 * and bounded termination — everything except the process boundary, which
 * test/integration_go_host.test.ts covers against the real Go host.
 */

import * as grpc from "@grpc/grpc-js";
import * as path from "node:path";
import {
  COOKIE_KEY,
  COOKIE_VALUE,
  PluginHandshakeError,
  failResult,
  loadProtos,
  passResult,
  serve,
} from "../src";
import type { CheckResultShape } from "../src";

/** Minimal typed client surface the test exercises. */
interface ProbeLike {
  describe(
    request: Record<string, never>,
    callback: (err: grpc.ServiceError | null, res?: { name: string; version: string }) => void,
  ): void;
  check(
    request: { target: string; timeoutMs: number; payloadJson: string },
    options: { deadline?: grpc.Deadline },
    callback: (err: grpc.ServiceError | null, res?: CheckResultShape) => void,
  ): void;
  close(): void;
}

interface HealthLike {
  check(
    request: { service: string },
    callback: (err: grpc.ServiceError | null, res?: { status: number }) => void,
  ): void;
  close(): void;
}

interface ControllerLike {
  shutdown(
    request: Record<string, never>,
    callback: (err: grpc.ServiceError | null, res?: Record<string, never>) => void,
  ): void;
  close(): void;
}

const HOST_ENV: Record<string, string> = {
  [COOKIE_KEY]: COOKIE_VALUE,
  PLUGIN_PROTOCOL_VERSIONS: "1",
};

const echoPlugin = {
  describe: () => ({ name: "probe-echo", version: "1.0.0" }),
  check: (call: { target: string }) =>
    call.target.startsWith("fail")
      ? failResult("synthetic probe failure", {
          status: 500,
          detail: `probe-echo checked ${call.target}`,
        })
      : passResult({ status: 200, detail: `probe-echo checked ${call.target}` }),
};

describe("serve() refuses to run outside a host", () => {
  it("rejects with a handshake error when the cookie is missing", async () => {
    await expect(serve(echoPlugin, { env: {} })).rejects.toThrow(
      PluginHandshakeError,
    );
  });

  it("rejects with a handshake error when the host version list excludes us", async () => {
    await expect(
      serve(echoPlugin, {
        env: { ...HOST_ENV, PLUGIN_PROTOCOL_VERSIONS: "9,10" },
      }),
    ).rejects.toThrow(PluginHandshakeError);
  });
});

describe("serve() full protocol round-trip", () => {
  let writeSpy: jest.SpyInstance;
  let chunks: string[];

  beforeAll(() => {
    chunks = [];
    writeSpy = jest
      .spyOn(process.stdout, "write")
      .mockImplementation(((chunk: unknown) => {
        chunks.push(String(chunk));
        return true;
      }) as typeof process.stdout.write);
  });

  afterAll(() => {
    writeSpy.mockRestore();
  });

  it("serves and shuts down behind the real handshake", async () => {
    const served = serve(echoPlugin, { env: HOST_ENV });

    // 1. exactly one handshake line on stdout, 1|1|tcp|127.0.0.1:PORT|grpc|
    const line = await waitForHandshakeLine();
    const parts = line.split("|");
    expect(parts).toHaveLength(6);
    expect(parts[0]).toBe("1");
    expect(parts[1]).toBe("1");
    expect(parts[2]).toBe("tcp");
    expect(parts[4]).toBe("grpc");
    expect(parts[5]).toBe("");
    const addr = parts[3];

    const creds = grpc.credentials.createInsecure();
    const proto = loadProtos(path.join(__dirname, "..", "protos"));
    const probe = new (grpc.makeClientConstructor(
      proto.probeService,
      "ProbePlugin",
    ) as unknown as new (
      address: string,
      credentials: grpc.ChannelCredentials,
    ) => ProbeLike)(addr, creds);
    const health = new (grpc.makeClientConstructor(
      proto.healthService,
      "Health",
    ) as unknown as new (
      address: string,
      credentials: grpc.ChannelCredentials,
    ) => HealthLike)(addr, creds);
    const controller = new (grpc.makeClientConstructor(
      proto.controllerService,
      "GRPCController",
    ) as unknown as new (
      address: string,
      credentials: grpc.ChannelCredentials,
    ) => ControllerLike)(addr, creds);

    try {
      // 2. health answers SERVING (go-plugin health-checks every plugin)
      await new Promise<void>((resolve, reject) => {
        health.check({ service: "" }, (err, res) => {
          if (err !== null || res === undefined) {
            reject(err ?? new Error("no health response"));
            return;
          }
          expect(res.status).toBe(proto.servingStatus);
          resolve();
        });
      });

      // 3. Describe returns the plugin self-description
      await new Promise<void>((resolve, reject) => {
        probe.describe({}, (err, res) => {
          if (err !== null || res === undefined) {
            reject(err ?? new Error("no describe response"));
            return;
          }
          expect(res).toEqual({ name: "probe-echo", version: "1.0.0" });
          resolve();
        });
      });

      // 4. Check against a passing target
      await new Promise<void>((resolve, reject) => {
        probe.check(
          { target: "ok-target", timeoutMs: 1000, payloadJson: "" },
          { deadline: new Date(Date.now() + 5000) },
          (err, res) => {
            if (err !== null || res === undefined) {
              reject(err ?? new Error("no check response"));
              return;
            }
            expect(res.ok).toBe(true);
            expect(res.status).toBe(200);
            expect(res.detail).toBe("probe-echo checked ok-target");
            expect(res.atUnixMs).toBeGreaterThan(0);
            resolve();
          },
        );
      });

      // 5. Check against a failing target
      await new Promise<void>((resolve, reject) => {
        probe.check(
          { target: "fail-target", timeoutMs: 1000, payloadJson: "" },
          { deadline: new Date(Date.now() + 5000) },
          (err, res) => {
            if (err !== null || res === undefined) {
              reject(err ?? new Error("no check response"));
              return;
            }
            expect(res.ok).toBe(false);
            expect(res.status).toBe(500);
            expect(res.error).toBe("synthetic probe failure");
            resolve();
          },
        );
      });

      // 6. GRPCController.Shutdown replies and terminates the server
      await new Promise<void>((resolve, reject) => {
        controller.shutdown({}, (err) => {
          if (err !== null) {
            reject(err);
            return;
          }
          resolve();
        });
      });
      await expect(served).resolves.toBeUndefined();
    } finally {
      probe.close();
      health.close();
      controller.close();
    }
  });

  function waitForHandshakeLine(): Promise<string> {
    return new Promise<string>((resolve, reject) => {
      const started = Date.now();
      const poll = (): void => {
        const text = chunks.join("");
        const match = text.match(/1\|1\|tcp\|127\.0\.0\.1:\d+\|grpc\|/);
        if (match !== null) {
          resolve(match[0]);
          return;
        }
        if (Date.now() - started > 10_000) {
          reject(new Error(`no handshake line appeared, stdout saw: ${text}`));
          return;
        }
        setTimeout(poll, 25);
      };
      poll();
    });
  }
});
