"""Croupier plugin SDK for Python.

Python plugins for the Croupier go-plugin hosts: sidecar function
adapter plugins (:mod:`croupier_plugin.funcplugin`) and synthetics
probe plugins (:mod:`croupier_plugin.probeplugin`). Both ride the
go-plugin local gRPC protocol implemented in :mod:`croupier_plugin.serve`.
"""

from .funcplugin import (
    BAD_RESPONSE,
    PROTOCOL_VERSION as FUNC_PROTOCOL_VERSION,
    RISK_DANGER,
    RISK_HIGH,
    RISK_LOW,
    RISK_MEDIUM,
    TARGET_REJECTED,
    TARGET_UNAVAILABLE,
    FunctionDescriptor,
    FunctionPlugin,
    InvokeInput,
    InvokeResult,
    error_result,
    ok_result,
    serve as serve_function_plugin,
)
from .probeplugin import (
    PROTOCOL_VERSION as PROBE_PROTOCOL_VERSION,
    CheckInput,
    CheckResult,
    FailResult,
    PassResult,
    ProbePlugin,
    serve as serve_probe_plugin,
)

__all__ = [
    "BAD_RESPONSE",
    "FUNC_PROTOCOL_VERSION",
    "PROBE_PROTOCOL_VERSION",
    "RISK_DANGER",
    "RISK_HIGH",
    "RISK_LOW",
    "RISK_MEDIUM",
    "TARGET_REJECTED",
    "TARGET_UNAVAILABLE",
    "CheckInput",
    "CheckResult",
    "FailResult",
    "FunctionDescriptor",
    "FunctionPlugin",
    "InvokeInput",
    "InvokeResult",
    "PassResult",
    "ProbePlugin",
    "error_result",
    "ok_result",
    "serve_function_plugin",
    "serve_probe_plugin",
]
