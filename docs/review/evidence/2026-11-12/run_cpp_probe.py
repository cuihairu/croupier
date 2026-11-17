import pathlib
import subprocess
import tempfile
root = pathlib.Path(__file__).resolve().parents[4]
here = pathlib.Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix='croupier-cpp-audit-') as folder:
    binary = pathlib.Path(folder) / 'probe'
    cmd = ['g++', '-std=c++17', '-pthread', '-ffunction-sections', '-fdata-sections',
           '-I', str(root/'sdks/cpp/include'), str(here/'cpp-short-send.cpp'),
           str(root/'sdks/cpp/src/tcp_transport_inbound.cpp'),
           '-Wl,--gc-sections', '-Wl,--wrap=send', '-o', str(binary)]
    result = subprocess.run(cmd, capture_output=True, text=True)
    if not result.returncode:
        result = subprocess.run([str(binary)], capture_output=True, text=True)
    output = result.stdout + result.stderr
    (here/'cpp-probe.log').write_text(output)
    print(output, end='')
    raise SystemExit(result.returncode)
