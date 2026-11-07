#!/usr/bin/env python3
"""FIXTURE-WE-HYGIENE-1 rig inputs (294).

Extracts the kernel project-state capture from the read-only 265 run artifact
(coord/runs/D2-STEREO-THIN-1/reverify1/d2_probe_chat.json, executed_kernel_reply
result node) and builds the four snapshot variants for the offline read-path
verification:
  v1_control_orig  = pristine fixture snapshot (Sep-5 state, WE slot _1055)
  v2_control_flip  = v1 + latest_request flipped to the real 266 run identity
                     (mixboard_l2_render_probe_1027_20261106T171511.867643400)
  v3_hyg           = hygienized snapshot (card-named three removals)
  v4_hyg_flip      = v3 + same lr flip (post-hygiene worst case, bridge off)

Also extracts acoustic package (1007,1011) (the bridge package) from BOTH the
orig and hygienized acoustic stores and asserts they are identical.
"""
import json
from pathlib import Path

WS = Path(r"D:/Vit_DAW_ws_294hyg")
RIG = WS / "rig"
RUN_CHAT = Path(r"D:/Vit_DAW/coord/runs/D2-STEREO-THIN-1/reverify1/d2_probe_chat.json")

FLIP_REQ_ID = "mixboard_l2_render_probe_1027_20261106T171511.867643400"
FLIP_CREATED = "2026-11-06T17:15:11.8676434Z"


def main():
    RIG.mkdir(parents=True, exist_ok=True)

    # --- project state from 265 run artifact (read-only source) ---
    d = json.load(open(RUN_CHAT, encoding="utf-8-sig"))
    replies = d.get("executed_kernel_reply") or d["executed_kernel_reply"]
    result = replies[0]["result"]
    assert isinstance(result.get("tracks"), list) and len(result["tracks"]) >= 6
    (RIG / "project_state_265.json").write_text(
        json.dumps(result, ensure_ascii=False), encoding="utf-8")
    tracks = result["tracks"]
    pairs = sorted((str(t.get("track_id")), tuple(sorted(str(c.get("clip_id")) for c in (t.get("clips") or [])))) for t in tracks)
    print("project_state tracks:", pairs)

    # --- snapshot variants ---
    orig = json.loads((WS / "orig/mixboard_feature_snapshot.json").read_text(encoding="utf-8"))
    hyg = json.loads((WS / "hyg/mixboard_feature_snapshot.json").read_text(encoding="utf-8"))

    piano_path = None
    for p in json.loads((WS / "orig/acoustic_package_status.json").read_text(encoding="utf-8"))["packages"]:
        if str(p.get("clip_id")) == "1031":
            piano_path = p.get("source_path")
    assert piano_path and "piano.wav" in piano_path

    def flipped_lr(lr):
        out = json.loads(json.dumps(lr))
        tgt = out["resolved_target"]
        tgt["track_id"] = "1027"
        tgt["track_name"] = "piano"
        tgt["clip_id"] = "1031"
        tgt["clip_name"] = "piano"
        tgt["file_path"] = piano_path
        out["request_id"] = FLIP_REQ_ID
        out["created_at"] = FLIP_CREATED
        return out

    def write_variant(name, snap, flip):
        v = json.loads(json.dumps(snap))
        if flip:
            v["latest_request"] = flipped_lr(v["latest_request"])
        (RIG / name).write_text(json.dumps(v, ensure_ascii=False, indent="\t") + "\n", encoding="utf-8")

    write_variant("v1_control_orig.json", orig, False)
    write_variant("v2_control_flip.json", orig, True)
    write_variant("v3_hyg.json", hyg, False)
    write_variant("v4_hyg_flip.json", hyg, True)
    lr = orig["latest_request"]["resolved_target"]
    print("orig lr target:", lr.get("track_id"), lr.get("clip_id"), str(lr.get("file_path")).split("\\")[-1])

    # --- acoustic bridge package (1007,1011), orig vs hyg identical ---
    def pkg_1011(path):
        for p in json.loads(Path(path).read_text(encoding="utf-8"))["packages"]:
            if str(p.get("clip_id")) == "1011":
                return p
        raise SystemExit("1011 package missing")
    p_o = pkg_1011(WS / "orig/acoustic_package_status.json")
    p_h = pkg_1011(WS / "hyg/acoustic_package_status.json")
    assert json.dumps(p_o, sort_keys=True) == json.dumps(p_h, sort_keys=True), "1011 package differs orig vs hyg"
    (RIG / "acoustic_pkg_1007_1011.json").write_text(
        json.dumps(p_o, ensure_ascii=False), encoding="utf-8")
    print("acoustic bridge package (1007,1011): identical in orig and hyg; l1_static layers:",
          list((p_o.get("package_layers") or {}).keys()))


if __name__ == "__main__":
    main()
