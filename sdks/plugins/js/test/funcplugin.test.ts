/** Tests for the function adapter contract in src/funcplugin.ts. */

// Imported from ../src/funcplugin (not the ../src barrel): the probe face
// owns the un-prefixed outcomeToProto/COOKIE_KEY exports, and mixing the two
// faces through the barrel silently maps func envelopes onto probe shapes.
import {
  BAD_RESPONSE,
  COOKIE_KEY,
  COOKIE_VALUE,
  ERROR_CODES,
  RISK_LEVELS,
  TARGET_REJECTED,
  TARGET_UNAVAILABLE,
  descriptorToProto,
  errorResult,
  funcHandlers,
  makeInvokeCall,
  okResult,
  outcomeToProto,
} from "../src/funcplugin";
import { FUNC_COOKIE_KEY, FUNC_COOKIE_VALUE } from "../src";
import type { FuncDescriptorShape } from "../src";

describe("envelope constructors", () => {
  it("okResult carries the result json", () => {
    const got = okResult('{"message":"hi"}');
    expect(got.ok).toBe(true);
    expect(got.resultJson).toBe('{"message":"hi"}');
    expect(got.errorCode).toBeUndefined();
  });

  it("errorResult accepts every closed-set code", () => {
    for (const code of ERROR_CODES) {
      expect(errorResult(code, "m").errorCode).toBe(code);
    }
  });

  it("errorResult rejects codes outside the closed set", () => {
    expect(() => errorResult("teapot", "m")).toThrow(/outside the closed set/);
    expect(() => errorResult("", "m")).toThrow();
  });

  it("outcomeToProto fills every wire field and defaults atUnixMs", () => {
    const before = Date.now();
    const got = outcomeToProto(errorResult(TARGET_REJECTED, "refused"));
    expect(got).toEqual({
      ok: false,
      resultJson: "",
      errorCode: TARGET_REJECTED,
      errorMessage: "refused",
      atUnixMs: expect.any(Number),
    });
    expect(got.atUnixMs).toBeGreaterThanOrEqual(before);
  });
});

describe("descriptorToProto", () => {
  it("defaults errorSchema and riskLevel", () => {
    const got = descriptorToProto({
      id: "plugin.pyecho.echo",
      summary: "s",
      paramSchema: "{}",
      resultSchema: "{}",
    });
    expect(got).toEqual({
      id: "plugin.pyecho.echo",
      summary: "s",
      paramSchema: "{}",
      resultSchema: "{}",
      errorSchema: "",
      riskLevel: "low",
    });
  });

  it("keeps an explicit risk level", () => {
    expect(
      descriptorToProto({
        id: "x",
        summary: "s",
        paramSchema: "{}",
        resultSchema: "{}",
        riskLevel: "danger",
      }).riskLevel,
    ).toBe("danger");
  });

  it("risk levels stay the host's closed set", () => {
    expect(RISK_LEVELS).toEqual(["low", "medium", "high", "danger"]);
  });
});

describe("makeInvokeCall", () => {
  const future = Date.now() + 60_000;

  it("maps the wire fields and stays live", () => {
    const call = makeInvokeCall(
      { functionId: "plugin.pyecho.echo", paramsJson: "{}", timeoutMs: 900 },
      { getDeadline: () => future },
    );
    expect(call.functionId).toBe("plugin.pyecho.echo");
    expect(call.paramsJson).toBe("{}");
    expect(call.timeoutMs).toBe(900);
    expect(call.isCancelled()).toBe(false);
  });

  it("reports cancelled past the deadline and honours the flag", () => {
    const past = makeInvokeCall(
      { functionId: "f", paramsJson: "", timeoutMs: 0 },
      { getDeadline: () => Date.now() - 1 },
    );
    expect(past.isCancelled()).toBe(true);
    const flagged = makeInvokeCall(
      { functionId: "f", paramsJson: "", timeoutMs: 0 },
      { getDeadline: () => future, cancelled: true },
    );
    expect(flagged.isCancelled()).toBe(true);
  });

  it("defaults missing wire fields", () => {
    const call = makeInvokeCall(
      {} as { functionId: string; paramsJson: string; timeoutMs: number },
      {},
    );
    expect(call.functionId).toBe("");
    expect(call.paramsJson).toBe("");
    expect(call.timeoutMs).toBe(0);
    expect(call.remainingMs()).toBe(Number.POSITIVE_INFINITY);
  });
});

describe("handler maps", () => {
  it("Describe returns the plugin self-description", () => {
    const handlers = funcHandlers({
      describe: () => ({ name: "pyecho", version: "1.2.3" }),
      listFunctions: () => [],
      invoke: () => okResult("{}"),
    });
    let response: { name?: string; version?: string } | null = null;
    (handlers.Describe as (call: unknown, cb: (err: null, res: { name?: string; version?: string }) => void) => void)(
      {},
      (_err, res) => {
        response = res;
      },
    );
    expect(response).toEqual({ name: "pyecho", version: "1.2.3" });
  });

  it("ListFunctions maps every descriptor onto the wire", () => {
    const handlers = funcHandlers({
      describe: () => ({ name: "pyecho", version: "1.2.3" }),
      listFunctions: () => [
        { id: "plugin.pyecho.echo", summary: "s", paramSchema: "{}", resultSchema: "{}" },
      ],
      invoke: () => okResult("{}"),
    });
    let seen = false;
    (handlers.ListFunctions as (call: unknown, cb: (err: null, res: { functions?: FuncDescriptorShape[] }) => void) => void)(
      {},
      (_err, res) => {
        seen = true;
        expect(res.functions).toHaveLength(1);
        expect(res.functions?.[0]).toEqual({
          id: "plugin.pyecho.echo",
          summary: "s",
          paramSchema: "{}",
          resultSchema: "{}",
          errorSchema: "",
          riskLevel: "low",
        });
      },
    );
    expect(seen).toBe(true);
  });

  it("Invoke maps an async outcome onto the wire envelope", async () => {
    const handlers = funcHandlers({
      describe: () => ({ name: "pyecho", version: "1.2.3" }),
      listFunctions: () => [],
      invoke: async () => errorResult(TARGET_UNAVAILABLE, "down"),
    });
    await new Promise<void>((resolve) => {
      (
        handlers.Invoke as (
          call: { request?: unknown },
          cb: (err: null, res: Record<string, unknown>) => void,
        ) => void
      )({ request: { functionId: "f", paramsJson: "{}", timeoutMs: 10 } }, (_err, res) => {
        expect(res.ok).toBe(false);
        expect(res.errorCode).toBe(TARGET_UNAVAILABLE);
        expect(res.errorMessage).toBe("down");
        resolve();
      });
    });
  });

  it("keeps the cookie constants in sync with the Python SDK", () => {
    expect(COOKIE_KEY).toBe("CROUPIER_SIDECAR_FUNC_PLUGIN");
    expect(COOKIE_VALUE).toBe("croupier-sidecar-function-plugin");
    expect(BAD_RESPONSE).toBe("bad_response");
    // And the barrel re-exports the func face under FUNC_* aliases.
    expect(FUNC_COOKIE_KEY).toBe(COOKIE_KEY);
    expect(FUNC_COOKIE_VALUE).toBe(COOKIE_VALUE);
  });
});
