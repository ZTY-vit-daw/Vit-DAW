import json, sys
import psutil
names = sys.argv[1:]
rows = []
for p in psutil.process_iter(["pid", "name"]):
    try:
        nm = (p.info.get("name") or "").lower()
    except Exception:
        continue
    if nm == "vitagent.exe":
        try:
            env = p.environ()
            rows.append({"pid": p.pid, "vars": dict((n, env.get(n)) for n in names)})
        except Exception as e:
            rows.append({"pid": p.pid, "error": repr(e)})
print(json.dumps(rows))
