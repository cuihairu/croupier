"""Sidecar function adapter plugin SDK (todo.md:38).

A function plugin is a resident subprocess the sidecar launches and maps
function calls onto the game's existing HTTP/TCP/admin interface. The
host keeps only the call; the plugin holds no Croupier credentials and
never learns the server address.

Business failures ride the closed-set InvokeResponse envelope and must
not be raised — a raised exception means the plugin process itself is
broken, which the host classifies as a plugin_* degradation cause.
"""

from __future__ import annotations

import abc
import time
from dataclasses import dataclass

import grpc

from . import funcplugin_pb2, funcplugin_pb2_grpc
from . import serve as _serve

# Handshake constants mirrored from the host (agents/sidecar/funcplugin/
# contract.go); a mismatch fails the subprocess before any RPC happens.
COOKIE_KEY = "CROUPIER_SIDECAR_FUNC_PLUGIN"
COOKIE_VALUE = "croupier-sidecar-function-plugin"
PROTOCOL_VERSION = 1

# Closed set for InvokeResult.error_code: the host refuses anything else
# and classifies the call as a plugin contract violation instead of
# letting an unknown code leak into the caller's error taxonomy.
TARGET_UNAVAILABLE = "target_unavailable"
TARGET_REJECTED = "target_rejected"
BAD_RESPONSE = "bad_response"
_ERROR_CODES = frozenset({TARGET_UNAVAILABLE, TARGET_REJECTED, BAD_RESPONSE})

# Closed set for FunctionDescriptor.risk_level; empty means low.
RISK_LOW = "low"
RISK_MEDIUM = "medium"
RISK_HIGH = "high"
RISK_DANGER = "danger"
RISK_LEVELS = frozenset({RISK_LOW, RISK_MEDIUM, RISK_HIGH, RISK_DANGER})


def _now_ms() -> int:
    return int(time.time() * 1000)


@dataclass(frozen=True, slots=True)
class FunctionDescriptor:
    """One declared function.

    id must live under "plugin.<plugin-name>.<function>"; the host
    rejects anything else and validates param_schema/result_schema as
    top-level object JSON Schemas, so declarations that cannot promise
    the contract fail at registration instead of at call time.
    """

    id: str
    summary: str
    param_schema: str
    result_schema: str
    error_schema: str = ""
    risk_level: str = RISK_LOW

    def to_proto(self) -> funcplugin_pb2.FuncDescriptor:
        return funcplugin_pb2.FuncDescriptor(
            id=self.id,
            summary=self.summary,
            param_schema=self.param_schema,
            result_schema=self.result_schema,
            error_schema=self.error_schema,
            risk_level=self.risk_level,
        )


@dataclass(frozen=True, slots=True)
class InvokeInput:
    """One host->plugin call: the registered function id, the JSON
    params object (already conforming to the declared param_schema) and
    the remaining host-side deadline in milliseconds."""

    function_id: str
    params_json: str
    timeout_ms: int
    context: grpc.ServicerContext


@dataclass(slots=True)
class InvokeResult:
    """ok=True carries result_json; ok=False carries error_code /
    error_message from the closed set. at_unix_ms defaults to now."""

    ok: bool
    result_json: str = ""
    error_code: str = ""
    error_message: str = ""
    at_unix_ms: int | None = None

    def to_proto(self) -> funcplugin_pb2.InvokeResponse:
        return funcplugin_pb2.InvokeResponse(
            ok=self.ok,
            result_json=self.result_json,
            error_code=self.error_code,
            error_message=self.error_message,
            at_unix_ms=self.at_unix_ms if self.at_unix_ms is not None else _now_ms(),
        )


def ok_result(result_json: str, at_unix_ms: int | None = None) -> InvokeResult:
    """Build a success envelope."""
    return InvokeResult(ok=True, result_json=result_json, at_unix_ms=at_unix_ms)


def error_result(
    code: str, message: str, at_unix_ms: int | None = None
) -> InvokeResult:
    """Build a business failure envelope; code must be in the closed set.

    Validating locally gives a plugin author an immediate TypeError
    instead of a rejected declaration at the host, where the same code
    is refused and the whole plugin call is degraded.
    """
    if code not in _ERROR_CODES:
        raise ValueError(
            f"error_code {code!r} is outside the closed set "
            f"{'|'.join(sorted(_ERROR_CODES))}"
        )
    return InvokeResult(
        ok=False, error_code=code, error_message=message, at_unix_ms=at_unix_ms
    )


class FunctionPlugin(abc.ABC):
    """Contract a sidecar function adapter plugin implements.

    describe/list_functions/invoke are the three lifecycle calls; the
    host issues describe once after the handshake, list_functions at
    registration and after each crash restart, and invoke per call.
    list_functions is re-read on every registration reconcile, so a
    plugin may declare exactly the endpoints its game interface offers
    right now.
    """

    @abc.abstractmethod
    def describe(self) -> tuple[str, str]:
        """Return (plugin_name, version). The name must match the
        sidecar's configured plugin name: declared ids live under
        plugin.<name>. and are rejected otherwise."""

    @abc.abstractmethod
    def list_functions(self) -> list[FunctionDescriptor]:
        """Return the functions this plugin offers right now."""

    @abc.abstractmethod
    def invoke(self, call: InvokeInput) -> InvokeResult:
        """Map one call onto the game interface.

        call.context.is_active() is False once the host's per-call
        deadline fires or the caller's task is cancelled — go-plugin
        propagates it into this RPC, so a long target wait must poll it
        rather than sleeping past the deadline.
        """


class _FunctionPluginServicer(funcplugin_pb2_grpc.FunctionPluginServicer):
    def __init__(self, impl: FunctionPlugin) -> None:
        self._impl = impl

    def Describe(self, request, context):  # noqa: N802 (grpc naming)
        name, version = self._impl.describe()
        return funcplugin_pb2.DescribeResponse(name=name, version=version)

    def ListFunctions(self, request, context):  # noqa: N802 (grpc naming)
        functions = [d.to_proto() for d in self._impl.list_functions()]
        return funcplugin_pb2.ListFunctionsResponse(functions=functions)

    def Invoke(self, request, context):  # noqa: N802 (grpc naming)
        call = InvokeInput(
            function_id=request.function_id,
            params_json=request.params_json,
            timeout_ms=request.timeout_ms,
            context=context,
        )
        return self._impl.invoke(call).to_proto()


def serve(plugin: FunctionPlugin) -> None:
    """Run one function adapter plugin until the host shuts it down."""
    _serve.serve_plugin(
        COOKIE_KEY,
        COOKIE_VALUE,
        lambda server: funcplugin_pb2_grpc.add_FunctionPluginServicer_to_server(
            _FunctionPluginServicer(plugin), server
        ),
        PROTOCOL_VERSION,
    )
