import sys
from pyfixmsg.reference import FixSpec
from pyfixmsg.codecs.stringfix import Codec
from pyfixmsg.fixmessage import FixMessage

spec = FixSpec("examples/FIX44.xml")
codec = Codec(spec=spec)
msg_str = "8=FIX.4.4\x019=71\x0135=A\x0134=1\x0149=EXEC\x0152=20250615-01:11:14.252\x0156=BANZAI\x0198=0\x01108=30\x0110=234\x01"

fields = codec.parse(msg_str)
print("Codec parsed fields:", fields)

try:
    msg = FixMessage()
    msg.from_wire(msg_str, codec=codec)
    print("Sanity check parsed message:", msg)
    if not msg:
        raise ValueError("Parsed message is empty")
except Exception as e:
    print("Exception during parsing:", e)
    sys.exit("ERROR: pyfixmsg failed to parse valid FIX44 logon message!")