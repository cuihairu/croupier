/**
 * AUD-075: the forced exit must be armed by shutdown, not by startup.
 *
 * The bug: servePlugin armed a `process.exit` timer right after the
 * handshake, so any healthy plugin that kept serving past one grace period
 * was force-exited without the host ever requesting a shutdown. These tests
 * stand in for the process boundary with a spied `process.exit` and the real
 * clock (fake timers stall grpc-js's response dispatch, so the suite runs
 * the genuine budget against real sockets):
 *
 *  - a healthy plugin survives past two grace periods and keeps answering
 *    invocations; no shutdown request means no exit callback may fire;
 *  - a Shutdown RPC (or SIGTERM) arms the bounded exit and starts the
 *    explicit wait budget for the async cleanup hook;
 *  - cleanup that settles inside the budget clears the forced exit;
 *  - cleanup that hangs past the budget gets the process force-exited.
 *
 * Real host⇄plugin round trips stay in serve_e2e.test.ts and
 * integration_go_host.test.ts.
 */

import * as grpc from "@grpc/grpc-js";
import {
  COOKIE_KEY,
  COOKIE_VALUE,
  STOP_GRACE_SECONDS,
  loadProtos,
  passResult,
  serve,
} from "../src";

/** The wait budget serve.ts grants a shutdown once it is requested. */
const BUDGET_MS = STOP_GRACE_SECONDS * 1000;
/** Sleep margin that keeps "not yet" assertions inside the budget window. */
const BUDGET_MARGIN_MS = 300;

const HOST_ENV: Record<string, string> = {
  [COOKIE_KEY]: COOKIE_VALUE,
  PLUGIN_PROTOCOL_VERSIONS: "1",
};

const echoPlugin = {
  describe: () => ({ name: "probe-echo", version: "1.0.0" }),
  check: (call: { target: string }) =>
    passResult({ status: 200, detail: `probe-echo checked ${call.target}` }),
};

interface ControllerLike {
  shutdown(
    request: Record<string, never>,
    callback: (err: grpc.ServiceError | null, res?: Record<string, never>) => void,
  ): void;
  close(): void;
}

interface ProbeLike {
  check(
    request: { target: string; timeoutMs: number; payloadJson: string },
    callback: (err: grpc.ServiceError | null, res?: { detail: string }) => void,
  ): void;
  close(): void;
}

function clientFor<T>(service: string, name: string, addr: string): T {
  const proto = loadProtos() as unknown as Record<string, grpc.ServiceDefinition>;
  const ctor = grpc.makeClientConstructor(
    proto[service],
    name,
  ) as unknown as new (address: string, credentials: grpc.ChannelCredentials) => T;
  return new ctor(addr, grpc.credentials.createInsecure());
}

function sleep(ms: number): Promise<void> {
  return new Promise<void>((resolve) => {
    setTimeout(resolve, ms);
  });
}

/** Poll a condition on the real clock; reject when the deadline passes. */
async function until(
  cond: () => boolean,
  timeoutMs: number = 10_000,
): Promise<void> {
  const started = Date.now();
  while (!cond()) {
    if (Date.now() - started > timeoutMs) {
      throw new Error(`condition not met within ${timeoutMs}ms`);
    }
    await sleep(25);
  }
}

/** Wait until the handshake line lands on the mocked stdout; return addr. */
async function waitForHandshakeLine(chunks: string[]): Promise<string> {
  const started = Date.now();
  for (;;) {
    const text = chunks.join("");
    const match = text.match(/1\|1\|tcp\|127\.0\.0\.1:(\d+)\|grpc\|/);
    if (match !== null) {
      return `127.0.0.1:${match[1]}`;
    }
    if (Date.now() - started > 10_000) {
      throw new Error(`no handshake line appeared, stdout saw: ${text}`);
    }
    await sleep(25);
  }
}

/** Make the controller reply; the stop itself runs after the flush margin. */
function requestShutdown(controller: ControllerLike): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    controller.shutdown({}, (err) => {
      if (err !== null) {
        reject(err);
        return;
      }
      resolve();
    });
  });
}

/** Await one invocation to prove the plugin still serves calls. */
function checkOnce(probe: ProbeLike, target: string): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    probe.check({ target, timeoutMs: 1000, payloadJson: "" }, (err, res) => {
      if (err !== null || res === undefined) {
        reject(err ?? new Error("no check response"));
        return;
      }
      resolve(res.detail);
    });
  });
}

describe("serve() bounded shutdown lifecycle (AUD-075)", () => {
  let writeSpy: jest.SpyInstance;
  let exitSpy: jest.SpyInstance;
  let exitCodes: number[];
  let chunks: string[];

  beforeEach(() => {
    chunks = [];
    exitCodes = [];
    writeSpy = jest
      .spyOn(process.stdout, "write")
      .mockImplementation(((chunk: unknown) => {
        chunks.push(String(chunk));
        return true;
      }) as typeof process.stdout.write);
    exitSpy = jest
      .spyOn(process, "exit")
      .mockImplementation(((code?: number) => {
        exitCodes.push(code ?? 0);
      }) as typeof process.exit);
  });

  afterEach(() => {
    writeSpy.mockRestore();
    exitSpy.mockRestore();
  });

  it(
    "keeps serving past two grace periods without arming the forced exit",
    async () => {
      const served = serve(echoPlugin, { env: HOST_ENV });
      const addr = await waitForHandshakeLine(chunks);
      const probe = clientFor<ProbeLike>("probeService", "ProbePlugin", addr);
      const controller = clientFor<ControllerLike>(
        "controllerService",
        "GRPCController",
        addr,
      );
      try {
        // Two full grace periods of healthy life, split around live
        // invocations: no shutdown request means no exit callback may fire.
        await sleep(BUDGET_MS + BUDGET_MARGIN_MS);
        expect(exitSpy).not.toHaveBeenCalled();
        await expect(checkOnce(probe, "healthy-target")).resolves.toBe(
          "probe-echo checked healthy-target",
        );
        await sleep(BUDGET_MS + BUDGET_MARGIN_MS);
        expect(exitSpy).not.toHaveBeenCalled();
        await expect(checkOnce(probe, "healthy-target-later")).resolves.toBe(
          "probe-echo checked healthy-target-later",
        );

        // A healthy plugin still shuts down gracefully on request: the
        // budget timer is armed, but teardown + cleanup settle inside it, so
        // the forced exit is cleared instead of firing.
        await requestShutdown(controller);
        await expect(served).resolves.toBeUndefined();
        expect(exitSpy).not.toHaveBeenCalled();
      } finally {
        probe.close();
        controller.close();
      }
    },
    30_000,
  );

  it(
    "Shutdown RPC starts the bounded exit and awaits the cleanup inside the budget",
    async () => {
      let releaseCleanup: (() => void) | undefined;
      const cleanupGate = new Promise<void>((resolve) => {
        releaseCleanup = resolve;
      });
      let cleanupFinished = false;
      const served = serve(echoPlugin, {
        env: HOST_ENV,
        onShutdown: async () => {
          await cleanupGate;
          cleanupFinished = true;
        },
      });
      const addr = await waitForHandshakeLine(chunks);
      const controller = clientFor<ControllerLike>(
        "controllerService",
        "GRPCController",
        addr,
      );
      try {
        expect(exitSpy).not.toHaveBeenCalled();
        await requestShutdown(controller);
        // The shutdown was requested: the budget timer is armed (proven by
        // the sibling test that lets it lapse), but until the budget lapses
        // the exit must not fire even while the cleanup hook is pending.
        await sleep(BUDGET_MS - BUDGET_MARGIN_MS - 500);
        expect(exitSpy).not.toHaveBeenCalled();

        releaseCleanup?.();
        // serve() resolves only after the async cleanup hook was awaited.
        await expect(served).resolves.toBeUndefined();
        expect(cleanupFinished).toBe(true);
        // The budget timer was cleared: waiting far past it changes nothing.
        await sleep(BUDGET_MS + 2 * BUDGET_MARGIN_MS);
        expect(exitSpy).not.toHaveBeenCalled();
      } finally {
        controller.close();
      }
    },
    30_000,
  );

  it(
    "cleanup exceeding the budget triggers the forced exit",
    async () => {
      let releaseCleanup: (() => void) | undefined;
      const cleanupGate = new Promise<void>((resolve) => {
        releaseCleanup = resolve;
      });
      const served = serve(echoPlugin, {
        env: HOST_ENV,
        onShutdown: () => cleanupGate, // hangs until the test releases it
      });
      let settled = false;
      void served.then(
        () => {
          settled = true;
        },
        () => {
          settled = true;
        },
      );
      const addr = await waitForHandshakeLine(chunks);
      const controller = clientFor<ControllerLike>(
        "controllerService",
        "GRPCController",
        addr,
      );
      try {
        expect(exitSpy).not.toHaveBeenCalled();
        await requestShutdown(controller);
        expect(exitSpy).not.toHaveBeenCalled();
        // Still inside the budget with the cleanup hanging: no exit yet
        // (1.3s check against a 2s budget keeps a wide margin for load).
        await sleep(BUDGET_MS - 700);
        expect(exitSpy).not.toHaveBeenCalled();
        // Past the budget the hung cleanup forces the exit.
        await until(() => exitCodes.length > 0);
        expect(exitCodes).toEqual([0]);
        // serve() itself stays parked on the hung cleanup: only the forced
        // exit would end the real process here.
        await sleep(BUDGET_MARGIN_MS);
        expect(settled).toBe(false);

        // Drain for real so the suite leaves no open handles behind: the
        // hook settles late, the parked serve completes after the exit
        // already fired.
        releaseCleanup?.();
        await expect(served).resolves.toBeUndefined();
        expect(settled).toBe(true);
      } finally {
        controller.close();
      }
    },
    30_000,
  );

  it(
    "SIGTERM starts the same bounded exit as Shutdown",
    async () => {
      let releaseCleanup: (() => void) | undefined;
      const cleanupGate = new Promise<void>((resolve) => {
        releaseCleanup = resolve;
      });
      const served = serve(echoPlugin, {
        env: HOST_ENV,
        onShutdown: () => cleanupGate, // hangs until the test releases it
      });
      let settled = false;
      void served.then(
        () => {
          settled = true;
        },
        () => {
          settled = true;
        },
      );
      const addr = await waitForHandshakeLine(chunks);
      // Healthy phase: merely living past the grace period arms nothing.
      await sleep(BUDGET_MS + BUDGET_MARGIN_MS);
      expect(exitSpy).not.toHaveBeenCalled();

      process.emit("SIGTERM", "SIGTERM");
      // Past the budget the hung cleanup forces the exit, exactly like the
      // Shutdown RPC path.
      await until(() => exitCodes.length > 0);
      expect(exitCodes).toEqual([0]);
      await sleep(BUDGET_MARGIN_MS);
      expect(settled).toBe(false);

      releaseCleanup?.();
      await expect(served).resolves.toBeUndefined();
      expect(settled).toBe(true);
    },
    30_000,
  );
});
