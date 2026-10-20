"""Full host⇄plugin round trips: the REAL Go hosts (the sidecar
function host and the synthetics probe controller that production code
uses) drive the reference Python plugins through the real go-plugin
handshake, via the Go hostdriver program.

pytest is only the runner and the Go toolchain provisioner — every
assertion lives in hostdriver so Go and pytest agree on one vocabulary.
Skipped only when no Go toolchain is installed (CI installs one)."""

from __future__ import annotations

import shutil
import subprocess
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[1]
REPO_ROOT = Path(__file__).resolve().parents[4]

go_bin = shutil.which("go")

pytestmark = pytest.mark.skipif(go_bin is None, reason="go toolchain not installed")


@pytest.fixture(scope="module")
def hostdriver() -> Path:
    assert go_bin is not None
    out = Path("/tmp") / "croupier-hostdriver-pytest"
    built = subprocess.run(
        [go_bin, "build", "-o", str(out), "./sdks/plugins/python/hostdriver"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        timeout=300,
    )
    if built.returncode != 0:
        pytest.fail(f"building hostdriver failed: {built.stderr}")
    return out


def _drive(driver: Path, mode: str, scenario: str, script: Path) -> str:
    run = subprocess.run(
        [
            str(driver),
            "-mode",
            mode,
            "-scenario",
            scenario,
            "-command",
            sys.executable,
            "-args",
            str(script),
        ],
        capture_output=True,
        text=True,
        timeout=180,
    )
    assert run.returncode == 0, (
        f"{mode}/{scenario} failed rc={run.returncode}\n"
        f"stdout: {run.stdout}\nstderr: {run.stderr}"
    )
    return run.stdout.strip()


@pytest.mark.parametrize(
    ("scenario", "script"),
    [
        ("func-register", "func_echo.py"),
        ("func-invoke", "func_echo.py"),
        ("func-envelope", "func_echo.py"),
        ("func-timeout", "func_echo.py"),
    ],
)
def test_function_host_scenarios(
    hostdriver: Path, scenario: str, script: str
) -> None:
    out = _drive(hostdriver, "func", scenario, ROOT / "examples" / script)
    assert out.startswith(f"PASS {scenario}"), out


@pytest.mark.parametrize(
    ("scenario", "script"),
    [
        ("probe-check-ok", "probe_echo.py"),
        ("probe-check-fail", "probe_echo.py"),
        ("probe-version", "probe_echo.py"),
    ],
)
def test_probe_controller_scenarios(
    hostdriver: Path, scenario: str, script: str
) -> None:
    out = _drive(hostdriver, "probe", scenario, ROOT / "examples" / script)
    assert out.startswith(f"PASS {scenario}"), out
