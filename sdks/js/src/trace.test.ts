import { traceParentFromContext, traceIdFromContext } from "./trace";

describe("trace context helpers", () => {
  it("extracts trace fields from a handler context", () => {
    const ctx = JSON.stringify({
      traceparent: "00-abc-def-01",
      traceId: "abc",
      game_id: "demo",
    });
    expect(traceParentFromContext(ctx)).toBe("00-abc-def-01");
    expect(traceIdFromContext(ctx)).toBe("abc");
  });

  it("returns empty strings when trace fields are absent", () => {
    const ctx = JSON.stringify({ game_id: "demo" });
    expect(traceParentFromContext(ctx)).toBe("");
    expect(traceIdFromContext(ctx)).toBe("");
  });

  it("tolerates empty and malformed context", () => {
    expect(traceParentFromContext("")).toBe("");
    expect(traceIdFromContext("not json")).toBe("");
  });
});
