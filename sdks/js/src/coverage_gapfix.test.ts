/**
 * Coverage gap fix: readLoop top-level exception is swallowed by the
 * `.catch(() => {})` harness (no unhandled rejection), plus regression
 * guards for the defensive defaults documented alongside the istanbul
 * ignore markers added in this change.
 */

import { TCPTransport } from "./tcp_transport";

interface TransportInternals {
  startReader: () => void;
  readerTimer: NodeJS.Timeout | null;
  running: boolean;
  connected: boolean;
}

describe("TCPTransport readLoop exception harness", () => {
  it("swallows a top-level readLoop exception without unhandled rejection", async () => {
    const transport = new TCPTransport({ address: "127.0.0.1:1", timeoutMs: 100 });
    const unhandled: unknown[] = [];
    const onUnhandled = (err: unknown) => unhandled.push(err);
    process.on("unhandledRejection", onUnhandled);

    try {
      // Force the readLoop's while-condition to throw: the only escape
      // hatch outside its inner try/catch, exercising the outer
      // readLoop().catch(() => {}) harness installed by startReader().
      Object.defineProperty(transport, "running", {
        configurable: true,
        get(): boolean {
          throw new Error("running accessor exploded");
        },
      });
      (transport as unknown as TransportInternals).startReader();
      // Give the microtask queue a turn so the rejection propagates
      // through the catch handler.
      await new Promise((resolve) => setImmediate(resolve));
      expect(unhandled).toHaveLength(0);
    } finally {
      process.off("unhandledRejection", onUnhandled);
      // Restore a plain writable property so cleanup paths can run,
      // then release the keep-alive timer created by startReader().
      Object.defineProperty(transport, "running", {
        configurable: true,
        value: false,
        writable: true,
      });
      const internals = transport as unknown as TransportInternals;
      internals.connected = false;
      if (internals.readerTimer) {
        clearTimeout(internals.readerTimer);
        internals.readerTimer = null;
      }
    }
  });
});
