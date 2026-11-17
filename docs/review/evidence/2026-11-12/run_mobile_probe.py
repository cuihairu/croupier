import os
import pathlib
import shutil
import subprocess
import tempfile
root = pathlib.Path(__file__).resolve().parents[4]
here = pathlib.Path(__file__).resolve().parent
flutter = os.environ.get('FLUTTER_BIN') or shutil.which('flutter') or str(pathlib.Path.home()/'flutter/bin/flutter')
with tempfile.TemporaryDirectory(prefix='croupier-mobile-audit-') as folder:
    source = pathlib.Path(folder)/'audit_test.dart'
    source.write_text((here/'mobile_probe.dart.txt').read_text())
    result = subprocess.run([flutter, 'test', '--no-pub', str(source), '--reporter', 'expanded'],
                            cwd=root/'mobile', text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    (here/'mobile-probe.log').write_text(result.stdout)
    print(result.stdout, end='')
    raise SystemExit(result.returncode)
