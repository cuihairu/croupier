/**
 * go-plugin handshake + gRPC server lifecycle for Node.js plugins.
 *
 * Mirrors sdks/plugins/python/croupier_plugin/serve.py so that both hosts and
 * plugin authors see identical behaviour: cookie gate, protocol negotiation,
 * one handshake line on stdout, parked GRPCStdio stream, bounded stop.
 *
 * Binding: grpc-js has no unix domain socket support, so this SDK binds TCP
 * (127.0.0.1 by default) instead of the Python SDK's socket-unix preference.
 * go-plugin dials whatever network+addr the handshake line advertises, so the
 * two SDKs are interchangeable behind the same host config.
 */

import * as grpc from "@grpc/grpc-js";
import { loadProtos } from "./proto";

/** Core protocol version go-plugin's host speaks (handshake field 1). */
export const CORE_PROTOCOL_VERSION = 1;
/** Protocol versions this SDK will negotiate into (handshake field 2). */
export const CORE_PROTOCOL_VERSIONS: readonly number[] = [1];
/**
 * Seconds granted to the shutdown sequence once the host has actually
 * requested it (Shutdown RPC or SIGTERM/SIGINT). The countdown does not run
 * while the plugin is healthy: a plugin that serves past any number of
 * grace periods without a shutdown request is never force-exited.
 */
export const STOP_GRACE_SECONDS = 2;

/** Raised when a plugin is executed outside its magic-cookie gate. */
export class PluginHandshakeError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PluginHandshakeError";
  }
}

/** Options for servePlugin. */
export interface ServeOptions {
  /** Env var name carrying the magic cookie. */
  cookieKey: string;
  /** Expected cookie value. */
  cookieValue: string;
  /** Env var holding the host's supported protocol version list. */
  protocolVersionsKey: string;
  /** Protocol version this SDK requests (handshake field 2). */
  protocolVersion?: number;
  /** Host-set env var enabling multiplex gRPC (handshake field 7). */
  multiplexKey: string;
  /** Env var overriding the TCP bind address (default 127.0.0.1:0). */
  bindEnv: string;
  /** Register the plugin's service handlers on the server. */
  register: (server: grpc.Server) => void;
  /**
   * Extra stop hooks (e.g. closing a plugin's own resources). Started only
   * when shutdown is requested and awaited inside STOP_GRACE_SECONDS; a
   * hook that hangs past the budget gets the process force-exited.
   */
  onShutdown?: () => Promise<void> | void;
  /** Injectable env for tests; defaults to process.env. */
  env?: Record<string, string | undefined>;
}

/** Fail closed: a plugin must not serve when the cookie gate is absent. */
export function requireCookie(
  env: Record<string, string | undefined>,
  key: string,
  value: string,
): void {
  const seen = env[key];
  if (seen === value) {
    return;
  }
  const message =
    `This binary is a Croupier plugin and must be launched by a Croupier ` +
    `host, not run directly. The host sets ${key}=<token>; here it is ` +
    `${seen === undefined ? "unset" : "set to an unexpected value"}.`;
  throw new PluginHandshakeError(message);
}

/** Parse the host's supported-version env var and refuse an unsupported pick. */
export function negotiateProtocolVersion(
  supportedRaw: string | undefined,
  requested: number = CORE_PROTOCOL_VERSION,
  supported: readonly number[] = CORE_PROTOCOL_VERSIONS,
): number {
  if (supportedRaw === undefined || supportedRaw.trim() === "") {
    // No env at all means a host older than the versioning feature.
    return supported.includes(requested) ? requested : CORE_PROTOCOL_VERSION;
  }
  const supportedByHost = supportedRaw
    .split(",")
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0);
  if (supportedByHost.length === 0) {
    throw new PluginHandshakeError(
      "plugin handshake: host PLUGIN_PROTOCOL_VERSIONS advertised an " +
        `unusable version list: ${JSON.stringify(supportedRaw)}`,
    );
  }
  if (!supportedByHost.includes(requested)) {
    throw new PluginHandshakeError(
      `plugin handshake: requested protocol version ${requested} is not in ` +
        `the host's supported list ${supportedByHost.join(",")}; the host ` +
        `must be upgraded or the plugin's version request lowered`,
    );
  }
  return requested;
}

/** Build the single stdout line go-plugin's ParseHandshake consumes. */
export function handshakeLine(
  coreProtocolVersion: number,
  protocolVersion: number,
  network: string,
  addr: string,
  multiplex: boolean,
): string {
  const line =
    `${coreProtocolVersion}|${protocolVersion}|${network}|${addr}|grpc|`;
  return multiplex ? `${line}true` : line;
}

/** Decide network+addr for the bind. TCP only: see the module header. */
export function resolveBind(
  env: Record<string, string | undefined>,
  bindEnv: string,
): { network: string; addr: string } {
  const addr = (env[bindEnv] || "").trim() || "127.0.0.1:0";
  const network = addr.startsWith("/") ? "unix" : "tcp";
  return { network, addr };
}

/** Bind the gRPC server to loopback and return the bound address. */
export function bindTcp(
  server: grpc.Server,
  bind: string,
): Promise<{ network: string; addr: string }> {
  return new Promise((resolve, reject) => {
    server.bindAsync(bind, grpc.ServerCredentials.createInsecure(), (err, port) => {
      if (err !== null) {
        reject(new PluginHandshakeError(
          `plugin handshake: cannot bind ${bind}: ${err.message}`,
        ));
        return;
      }
      resolve({ network: "tcp", addr: `127.0.0.1:${port}` });
    });
  });
}

/** Health servicer: go-plugin health-checks every loaded plugin on startup. */
export function healthHandlers(): grpc.UntypedServiceImplementation {
  const proto = loadProtos();
  return {
    Check: (
      _call: grpc.ServerUnaryCall<unknown, unknown>,
      callback: grpc.sendUnaryData<{ status: number }>,
    ) => {
      callback(null, { status: proto.servingStatus });
    },
    Watch: (call: grpc.ServerWritableStream<unknown, unknown>) => {
      call.write({ status: proto.servingStatus });
      call.end();
    },
  };
}

/** GRPCStdio handlers: the stream must stay open for the host's lifetime. */
export function stdioHandlers(): {
  handlers: grpc.UntypedServiceImplementation;
  parked: Set<grpc.ServerDuplexStream<unknown, unknown>>;
} {
  const parked = new Set<grpc.ServerDuplexStream<unknown, unknown>>();
  return {
    handlers: {
      StreamStdio: (call: grpc.ServerDuplexStream<unknown, unknown>) => {
        parked.add(call);
        call.on("cancelled", () => parked.delete(call));
        call.on("close", () => parked.delete(call));
      },
    },
    parked,
  };
}

/**
 * Controller handlers: Shutdown is the only RPC go-plugin issues here. The
 * reply must leave before the teardown starts (the host waits for it), so
 * the stop runs on the next macrotask with a small flush margin.
 */
export function controllerHandlers(onShutdown: () => void): grpc.UntypedServiceImplementation {
  const proto = loadProtos();
  return {
    Shutdown: (
      _call: grpc.ServerUnaryCall<unknown, unknown>,
      callback: grpc.sendUnaryData<unknown>,
    ) => {
      callback(null, {});
      setTimeout(onShutdown, 50);
    },
  };
}

/**
 * Serve a plugin until the host shuts it down or a signal arrives. While the
 * plugin is healthy no exit timer exists: it keeps answering calls past any
 * number of grace periods. Resolves once the server has stopped and the
 * onShutdown hook has settled; a shutdown that outlives STOP_GRACE_SECONDS
 * (hung session or hanging cleanup) is force-exited by the unref'd budget
 * timer armed in stop().
 */
export async function servePlugin(options: ServeOptions): Promise<void> {
  const env = options.env ?? process.env;
  requireCookie(env, options.cookieKey, options.cookieValue);

  const proto = loadProtos();
  const { network, addr } = resolveBind(env, options.bindEnv);
  const version = negotiateProtocolVersion(
    env[options.protocolVersionsKey],
    options.protocolVersion ?? CORE_PROTOCOL_VERSION,
  );
  const multiplex =
    env[options.multiplexKey] === "1" || env[options.multiplexKey] === "true";

  const server = new grpc.Server();
  server.addService(proto.healthService, healthHandlers());
  const stdio = stdioHandlers();
  server.addService(proto.stdioService, stdio.handlers);

  let terminated: (() => void) | null = null;
  const done = new Promise<void>((resolve) => {
    terminated = resolve;
  });

  let stopping = false;
  // The forced exit exists only while a shutdown is in flight (AUD-075):
  // arming it at startup killed healthy plugins that served past the grace
  // period without any shutdown request.
  let forcedExit: NodeJS.Timeout | null = null;
  // The plugin's own async cleanup, started by the first shutdown request
  // and awaited at the end of servePlugin inside the same budget.
  let cleanup: Promise<void> = Promise.resolve();
  const stop = (): void => {
    if (stopping) {
      return;
    }
    stopping = true;
    // The bounded wait budget starts here, on the actual shutdown request —
    // never at startup. forceShutdown first: the parked GRPCStdio stream
    // never ends, so a graceful tryShutdown alone would wait forever.
    // forceShutdown kills the sessions and lets tryShutdown's completion
    // callback fire.
    forcedExit = setTimeout(() => {
      process.exit(0);
    }, STOP_GRACE_SECONDS * 1000);
    forcedExit.unref();
    server.forceShutdown();
    server.tryShutdown(() => {
      if (terminated !== null) {
        terminated();
      }
    });
    // Explicit wait budget for the plugin's own async cleanup: the forced
    // exit above fires only if the cleanup hook (or the server teardown) is
    // still running when the budget lapses; settling both clears it.
    cleanup = Promise.resolve(options.onShutdown?.()).catch(() => {
      // stop is best-effort; a rejecting hook still lets the process exit
    });
    void Promise.all([done, cleanup]).then(() => {
      if (forcedExit !== null) {
        clearTimeout(forcedExit);
        forcedExit = null;
      }
    });
  };
  server.addService(proto.controllerService, controllerHandlers(stop));
  options.register(server);

  const bound = await bindTcp(server, addr);
  // The host blocks on this exact line. Node writes to a piped stdout are
  // async but the event loop keeps running, so the line reaches the host's
  // scanner without a flush primitive (process.stdout has none).
  process.stdout.write(
    handshakeLine(CORE_PROTOCOL_VERSION, version, bound.network, bound.addr, multiplex) + "\n",
  );

  const sign = (): void => {
    stop();
  };
  process.on("SIGTERM", sign);
  process.on("SIGINT", sign);

  await done;
  // Wait for the plugin's own cleanup inside the same budget: while the
  // hook is pending the armed forced exit stays live, so a hook that hangs
  // past STOP_GRACE_SECONDS force-exits the process instead of parking the
  // drain forever. Settling (done + cleanup) cleared the timer in stop().
  await cleanup;
  process.removeListener("SIGTERM", sign);
  process.removeListener("SIGINT", sign);
  for (const call of stdio.parked) {
    call.end();
  }
}
