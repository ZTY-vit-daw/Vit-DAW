# -*- coding: utf-8 -*-
"""Dry-run the genapi config generation block of run_yue_wire_smoke.ps1:
extract the block, execute it under fake paths, parse the JSON, and assert
the engine paths decode back to real Windows paths (JSON escaping check)."""
import io
import json
import os
import subprocess
import sys
import tempfile

SRC = os.path.join(os.path.dirname(__file__), "..", "..", "..", "scripts", "run_yue_wire_smoke.ps1")
src = io.open(os.path.abspath(SRC), encoding="utf-8").read()
start = src.index("$engineExe = Join-Path")
end = src.index('Add-Prereq ("genapi_config=', start)
block = src[start:end]

tmp = tempfile.mkdtemp(prefix="yuewire_cfg_")
cfg_path = os.path.join(tmp, "genapi_providers.json")
prelude = (
    "$EngineRoot = 'E:\\AI\\YuE2'\n"
    "$DownloadDir = '%s'\n"
    "$EngineStdout = '%s'\n"
    "$EngineStderr = '%s'\n"
    "$GenapiConfig = '%s'\n"
    "$EngineReadyBudgetSeconds = 300\n"
) % (
    os.path.join(tmp, "downloads"),
    os.path.join(tmp, "engine_supervisor_stdout.log"),
    os.path.join(tmp, "engine_supervisor_stderr.log"),
    cfg_path,
)
ps1 = os.path.join(tmp, "gen.ps1")
io.open(ps1, "w", encoding="utf-8-sig").write(prelude + block + "\n")
out = subprocess.run(
    ["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", ps1],
    capture_output=True, timeout=60,
)
if out.returncode != 0:
    print(out.stdout.decode("gbk", "replace"))
    print(out.stderr.decode("gbk", "replace"))
    sys.exit(1)
doc = json.loads(io.open(cfg_path, encoding="utf-8-sig").read())
p = doc["providers"][0]
e = p["engine"]
print("id:", p["id"], "dialect:", p["dialect"])
print("command:", e["command"])
print("model arg:", e["args"][1])
print("working_dir:", e["working_dir"])
print("stdout_log:", e["stdout_log"])
assert e["command"] == r"E:\AI\YuE2\engine\yue-server.exe", e["command"]
assert e["args"][1] == r"E:\AI\YuE2\models\YuE2-3B-Q8_0.gguf", e["args"][1]
assert e["working_dir"] == r"E:\AI\YuE2\engine", e["working_dir"]
assert e["stdout_log"].endswith("engine_supervisor_stdout.log"), e["stdout_log"]
assert p["download_dir"].endswith("downloads"), p["download_dir"]
assert p["extra_params"]["abc"] == "POLLUTED-ABC"
assert e["ready_timeout_seconds"] == 300
print("JSON_CFG_DRYRUN_OK")
