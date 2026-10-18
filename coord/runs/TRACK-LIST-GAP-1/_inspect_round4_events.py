import json
import sys
from pathlib import Path

path = Path(r"D:\Vit_DAW\coord\runs\M2-EXPERIENCE-1\events_round4_1817.json")
raw = path.read_bytes().decode("utf-8", errors="replace")
try:
    doc = json.loads(raw)
except json.JSONDecodeError as exc:
    print("JSON decode failed:", exc, file=sys.stderr)
    sys.exit(2)

events = doc.get("events") if isinstance(doc, dict) else doc
print("top-level type:", type(doc).__name__, "events:", len(events) if events else None, "next_seq:", doc.get("next_seq") if isinstance(doc, dict) else None)

def short(value, limit=220):
    text = json.dumps(value, ensure_ascii=False) if not isinstance(value, str) else value
    return text[:limit]

for event in events:
    seq = event.get("seq")
    if seq is None or seq > 12:
        continue
    payload = event.get("payload") or {}
    result = payload.get("result") if isinstance(payload, dict) else None
    interesting = {
        "seq": seq,
        "type": event.get("type"),
        "status": event.get("status"),
        "tool": payload.get("tool") if isinstance(payload, dict) else None,
        "command_name": payload.get("command_name") if isinstance(payload, dict) else None,
        "item_type": event.get("item_type"),
    }
    print("\n---", json.dumps({k: v for k, v in interesting.items() if v is not None}, ensure_ascii=False))
    if isinstance(payload, dict):
        print("  payload keys:", list(payload.keys()))
        for key in ("command", "command_raw", "command_name", "result", "vsp"):
            if key in payload:
                print(f"  {key}: {short(payload[key], 900)}")
        if isinstance(payload.get("result"), dict):
            res = payload["result"]
            for key in ("status", "track_count", "user_track_count", "tracks", "command_ack", "vsp"):
                if key in res:
                    print(f"    result.{key}: {short(res[key], 900)}")
    if event.get("body"):
        print("  body:", short(event["body"], 300))
