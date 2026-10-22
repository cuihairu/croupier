/** Unit tests for the go-plugin handshake pieces in src/serve.ts. */

import {
  CORE_PROTOCOL_VERSION,
  PluginHandshakeError,
  handshakeLine,
  negotiateProtocolVersion,
  requireCookie,
  resolveBind,
} from "../src";

describe("requireCookie", () => {
  const key = "CROUPIER_SYNTHETICS_PLUGIN";
  const value = "croupier-synthetics-probe-plugin";

  it("passes when the host set the exact cookie", () => {
    expect(() => requireCookie({ [key]: value }, key, value)).not.toThrow();
  });

  it("refuses an unset cookie", () => {
    expect(() => requireCookie({}, key, value)).toThrow(PluginHandshakeError);
  });

  it("refuses a wrong cookie value", () => {
    expect(() => requireCookie({ [key]: "other" }, key, value)).toThrow(
      PluginHandshakeError,
    );
  });

  it("refuses with a message that names the env key", () => {
    expect(() => requireCookie({}, key, value)).toThrow(key);
  });
});

describe("negotiateProtocolVersion", () => {
  it("returns the built-in version when the host predates versioning", () => {
    expect(negotiateProtocolVersion(undefined, 1)).toBe(1);
    expect(negotiateProtocolVersion("   ", 1)).toBe(1);
  });

  it("accepts the requested version when the host supports it", () => {
    expect(negotiateProtocolVersion("1,2,3", 1)).toBe(1);
    expect(negotiateProtocolVersion("2,3", 3)).toBe(3);
  });

  it("refuses a requested version outside the host list", () => {
    expect(() => negotiateProtocolVersion("2,3", 1)).toThrow(
      PluginHandshakeError,
    );
  });

  it("refuses a garbage host list", () => {
    expect(() => negotiateProtocolVersion("not-a-list", 1)).toThrow(
      PluginHandshakeError,
    );
  });

  it("defaults the request to the core protocol version", () => {
    expect(negotiateProtocolVersion("1")).toBe(CORE_PROTOCOL_VERSION);
    expect(negotiateProtocolVersion("9,1")).toBe(CORE_PROTOCOL_VERSION);
  });
});

describe("handshakeLine", () => {
  it("writes core|protocol|network|addr|grpc with a trailing separator", () => {
    expect(handshakeLine(1, 1, "tcp", "127.0.0.1:54321", false)).toBe(
      "1|1|tcp|127.0.0.1:54321|grpc|",
    );
  });

  it("appends the multiplex flag last", () => {
    expect(handshakeLine(1, 1, "tcp", "127.0.0.1:54321", true)).toBe(
      "1|1|tcp|127.0.0.1:54321|grpc|true",
    );
  });
});

describe("resolveBind", () => {
  it("defaults to loopback tcp with an ephemeral port", () => {
    expect(resolveBind({}, "CROUPIER_PLUGIN_BIND")).toEqual({
      network: "tcp",
      addr: "127.0.0.1:0",
    });
  });

  it("honours an explicit bind override", () => {
    expect(
      resolveBind({ CROUPIER_PLUGIN_BIND: "127.0.0.1:19191" }, "CROUPIER_PLUGIN_BIND"),
    ).toEqual({ network: "tcp", addr: "127.0.0.1:19191" });
  });

  it("labels path-style addresses as unix (grpc-js cannot bind them)", () => {
    expect(
      resolveBind({ CROUPIER_PLUGIN_BIND: "/tmp/plugin.sock" }, "CROUPIER_PLUGIN_BIND"),
    ).toEqual({ network: "unix", addr: "/tmp/plugin.sock" });
  });
});
