import { BasicClient } from "./index";

describe("BasicClient", () => {
  describe("serve()", () => {
    test("serve() requires at least one function", async () => {
      const client = new BasicClient({ agentAddr: "tcp://127.0.0.1:19090" });

      // Should reject because no functions registered
      await expect(client.serve()).rejects.toThrow();
    });

    test("serve() connects and waits indefinitely", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock the connect method to avoid actual connection
      const connectSpy = jest
        .spyOn(client, "connect")
        .mockImplementation(async () => {
          (client as any).connected = true;
        });

      // serve() should return a Promise
      const servePromise = client.serve();
      expect(servePromise).toBeInstanceOf(Promise);
      expect(connectSpy).toHaveBeenCalled();

      // It should not resolve immediately
      let resolved = false;
      servePromise.then(() => {
        resolved = true;
      });
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(resolved).toBe(false);

      // Cleanup - restore connect
      connectSpy.mockRestore();
      (client as any).connected = false;
    });

    test("serveAsync is alias for serve", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock connect to avoid actual connection
      jest.spyOn(client, "connect").mockImplementation(async () => {
        (client as any).connected = true;
      });

      // Both methods should return promises
      const serve1 = client.serve();
      const serve2 = client.serveAsync();

      // Both should be Promise instances
      expect(serve1).toBeInstanceOf(Promise);
      expect(serve2).toBeInstanceOf(Promise);

      // Cleanup
      (client.connect as jest.Mock).mockRestore();
      (client as any).connected = false;
    });

    test("serve() connects if not connected", async () => {
      const client = new BasicClient();
      client.registerFunction(
        { id: "serve.fn", version: "1.0.0" },
        async () => "ok",
      );

      // Mock the connect method BEFORE calling serve
      const connectSpy = jest
        .spyOn(client, "connect")
        .mockImplementation(async () => {
          (client as any).connected = true;
        });
      (client as any).connected = false;

      const servePromise = client.serve();

      // Give it a moment to call connect
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(connectSpy).toHaveBeenCalled();

      // Cleanup
      connectSpy.mockRestore();
      (client as any).connected = false;
    });
  });
});
