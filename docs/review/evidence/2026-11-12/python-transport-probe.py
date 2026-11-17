import pathlib
import struct
import sys
from unittest.mock import Mock
root = pathlib.Path(__file__).resolve().parents[4]
sys.path.insert(0, str(root / 'sdks/python'))
from croupier.transport.tcp import TCPTransport, _PendingCall, _MAX_FRAME_BYTES
transport = TCPTransport()
transport._connected = True
transport._running = True
sock = Mock()
sock.recv.return_value = struct.pack('>I', _MAX_FRAME_BYTES + 1)
transport._sock = sock
pending = _PendingCall()
transport._pending[7] = pending
transport._reader_loop()
assert transport._connected is True
assert transport._running is True
assert not pending.event.is_set()
assert pending.error is None
assert 7 in transport._pending
print('REPRODUCED: oversize frame exits Python reader but connected/running remain True and pending call receives no failure')
