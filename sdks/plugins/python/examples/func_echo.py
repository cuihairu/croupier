#!/usr/bin/env python3
"""Reference function adapter plugin: pyecho (todo.md:38).

Mirrors the Go reference adapters (funcplugin/examples/httpget and
examples/tcp) in Python, so the plugin contract has a cross-language
proof: the real Go host drives this process through the real go-plugin
handshake in the SDK test suite (hostdriver scenarios).

Declared functions:

    plugin.pyecho.echo             echo {"message"} back
    plugin.pyecho.fail_unavailable always fail target_unavailable
    plugin.pyecho.fail_rejected    always fail target_rejected
    plugin.pyecho.slow             sleep {"sleep_ms"} (bounded), then echo

Envelope discipline, identical to the Go references: business failures
ride the closed-set InvokeResult envelope and never raise; a raised
exception means the plugin process itself is broken, which the host
classifies as a plugin_* degradation cause. Unknown function ids and
malformed params are bad_response, not fake successes.

Run (the host supplies the handshake env; direct execution is refused):
    python3 examples/func_echo.py
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path

if __package__ in (None, ""):
    # Runnable from a checkout without pip install; installed via
    # `pip install -e sdks/plugins/python` in CI.
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import croupier_plugin as cp  # noqa: E402

PLUGIN_NAME = "pyecho"
PLUGIN_VERSION = "1.2.3"
NAMESPACE = f"plugin.{PLUGIN_NAME}."

_PARAM_MESSAGE = (
    '{"type":"object","properties":{"message":{"type":"string"}},'
    '"required":["message"],"additionalProperties":false}'
)
_PARAM_EMPTY = '{"type":"object","additionalProperties":false}'
_PARAM_SLEEP = (
    '{"type":"object","properties":{"sleep_ms":{"type":"integer","minimum":0},'
    '"message":{"type":"string"}},"required":["sleep_ms"],'
    '"additionalProperties":false}'
)
_RESULT_MESSAGE = (
    '{"type":"object","properties":{"message":{"type":"string"}},'
    '"required":["message"],"additionalProperties":false}'
)


class PyEchoPlugin(cp.FunctionPlugin):
    def describe(self) -> tuple[str, str]:
        return PLUGIN_NAME, PLUGIN_VERSION

    def list_functions(self) -> list[cp.FunctionDescriptor]:
        # Declared at startup; the host re-reads this list on every
        # registration reconcile, so a dynamic list is supported the same
        # way the Go reference re-reads its functions file.
        return [
            cp.FunctionDescriptor(
                id=f"{NAMESPACE}echo",
                summary="echo the message back",
                param_schema=_PARAM_MESSAGE,
                result_schema=_RESULT_MESSAGE,
            ),
            cp.FunctionDescriptor(
                id=f"{NAMESPACE}fail_unavailable",
                summary="always fail with target_unavailable",
                param_schema=_PARAM_EMPTY,
                result_schema=_RESULT_MESSAGE,
            ),
            cp.FunctionDescriptor(
                id=f"{NAMESPACE}fail_rejected",
                summary="always fail with target_rejected",
                param_schema=_PARAM_EMPTY,
                result_schema=_RESULT_MESSAGE,
            ),
            cp.FunctionDescriptor(
                id=f"{NAMESPACE}slow",
                summary="sleep sleep_ms (bounded by the call deadline), then echo",
                param_schema=_PARAM_SLEEP,
                result_schema=_RESULT_MESSAGE,
            ),
        ]

    def invoke(self, call: cp.InvokeInput) -> cp.InvokeResult:
        if not call.function_id.startswith(NAMESPACE):
            return cp.error_result(
                cp.BAD_RESPONSE, f"unknown function {call.function_id!r}"
            )
        name = call.function_id[len(NAMESPACE) :]
        handler = {
            "echo": self._echo,
            "fail_unavailable": self._fail_unavailable,
            "fail_rejected": self._fail_rejected,
            "slow": self._slow,
        }.get(name)
        if handler is None:
            return cp.error_result(
                cp.BAD_RESPONSE, f"unknown function {call.function_id!r}"
            )
        params, err = self._params(call)
        if err is not None:
            return err
        return handler(call, params)

    # -- handlers -----------------------------------------------------------

    def _echo(self, call: cp.InvokeInput, params: dict) -> cp.InvokeResult:
        message = params.get("message")
        if not isinstance(message, str) or message == "":
            return cp.error_result(cp.BAD_RESPONSE, "params.message must be a non-empty string")
        return cp.ok_result(json.dumps({"message": message}))

    def _fail_unavailable(self, call: cp.InvokeInput, params: dict) -> cp.InvokeResult:
        return cp.error_result(
            cp.TARGET_UNAVAILABLE, "game interface unreachable (synthetic)"
        )

    def _fail_rejected(self, call: cp.InvokeInput, params: dict) -> cp.InvokeResult:
        return cp.error_result(
            cp.TARGET_REJECTED, "game interface refused: synthetic rejection"
        )

    def _slow(self, call: cp.InvokeInput, params: dict) -> cp.InvokeResult:
        # Bounded wait: poll the RPC context so a host deadline or caller
        # cancellation ends the wait instead of outliving it.
        remaining_ms = params.get("sleep_ms", 0)
        deadline = time.monotonic() + max(0, remaining_ms) / 1000.0
        while time.monotonic() < deadline:
            if not call.context.is_active():
                return cp.error_result(cp.TARGET_UNAVAILABLE, "call cancelled")
            time.sleep(min(0.02, max(0.0, deadline - time.monotonic())))
        message = params.get("message", "woke up")
        return cp.ok_result(json.dumps({"message": message}))

    # -- params -------------------------------------------------------------

    @staticmethod
    def _params(call: cp.InvokeInput) -> tuple[dict | None, cp.InvokeResult | None]:
        try:
            params = json.loads(call.params_json)
        except ValueError:
            return None, cp.error_result(
                cp.BAD_RESPONSE, "params are not the declared JSON object"
            )
        if not isinstance(params, dict):
            return None, cp.error_result(
                cp.BAD_RESPONSE, "params are not the declared JSON object"
            )
        return params, None


def main() -> None:
    cp.serve_function_plugin(PyEchoPlugin())


if __name__ == "__main__":
    main()
