"""In-process contract tests: envelope helpers, dataclass→proto mapping,
servicer wiring and the serve-module handshake gates. The cross-process
proof lives in test_handshake.py and test_integration_go_host.py."""

from __future__ import annotations

import pytest

import croupier_plugin as cp
from croupier_plugin import funcplugin_pb2, probeplugin_pb2
from croupier_plugin import serve as serve_mod
from croupier_plugin.funcplugin import (
    _FunctionPluginServicer,
    _now_ms,
    error_result,
    ok_result,
)
from croupier_plugin.probeplugin import _ProbePluginServicer


class _ActiveContext:
    def is_active(self) -> bool:
        return True


class _RecordingPlugin(cp.FunctionPlugin):
    def __init__(self) -> None:
        self.calls: list[cp.InvokeInput] = []

    def describe(self) -> tuple[str, str]:
        return "pyecho", "9.9.9"

    def list_functions(self) -> list[cp.FunctionDescriptor]:
        return [
            cp.FunctionDescriptor(
                id="plugin.pyecho.echo",
                summary="echo",
                param_schema='{"type":"object"}',
                result_schema='{"type":"object"}',
                risk_level=cp.RISK_MEDIUM,
            )
        ]

    def invoke(self, call: cp.InvokeInput) -> cp.InvokeResult:
        self.calls.append(call)
        return error_result(cp.TARGET_REJECTED, "recorded")


def test_envelope_helpers() -> None:
    ok = ok_result('{"message":"hi"}', at_unix_ms=1234)
    assert ok.ok and ok.result_json == '{"message":"hi"}'
    proto = ok.to_proto()
    assert proto.ok is True and proto.at_unix_ms == 1234

    err = error_result(cp.TARGET_UNAVAILABLE, "down")
    assert err.ok is False and err.error_code == "target_unavailable"
    err_proto = err.to_proto()
    assert err_proto.error_message == "down"
    assert err_proto.at_unix_ms > 0  # defaults to now


def test_error_result_rejects_codes_outside_the_closed_set() -> None:
    with pytest.raises(ValueError, match="closed set"):
        error_result("totally_novel_code", "leaks a new taxonomy")


def test_function_descriptor_maps_to_proto() -> None:
    d = cp.FunctionDescriptor(
        id="plugin.pyecho.echo",
        summary="echo",
        param_schema='{"type":"object"}',
        result_schema='{"type":"object"}',
        error_schema='{"type":"object"}',
        risk_level=cp.RISK_DANGER,
    )
    proto = d.to_proto()
    assert proto.id == "plugin.pyecho.echo"
    assert proto.risk_level == "danger"
    assert proto.error_schema == '{"type":"object"}'


def test_function_servicer_maps_all_three_calls() -> None:
    plugin = _RecordingPlugin()
    servicer = _FunctionPluginServicer(plugin)

    described = servicer.Describe(funcplugin_pb2.DescribeRequest(), _ActiveContext())
    assert (described.name, described.version) == ("pyecho", "9.9.9")

    listed = servicer.ListFunctions(
        funcplugin_pb2.ListFunctionsRequest(), _ActiveContext()
    )
    assert [f.id for f in listed.functions] == ["plugin.pyecho.echo"]
    assert listed.functions[0].risk_level == "medium"

    invoked = servicer.Invoke(
        funcplugin_pb2.InvokeRequest(
            function_id="plugin.pyecho.echo",
            params_json='{"message":"x"}',
            timeout_ms=1500,
        ),
        _ActiveContext(),
    )
    assert invoked.ok is False
    assert invoked.error_code == "target_rejected"
    assert len(plugin.calls) == 1
    call = plugin.calls[0]
    assert (call.function_id, call.params_json, call.timeout_ms) == (
        "plugin.pyecho.echo",
        '{"message":"x"}',
        1500,
    )
    assert isinstance(call.context, _ActiveContext)


class _ProbeRecordingPlugin(cp.ProbePlugin):
    def describe(self) -> tuple[str, str]:
        return "probe-echo", "2.0.0"

    def check(self, call: cp.CheckInput) -> cp.CheckResult:
        return cp.PassResult(
            status=200, latency_ms=3, detail=f"checked {call.target}"
        )


def test_probe_servicer_maps_describe_and_check() -> None:
    servicer = _ProbePluginServicer(_ProbeRecordingPlugin())

    described = servicer.Describe(probeplugin_pb2.DescribeRequest(), _ActiveContext())
    assert (described.name, described.version) == ("probe-echo", "2.0.0")

    result = servicer.Check(
        probeplugin_pb2.CheckRequest(
            target="ok-target", timeout_ms=5000, payload_json="{}"
        ),
        _ActiveContext(),
    )
    assert result.ok is True
    assert result.status == 200
    assert result.detail == "checked ok-target"
    assert result.at_unix_ms > 0


def test_probe_result_subclass_constructors() -> None:
    passed = cp.PassResult(status=204, latency_ms=7)
    assert passed.ok and passed.error == ""
    failed = cp.FailResult("boom", status=503, detail="d")
    assert not failed.ok and failed.error == "boom" and failed.status == 503
    proto = failed.to_proto()
    assert proto.ok is False and proto.at_unix_ms > 0


# -- serve module gates ------------------------------------------------------


def test_negotiate_protocol_version(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("PLUGIN_PROTOCOL_VERSIONS", raising=False)
    assert serve_mod.negotiate_protocol_version(1) == 1

    monkeypatch.setenv("PLUGIN_PROTOCOL_VERSIONS", " 2 , 1 ")
    assert serve_mod.negotiate_protocol_version(1) == 1

    monkeypatch.setenv("PLUGIN_PROTOCOL_VERSIONS", "2,3")
    with pytest.raises(serve_mod.PluginHandshakeError, match="speaks plugin protocol"):
        serve_mod.negotiate_protocol_version(1)

    monkeypatch.setenv("PLUGIN_PROTOCOL_VERSIONS", "1,x")
    with pytest.raises(serve_mod.PluginHandshakeError, match="invalid plugin protocol"):
        serve_mod.negotiate_protocol_version(1)


def test_require_cookie_gate(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("T_KEY", "right")
    serve_mod._require_cookie("T_KEY", "right")  # passes

    with pytest.raises(SystemExit):
        serve_mod._require_cookie("T_KEY", "wrong")
    with pytest.raises(serve_mod.PluginHandshakeError):
        serve_mod._require_cookie("", "right")


def test_handshake_line_builder(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("PLUGIN_MULTIPLEX_GRPC", raising=False)
    line = serve_mod._handshake_line(1, "unix", "/tmp/pluginx/grpc.sock")
    assert line == "1|1|unix|/tmp/pluginx/grpc.sock|grpc|"

    monkeypatch.setenv("PLUGIN_MULTIPLEX_GRPC", "1")
    line = serve_mod._handshake_line(1, "unix", "/tmp/pluginx/grpc.sock")
    assert line.endswith("|true")


def test_now_ms_is_unix_milliseconds() -> None:
    assert _now_ms() > 1_600_000_000_000
