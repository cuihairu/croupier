"""Test bootstrap: make croupier_plugin importable from a plain
checkout (no pip install needed) and give subprocesses a PYTHONPATH that
resolves it the same way."""

import os
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

os.environ["PYTHONPATH"] = str(ROOT) + os.pathsep + os.environ.get("PYTHONPATH", "")
