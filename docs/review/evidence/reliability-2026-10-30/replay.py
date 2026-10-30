#!/usr/bin/env python3
"""Replay defect-confirmation diagnostics; PASS is NOT product acceptance."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile

root = Path(__file__).resolve().parents[4]
evidence = Path(__file__).resolve().parent
pairs = {
    'internal/platform/functioninvoke': 'govern-acl',
    'internal/server': 'task-event-gap',
    'internal/tasks/scheduler': 'scheduler-recovery',
    'internal/db/router': 'router-budget',
    'internal/agent': 'drain-accepted-task',
    'internal/analytics/worker': 'aggregate-race',
}
race = '--race' in sys.argv
with tempfile.TemporaryDirectory(prefix='croupier-review-') as tmp:
    overlay = Path(tmp) / 'overlay.json'
    overlay.write_text(json.dumps({'Replace': {
        str(root / package / 'review_20261030_test.go'): str(evidence / (source + '.go.txt'))
        for package, source in pairs.items()
    }}))
    packages = ['internal/analytics/worker'] if race else [p for p in pairs if not p.endswith('/worker')]
    command = ['go', 'test', '-overlay=' + str(overlay)]
    if race:
        command.append('-race')
    command += ['./' + p for p in packages] + ['-run', '^TestReview20261030', '-count=1', '-v', '-p=1']
    print('Defect diagnostics: a passing assertion confirms the defect; race is expected to report a conflict.', flush=True)
    sys.exit(subprocess.run(command, cwd=root).returncode)
