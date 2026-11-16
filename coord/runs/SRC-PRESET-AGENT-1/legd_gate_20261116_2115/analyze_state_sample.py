# 384 leg D gate analyzer: decode the boot XML's PLUGIN state attr with the
# LEG-E solved JUCE alphabet and classify the content layer vs the 309 form.
import hashlib
import json
import math
import re
import sys
from collections import Counter

JUCE_ALPHABET = ".ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+"
IDX = {c: i for i, c in enumerate(JUCE_ALPHABET)}

XML_MAGIC_LE = bytes([0x56, 0x43, 0x32, 0x21])  # little-endian 0x21324356
T309_HEAD = bytes([0x58, 0xD9, 0x0C])


def decode_juce_b64(blob, size):
    bits = 0
    nbits = 0
    out = bytearray()
    for ch in blob:
        if ch not in IDX:
            raise ValueError("char outside JUCE alphabet: %r" % ch)
        bits = (bits << 6) | IDX[ch]
        nbits += 6
        while nbits >= 8:
            nbits -= 8
            out.append((bits >> nbits) & 0xFF)
    return bytes(out[:size])


def entropy(data):
    if not data:
        return 0.0
    c = Counter(data)
    n = len(data)
    return -sum(v / n * math.log2(v / n) for v in c.values())


def template_head(path, n=32):
    """Decode the 309-era template state blob head for byte-identity checks."""
    text = open(path, "rb").read().decode("utf-8", errors="replace")
    m = re.search(r'state="(\d+)\.([A-Za-z0-9+.]+)"', text)
    if not m:
        return None
    raw = decode_juce_b64(m.group(2), int(m.group(1)))
    return raw[:n]


def classify(raw, t309_head):
    head = raw[:16]
    if raw.startswith(XML_MAGIC_LE) and b"<VST3PluginState" in raw[:4096]:
        return "xml_wrapper", head
    if t309_head and raw.startswith(t309_head):
        return "binary_309_identical", head
    if raw.startswith(T309_HEAD):
        return "binary_309_family", head
    if raw.startswith(b"sub3"):
        return "raw_save_patch", head
    return "new_form", head


def main(xml_path, out_json, t309_xml=None):
    text = open(xml_path, "rb").read().decode("utf-8", errors="replace")
    m = re.search(r'state="(\d+)\.([A-Za-z0-9+.]+)"', text)
    if not m:
        json.dump({"error": "no state attr found", "path": xml_path},
                  open(out_json, "w", encoding="utf-8"), indent=1)
        return 1
    size, blob = int(m.group(1)), m.group(2)
    raw = decode_juce_b64(blob, size)
    t309_head = template_head(t309_xml) if t309_xml else None
    verdict, head = classify(raw, t309_head)
    ascii_runs = re.findall(rb"[ -~]{12,}", raw[:8192])
    result = {
        "path": xml_path,
        "declared_size": size,
        "decoded_size": len(raw),
        "blob_sha256": hashlib.sha256(blob.encode("ascii")).hexdigest(),
        "raw_sha256": hashlib.sha256(raw).hexdigest(),
        "head_hex": raw[:32].hex(),
        "t309_head_hex": t309_head.hex() if t309_head else None,
        "verdict": verdict,
        "magic_hits": {name.decode(): raw.find(name) for name in
                       [b"sub3", b"<?xml", b"<VST3PluginState", b"CcnK", b"cjs3"]},
        "entropy_head_1k": round(entropy(raw[:1024]), 3),
        "ascii_runs_head": [r[:48].decode("ascii", "replace") for r in ascii_runs[:5]],
        "xml_wrapper_text": raw[:200].decode("utf-8", "replace")
        if verdict == "xml_wrapper" else "",
    }
    json.dump(result, open(out_json, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print(json.dumps(result, ensure_ascii=False, indent=1))
    return 0


if __name__ == "__main__":
    t309 = sys.argv[3] if len(sys.argv) > 3 else None
    sys.exit(main(sys.argv[1], sys.argv[2], t309))
