# offline self-test for render_gain_diag.py (no kernel needed)
import importlib.util
import struct
import sys
import tempfile
import wave
import xml.etree.ElementTree as ET
from pathlib import Path

MODULE = r"D:\Vit_DAW_wt_rendergain1\coord\runs\KERNEL-RENDERGAIN-1\render_gain_diag.py"
spec = importlib.util.spec_from_file_location("rgd", MODULE)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)

tmp = Path(tempfile.mkdtemp())

# source + analysis roundtrip
p = tmp / "src.wav"
m.write_sine_wav(p, 440.0, 0.5, 48000, 2)
st = m.analyze_wav(p)
print("source stats:", {k: st[k] for k in ("rate", "sampwidth", "channels", "peak", "amp440")})

# synthetic +6.0 dB scaled copy proves ratio detection
p2 = tmp / "x2.wav"
with wave.open(str(p), "rb") as r:
    raw = r.readframes(r.getnframes())
    rate = r.getframerate()
    nch = r.getnchannels()
vals = struct.unpack("<%dh" % (len(raw) // 2), raw)
scaled = tuple(min(32767, int(round(v * m.GAIN_POS_100))) for v in vals)
with wave.open(str(p2), "wb") as w:
    w.setnchannels(nch)
    w.setsampwidth(2)
    w.setframerate(rate)
    w.writeframes(struct.pack("<%dh" % len(scaled), *scaled))
st2 = m.analyze_wav(p2)
ratio = st2["amp440"][0] / 0.5
print("scaled ratio:", round(ratio, 6), "expected:", round(m.GAIN_POS_100, 6), "close:", m.close(ratio, m.GAIN_POS_100))

# fixtures parse as XML and carry the expected master volume block
for name, vol in (("pos_100", "1.0"), ("pos_0db", "0.7408182206817179"), ("absent", None), ("attr_absent", "__absent_attr__")):
    ph = tmp / ("fixture_%s.vit" % name)
    m.write_fixture(ph, "test" + name, vol)
    ET.parse(ph)
    print(name, "xml ok; has MASTERVOLUME:", "MASTERVOLUME" in ph.read_text(encoding="utf-8"))

xml = '<X><MASTERVOLUME><PLUGIN type="volume" id="1" volume="0.74"><M/></PLUGIN></MASTERVOLUME></X>'
print("snapshot parse:", m.snapshot_master_volume({"snapshot_xml": xml}))
print("snapshot absent:", m.snapshot_master_volume({"snapshot_xml": "<X/>"}))
print("write_sine mono:", m.analyze_wav((lambda: (m.write_sine_wav(tmp / "mono.wav", 440.0, 0.5, 48000, 1), tmp / "mono.wav")[1])())["channels"])
print("OK")
