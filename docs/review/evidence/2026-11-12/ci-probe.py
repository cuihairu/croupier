"""Extract and execute the real mobile coverage gate shell block in a temp directory."""
import pathlib
import subprocess
import tempfile
import textwrap
root = pathlib.Path(__file__).resolve().parents[4]
workflow = (root / '.github/workflows/ci-mobile.yml').read_text()
block = workflow.split('      - name: Coverage gate\n        run: |\n', 1)[1].split('      # 上传', 1)[0]
script = textwrap.dedent(block)
with tempfile.TemporaryDirectory(prefix='croupier-mobile-ci-audit-') as folder:
    p = pathlib.Path(folder) / 'coverage'
    p.mkdir()
    (p/'lcov.info').write_text('SF:lib/probe.dart\nLF:10\nLH:10\nend_of_record\n')
    res = subprocess.run(['bash', '-c', script], cwd=folder, capture_output=True, text=True)
    print(res.stdout, end='')
    print(res.stderr, end='')
    assert res.returncode != 0 and 'ValueError' in res.stderr and '${COVERAGE_THRESHOLD:-80}' in res.stderr
    print('REPRODUCED: 100% coverage still fails because quoted heredoc passes literal shell parameter expression to float()')
