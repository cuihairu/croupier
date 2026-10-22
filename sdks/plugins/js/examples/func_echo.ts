/**
 * Reference function adapter plugin: pyecho (todo.md:38).
 *
 * Mirrors the Go reference adapters (funcplugin/examples httpget/tcp) in
 * Node.js, so the plugin contract has a cross-language proof: the real
 * Go host drives this process through the real go-plugin handshake in
 * the SDK test suite (hostdriver scenarios).
 *
 * Declared functions:
 *
 *     plugin.pyecho.echo             echo {"message"} back
 *     plugin.pyecho.fail_unavailable always fail target_unavailable
 *     plugin.pyecho.fail_rejected    always fail target_rejected
 *     plugin.pyecho.slow             sleep {"sleep_ms"} (bounded), then echo
 *
 * Envelope discipline, identical to the Go references: business failures
 * ride the closed-set InvokeResult envelope and never throw; a thrown
 * exception means the plugin process itself is broken, which the host
 * classifies as a plugin_* degradation cause. Unknown function ids and
 * malformed params are bad_response, not fake successes.
 *
 * Run (the host supplies the handshake env; direct execution is refused):
 *     node dist/examples/func_echo.js
 */

import {
  BAD_RESPONSE,
  RISK_LOW,
  TARGET_REJECTED,
  TARGET_UNAVAILABLE,
  errorResult,
  okResult,
  serveFunctionPlugin,
  type FunctionDescriptor,
  type FunctionPlugin,
  type InvokeCall,
  type InvokeOutcome,
} from "../src";

const PLUGIN_NAME = "pyecho";
const PLUGIN_VERSION = "1.2.3";
const NAMESPACE = `plugin.${PLUGIN_NAME}.`;

const PARAM_MESSAGE =
  '{"type":"object","properties":{"message":{"type":"string"}},' +
  '"required":["message"],"additionalProperties":false}';
const PARAM_EMPTY = '{"type":"object","additionalProperties":false}';
const PARAM_SLEEP =
  '{"type":"object","properties":{"sleep_ms":{"type":"integer","minimum":0},' +
  '"message":{"type":"string"}},"required":["sleep_ms"],' +
  '"additionalProperties":false}';
const RESULT_MESSAGE =
  '{"type":"object","properties":{"message":{"type":"string"}},' +
  '"required":["message"],"additionalProperties":false}';

interface EchoParams {
  message?: unknown;
  sleep_ms?: unknown;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    setTimeout(resolve, Math.max(0, Math.floor(ms)));
  });
}

function parseParams(paramsJson: string): { params: EchoParams } | { err: InvokeOutcome } {
  try {
    const params = JSON.parse(paramsJson) as unknown;
    if (typeof params !== "object" || params === null || Array.isArray(params)) {
      return {
        err: errorResult(BAD_RESPONSE, "params are not the declared JSON object"),
      };
    }
    return { params: params as EchoParams };
  } catch {
    return {
      err: errorResult(BAD_RESPONSE, "params are not the declared JSON object"),
    };
  }
}

const pyecho: FunctionPlugin = {
  describe: () => ({ name: PLUGIN_NAME, version: PLUGIN_VERSION }),

  // Declared at startup; the host re-reads this list on every
  // registration reconcile, so a dynamic list is supported the same way
  // the Go reference re-reads its functions file.
  listFunctions: (): FunctionDescriptor[] => [
    {
      id: `${NAMESPACE}echo`,
      summary: "echo the message back",
      paramSchema: PARAM_MESSAGE,
      resultSchema: RESULT_MESSAGE,
      riskLevel: RISK_LOW,
    },
    {
      id: `${NAMESPACE}fail_unavailable`,
      summary: "always fail with target_unavailable",
      paramSchema: PARAM_EMPTY,
      resultSchema: RESULT_MESSAGE,
      riskLevel: RISK_LOW,
    },
    {
      id: `${NAMESPACE}fail_rejected`,
      summary: "always fail with target_rejected",
      paramSchema: PARAM_EMPTY,
      resultSchema: RESULT_MESSAGE,
      riskLevel: RISK_LOW,
    },
    {
      id: `${NAMESPACE}slow`,
      summary: "sleep sleep_ms (bounded by the call deadline), then echo",
      paramSchema: PARAM_SLEEP,
      resultSchema: RESULT_MESSAGE,
      riskLevel: RISK_LOW,
    },
  ],

  invoke: async (call: InvokeCall): Promise<InvokeOutcome> => {
    if (!call.functionId.startsWith(NAMESPACE)) {
      return errorResult(BAD_RESPONSE, `unknown function ${JSON.stringify(call.functionId)}`);
    }
    const name = call.functionId.slice(NAMESPACE.length);
    const parsed = parseParams(call.paramsJson);
    if ("err" in parsed) {
      return parsed.err;
    }
    const params = parsed.params;
    switch (name) {
      case "echo": {
        const { message } = params;
        if (typeof message !== "string" || message === "") {
          return errorResult(BAD_RESPONSE, "params.message must be a non-empty string");
        }
        return okResult(JSON.stringify({ message }));
      }
      case "fail_unavailable":
        return errorResult(TARGET_UNAVAILABLE, "game interface unreachable (synthetic)");
      case "fail_rejected":
        return errorResult(TARGET_REJECTED, "game interface refused: synthetic rejection");
      case "slow": {
        // Bounded wait: poll the RPC deadline so a host deadline or caller
        // cancellation ends the wait instead of outliving it. Timer-based:
        // the gRPC event loop must never block (no worker pool here).
        const remainingMs = Number(params.sleep_ms) || 0;
        const deadline = Date.now() + Math.max(0, remainingMs);
        while (Date.now() < deadline) {
          if (call.isCancelled()) {
            return errorResult(TARGET_UNAVAILABLE, "call cancelled");
          }
          await sleep(Math.min(20, deadline - Date.now()));
        }
        const message = typeof params.message === "string" ? params.message : "woke up";
        return okResult(JSON.stringify({ message }));
      }
      default:
        return errorResult(BAD_RESPONSE, `unknown function ${JSON.stringify(call.functionId)}`);
    }
  },
};

void serveFunctionPlugin(pyecho).then(
  () => undefined,
  (err: unknown) => {
    process.stderr.write(`${err instanceof Error ? err.message : String(err)}\n`);
    process.exit(1);
  },
);
