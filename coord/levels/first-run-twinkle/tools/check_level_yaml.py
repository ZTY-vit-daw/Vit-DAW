#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""level.yaml 结构自检（279 定稿轮；PyYAML 解析 + 枚举/嵌套/引用/bar 域全检）。

覆盖面=卡面验收标准（YAML 可解析/判定器名全在首版 3 枚举内/素材清单带哈希可对）
+ 设计稿 §1.9 校验门拒绝路径的卡内可判项（1/2/3/4/5/6/7/8/9/10/12 的 YAML 侧先检；
内核交互语义（如 clip 实存）不在此层）。280 校验门（lesson_card.go）落地后本脚本
退位为内容侧预检（lint），不替代校验门。

用法（在 first-run-twinkle/ 目录下）：python tools/check_level_yaml.py [--level level.yaml]
退出码：0=全部通过；1=有失败项（逐项打印）。
"""
import argparse
import re
import sys
from pathlib import Path

import yaml

JUDGES = {"note_sequence_match", "pitch_set_match", "rhythm_match"}          # 首版 3 判定器（闭集）
EVENTS = {"human_replied", "user_confirmed", "user_chose", "user_feedback_given",
          "user_identified", "export_completed"}                             # 交互事件（闭集）
STRUCTURES = {"clip_exists", "track_non_empty", "tempo_set"}                 # 结构谓词（闭集）
TRANSFORMS = {"repeat", "sequence", "inversion", "retrograde"}               # 276 白名单
ANALYZE_PARAM_KEYS = {"clip_id", "track_id", "range_beats", "transformations",
                      "min_match_notes", "key"}                              # 276 parseAnalyzeArgs 字段族
FINDING_ASSERT_KEYS = {"kind", "sequence_quality", "proto_bars", "match_bars",
                       "shift_semitones", "diatonic_steps", "note_count"}    # 教学断言字段（§1.3）
WRITE_COMMANDS_VERIFIED = {                                                  # 定稿轮核对 @main 24f7280a 注册面
    "import_midi_to_track", "create_midi_clip", "add_midi_notes", "add_midi_notes_bulk",
    "mutate_midi_notes", "delete_midi_notes", "apply_midi_note_patch", "set_tempo",
    "project.markers.upsert",
}
COMBINATORS = {"all", "any"}
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
EPS = 1e-9

ok_count = 0
fails = []


def check(cond, msg):
    global ok_count
    if cond:
        ok_count += 1
    else:
        fails.append(msg)


def check_leaves(node_done_when, node_id, depth_ok):
    """递归校验 done_when：叶子枚举 + 组合子深度 ≤1。"""
    if not isinstance(node_done_when, dict):
        fails.append(f"nodes[{node_id}].done_when 非映射")
        return
    keys = set(node_done_when.keys())
    if keys & COMBINATORS:
        if not depth_ok:
            fails.append(f"nodes[{node_id}].done_when 组合子嵌套超过一层")
            return
        for sub in node_done_when[list(keys & COMBINATORS)[0]]:
            check_leaves(sub, node_id, False)
        return
    if "judge" in keys:
        name = node_done_when["judge"]
        check(name in JUDGES, f"nodes[{node_id}].done_when 未知 judge: {name}")
        bad = keys - {"judge", "target", "reference", "accept_transposed"}
        check(not bad, f"nodes[{node_id}] judge 叶未知参数字段: {bad}")
    elif "event" in keys:
        name = node_done_when["event"]
        check(name in EVENTS, f"nodes[{node_id}].done_when 未知 event: {name}")
        bad = keys - {"event", "key", "min_count"}
        check(not bad, f"nodes[{node_id}] event 叶未知参数字段: {bad}")
    elif "structure" in keys:
        name = node_done_when["structure"]
        check(name in STRUCTURES, f"nodes[{node_id}].done_when 未知 structure: {name}")
        bad = keys - {"structure", "clip", "track", "key"}
        check(not bad, f"nodes[{node_id}] structure 叶未知参数字段: {bad}")
    else:
        fails.append(f"nodes[{node_id}].done_when 叶子无 judge/event/structure: {sorted(keys)}")


def check_range(rb, where, bar_span_beats):
    check(isinstance(rb, list) and len(rb) == 2, f"{where}.range_beats 形态非法: {rb}")
    if isinstance(rb, list) and len(rb) == 2:
        s, e = rb
        check(0 <= s < e, f"{where}.range_beats 逆序/零宽/负: {rb}")
        check(e <= bar_span_beats + EPS, f"{where}.range_beats 超出 bar_span 覆盖: {rb}")


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--level", default="level.yaml")
    args = ap.parse_args()
    pkg = Path(args.level).resolve().parent

    # -- PyYAML 解析 --
    try:
        lv = yaml.safe_load(Path(args.level).read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        print(f"FAIL: YAML 解析失败: {exc}")
        return 1
    check(isinstance(lv, dict), "顶层不是映射")

    # -- 顶层键 --
    required = {"schema_version", "id", "title", "lead_default", "players", "timing",
                "persona_slot", "assets", "notes", "nodes", "guidance",
                "write_boundary", "finale", "budget"}
    check(required <= set(lv), f"缺顶层必填键: {required - set(lv)}")
    check(lv.get("schema_version") == "vit.level_card.v0", "schema_version 非 vit.level_card.v0")
    check(lv.get("id") == pkg.name, f"id 与目录名不一致: {lv.get('id')} vs {pkg.name}")
    check(lv.get("lead_default") in ("human", "model"), "lead_default 非 human/model")
    check(set(lv.get("players", [])) == {"human", "model"}, "players 非 [human, model]")

    # -- timing --
    tm = lv.get("timing", {})
    bpb = tm.get("beats_per_bar")
    first_bar, last_bar = tm.get("bar_span", {}).get("first_bar"), tm.get("bar_span", {}).get("last_bar")
    check(isinstance(bpb, int) and bpb > 0, "beats_per_bar 非正整数")
    check(isinstance(first_bar, int) and isinstance(last_bar, int) and 1 <= first_bar <= last_bar,
          "bar_span 非法")
    bar_span_beats = (last_bar - first_bar + 1) * bpb

    # -- assets --
    assets = lv.get("assets", [])
    asset_ids = set()
    for a in assets:
        aid = a.get("id")
        check(aid not in asset_ids, f"assets id 重复: {aid}")
        asset_ids.add(aid)
        if "file" in a:
            check(bool(SHA256_RE.match(a.get("sha256", ""))), f"assets[{aid}] sha256 缺失/非 64hex")
            f = pkg / a["file"]
            check(f.exists(), f"assets[{aid}] 文件不存在: {a['file']}")
        if "derived_from" in a:
            check(a["derived_from"] in asset_ids or True, "")  # 顺序无关，后面统一查
    for a in assets:
        if "derived_from" in a:
            check(a["derived_from"] in asset_ids, f"assets[{a['id']}].derived_from 引用未知 id: {a['derived_from']}")
        if "notes_ref" in a:
            # notes_ref 为卡根起点路径（如 "notes.theme"）：逐段解析到内嵌表
            cur = lv
            for seg in str(a["notes_ref"]).split("."):
                cur = cur.get(seg) if isinstance(cur, dict) else None
            check(isinstance(cur, list) and cur, f"assets[{a['id']}].notes_ref 引用未知 notes 表: {a['notes_ref']}")
        if a.get("kind") == "midi_reference" and "file" in a:
            check("notes_ref" in a or "derived_from" in a,
                  f"assets[{a['id']}] midi_reference 缺 notes_ref（或 derived_from 派生标记）")
        for key in ("key",):
            if key in a:
                k = a[key]
                check(isinstance(k, dict) and 0 <= int(k.get("tonic", -1)) <= 11
                      and k.get("mode", "major") == "major", f"assets[{a['id']}].key 非法: {k}")

    # -- runtime_slots --
    slots = lv.get("runtime_slots", {})
    check(len(slots) == 3, f"runtime_slots 数非 3（§2.3 三轨）: {len(slots)}")
    track_indices = set()
    for sid, s in slots.items():
        check(isinstance(s.get("track_index"), int), f"runtime_slots[{sid}] 缺 track_index")
        ti = s.get("track_index")
        check(ti not in track_indices, f"runtime_slots track_index 重复: {ti}")
        track_indices.add(ti)
        if s.get("preload") is not None:
            check(s["preload"] in asset_ids, f"runtime_slots[{sid}].preload 引用未知 asset: {s['preload']}")

    # -- notes 表 --
    notes = lv.get("notes", {})
    theme = notes.get("theme", [])
    check(len(theme) == 42, f"notes.theme 音数非 42: {len(theme)}")
    last_start, span_end = -1.0, 0.0
    for i, n in enumerate(theme):
        check(set(n) == {"pitch", "start", "length"}, f"notes.theme[{i}] 字段非法: {sorted(n)}")
        check(isinstance(n["pitch"], int) and 0 <= n["pitch"] <= 127, f"notes.theme[{i}] pitch 越界")
        check(n["start"] >= 0 and n["length"] > 0, f"notes.theme[{i}] start/length 非法")
        check(n["start"] >= last_start, f"notes.theme[{i}] start 非递增")
        last_start = n["start"]
        span_end = max(span_end, n["start"] + n["length"])
    check(abs(span_end - bar_span_beats) < EPS,
          f"notes.theme 覆盖末端 {span_end} ≠ bar_span 覆盖 {bar_span_beats}")

    # -- deviations vs notes（损坏点 replaces 必须命中原表音高）--
    for a in assets:
        for d in a.get("deviations", []):
            hit = [n for n in theme if abs(n["start"] - d["at_beat"]) < EPS]
            check(len(hit) == 1 and hit[0]["pitch"] == d["replaces"],
                  f"assets[{a['id']}].deviation {d} 未命中原表音（replaces 应={hit[0]['pitch'] if len(hit)==1 else '?'})")

    # -- objectives --
    node_ids = {n.get("id") for n in lv.get("nodes", [])}
    for o in lv.get("objectives", []):
        c = o.get("check", {})
        if "judge" in c:
            check(c["judge"] in JUDGES, f"objectives[{o.get('id')}] 未知 judge: {c['judge']}")
        if "event" in c:
            check(c["event"] in EVENTS, f"objectives[{o.get('id')}] 未知 event: {c['event']}")
        if c.get("kind") == "verbal_rubric":
            check("rubric" in c, f"objectives[{o.get('id')}] verbal_rubric 缺 rubric")
        if "node" in c:
            check(c["node"] in node_ids, f"objectives[{o.get('id')}] 引用未知 node: {c['node']}")

    # -- nodes --
    nodes = lv.get("nodes", [])
    check(len(nodes) == 7, f"nodes 数非 7: {len(nodes)}")
    seen_ids = set()
    for n in nodes:
        nid = n.get("id")
        check(nid not in seen_ids, f"node.id 重复: {nid}")
        seen_ids.add(nid)
        check(n.get("lead") in ("human", "model"), f"nodes[{nid}].lead 非法")
        dw = n.get("done_when")
        check(dw is not None, f"nodes[{nid}] 缺 done_when")
        if dw is not None:
            check_leaves(dw, nid, True)
        # judge 叶的 target/reference 寻址
        if isinstance(dw, dict) and "judge" in dw:
            tgt, ref = dw.get("target", {}), dw.get("reference", {})
            check(tgt.get("clip") in slots, f"nodes[{nid}] target.clip 引用未知轨槽: {tgt.get('clip')}")
            check(ref.get("asset") in asset_ids, f"nodes[{nid}] reference.asset 引用未知资产: {ref.get('asset')}")
            check_range(tgt.get("range_beats"), f"nodes[{nid}].target", bar_span_beats)
            check_range(ref.get("range_beats"), f"nodes[{nid}].reference", bar_span_beats)
            check(set(tgt) <= {"clip", "range_beats"}, f"nodes[{nid}] target 字段超集")
            check(set(ref) <= {"asset", "clip", "range_beats", "range_bars"}, f"nodes[{nid}] reference 字段超集")
        if isinstance(dw, dict) and "structure" in dw and "clip" in dw:
            check(dw["clip"] in slots, f"nodes[{nid}] structure.clip 引用未知轨槽: {dw['clip']}")

    # -- evidence hooks --
    hook_count = 0
    for n in nodes:
        teaching = n.get("teaching", {})
        for h in teaching.get("evidence_hooks", []):
            hook_count += 1
            hid = f"nodes[{n['id']}].hooks[{h.get('id')}]"
            check(h.get("analyze") == "midi.analyze_transformations", f"{hid} analyze 名非法")
            p = h.get("params", {})
            check(set(p) <= ANALYZE_PARAM_KEYS, f"{hid} 参数超出 276 parseAnalyzeArgs 字段族: {set(p) - ANALYZE_PARAM_KEYS}")
            check(p.get("clip_id") in slots, f"{hid} clip_id 引用未知轨槽: {p.get('clip_id')}")
            if "range_beats" in p:
                check_range(p["range_beats"], hid, bar_span_beats)
            if "transformations" in p:
                check(set(p["transformations"]) <= TRANSFORMS, f"{hid} transformations 白名单外")
            if "min_match_notes" in p:
                m = p["min_match_notes"]
                check(isinstance(m, int) and (m == 0 or m >= 2), f"{hid} min_match_notes <2 或非整数")
            if "key" in p:
                k = p["key"]
                check(0 <= int(k.get("tonic", -1)) <= 11 and k.get("mode", "major") == "major",
                      f"{hid} key 非法: {k}")
            if not ({"expected_findings", "expect_absent", "expect_present"} & set(h)):
                fails.append(f"{hid} 缺 expected_findings/expect_absent/expect_present 之一")
            for section in ("expected_findings", "expect_absent", "expect_present"):
                for f_ in h.get(section, []):
                    check(set(f_) <= FINDING_ASSERT_KEYS,
                          f"{hid}.{section} 断言字段超集: {set(f_) - FINDING_ASSERT_KEYS}")
                    check(f_.get("kind") in TRANSFORMS, f"{hid}.{section} kind 非法: {f_.get('kind')}")
                    if "sequence_quality" in f_:
                        check(f_["sequence_quality"] in ("chromatic", "diatonic"),
                              f"{hid}.{section} sequence_quality 非法")
                    for bk in ("proto_bars", "match_bars"):
                        if bk in f_:
                            b = f_[bk]
                            check(isinstance(b, list) and len(b) == 2
                                  and first_bar <= b[0] <= b[1] <= last_bar,
                                  f"{hid}.{section}.{bk} 越界/形态非法: {b}")

    # -- guidance / write_boundary / finale / budget / persona --
    g = lv.get("guidance", {})
    check(isinstance(g.get("pullback_after_turns"), int) and g["pullback_after_turns"] > 0,
          "guidance.pullback_after_turns 非正整数")
    wb = lv.get("write_boundary", {})
    allow = wb.get("allow", [])
    check(set(allow) <= WRITE_COMMANDS_VERIFIED,
          f"write_boundary.allow 含未注册命令名: {set(allow) - WRITE_COMMANDS_VERIFIED}")
    check(len(set(allow)) == len(allow), "write_boundary.allow 有重复项")
    check(wb.get("confirm_downgrade") == "none", "confirm_downgrade 非 none（用户裁定=不降级）")
    fin = lv.get("finale", {})
    check(fin.get("completion_before_export") is True, "finale.completion_before_export 非 true")
    npb = fin.get("closure_check", {}).get("structure_complete", {}).get("no_placeholder_blanks", [])
    check(all(x in slots for x in npb), f"finale no_placeholder_blanks 引用未知轨槽: {npb}")
    check(fin.get("user_direct_export") == "skip_ask", "finale.user_direct_export 非 skip_ask")
    check(fin.get("edges", {}).get("completion_neq_stop") is True
          and fin["edges"].get("completion_neq_freeze") is True, "finale.edges 两边界缺一")
    bud = lv.get("budget", {})
    check(isinstance(bud.get("max_turns"), int) and bud["max_turns"] > 0, "budget.max_turns 非法")
    check(isinstance(bud.get("max_llm_calls"), int) and bud["max_llm_calls"] > 0, "budget.max_llm_calls 非法")
    check(bool(bud.get("on_stop_loss")), "budget.on_stop_loss 缺失")
    ps = lv.get("persona_slot", {})
    check((pkg / ps.get("source", "")).exists(), f"persona_slot.source 文件不存在: {ps.get('source')}")

    # -- MANIFEST 哈希对账 --
    man_path = pkg / "assets" / "MANIFEST.yaml"
    check(man_path.exists(), "assets/MANIFEST.yaml 不存在")
    if man_path.exists():
        import hashlib
        man = yaml.safe_load(man_path.read_text(encoding="utf-8"))
        man_by_path = {f["path"]: f for f in man.get("files", [])}
        for a in assets:
            if "file" in a:
                f = pkg / a["file"]
                real = hashlib.sha256(f.read_bytes()).hexdigest()
                check(a["sha256"] == real, f"assets[{a['id']}] sha256 与实际文件不符")
                m = man_by_path.get(a["file"])
                check(m is not None and m.get("sha256") == real, f"MANIFEST 缺 {a['file']} 或哈希不符")

    # -- 汇总 --
    print(f"checks passed: {ok_count}; failed: {len(fails)}; evidence_hooks: {hook_count}")
    for msg in fails:
        print(f"FAIL: {msg}")
    return 1 if fails else 0


if __name__ == "__main__":
    sys.exit(main())
