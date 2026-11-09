import { BasicClient } from "./index";

describe("BasicClient", () => {
  describe("Configuration Options - controlAddr", () => {
    test("controlAddr can be set", () => {
      const client = new BasicClient({
        controlAddr: "localhost:9090",
      });
      expect((client as any).config.controlAddr).toBe("localhost:9090");
    });

    test("controlAddr defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.controlAddr).toBe("");
    });
  });

  describe("Configuration Options - agentId and serverName", () => {
    test("agentId can be set", () => {
      const client = new BasicClient({
        agentId: "agent-1",
      });
      expect((client as any).config.agentId).toBe("agent-1");
    });

    test("agentId defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.agentId).toBe("");
    });

    test("serverName can be set", () => {
      const client = new BasicClient({
        serverName: "agent.example.com",
      });
      expect((client as any).config.serverName).toBe("agent.example.com");
    });

    test("serverName defaults to empty string", () => {
      const client = new BasicClient();
      expect((client as any).config.serverName).toBe("");
    });
  });

  describe("Configuration Options - Logging", () => {
    test("disableLogging can be set to true", () => {
      const client = new BasicClient({
        disableLogging: true,
      });
      expect((client as any).config.disableLogging).toBe(true);
    });

    test("disableLogging defaults to false", () => {
      const client = new BasicClient();
      expect((client as any).config.disableLogging).toBe(false);
    });

    test("debugLogging can be set to true", () => {
      const client = new BasicClient({
        debugLogging: true,
      });
      expect((client as any).config.debugLogging).toBe(true);
    });

    test("debugLogging defaults to false", () => {
      const client = new BasicClient();
      expect((client as any).config.debugLogging).toBe(false);
    });

    test("logLevel can be set to valid values", () => {
      const validLevels: Array<"DEBUG" | "INFO" | "WARN" | "ERROR" | "OFF"> = [
        "DEBUG",
        "INFO",
        "WARN",
        "ERROR",
        "OFF",
      ];

      for (const level of validLevels) {
        const client = new BasicClient({ logLevel: level });
        expect((client as any).config.logLevel).toBe(level);
      }
    });

    test("logLevel defaults to INFO", () => {
      const client = new BasicClient();
      expect((client as any).config.logLevel).toBe("INFO");
    });

    test("all logging options can be set together", () => {
      const client = new BasicClient({
        disableLogging: false,
        debugLogging: true,
        logLevel: "DEBUG",
      });
      expect((client as any).config.disableLogging).toBe(false);
      expect((client as any).config.debugLogging).toBe(true);
      expect((client as any).config.logLevel).toBe("DEBUG");
    });
  });
});
