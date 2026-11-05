"""Offline unit tests for the card 252 three-state guidance check
(SMOKE-GUIDANCE-CHECK-DISCRIM-1).

Fixture derivation: the all-arbitration telemetry rows are copied verbatim
from the 251 archived run coord/runs/M3-GROUP3-DISCRIMINATE-1/g3run1/
agent_llm_telemetry.jsonl (13/13 rows source="ordinary_agent_semantic_entry",
zero message-loop rows — the form whose missing prompt-render file caused the
stop-condition-2 false positive). The mixed/other fixtures add rows of the
remaining telemetry source families so the fail-closed branches are covered.

No stack, no LLM, no network: pure file-shape tests (card 252 verifies the
script face only; the real-stack all-arbitration rerun rides the next such
run per the card's verification clause)."""

import json
import tempfile
import unittest
from pathlib import Path

import automation_llm_route_smoke as smoke


# Verbatim pre-252 implementation of the guidance check (three lines of
# behavior the card locks byte-for-byte for every non-all-arbitration form).
# Kept as the control the new three-state result must reproduce exactly.
def legacy_check_prompt_guidance(prompt_render_path):
    if not prompt_render_path or not prompt_render_path.exists():
        return False
    try:
        blob = prompt_render_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return False
    return "automation.set_points" in blob and "automation.get_curve" in blob


# 251 archived telemetry rows (g3run1, verbatim; the strip-silence setup leg
# plus the first arbitration round of llm_route_g3_r1).
ARCHIVED_SEMANTIC_ENTRY_ROWS = [
    '{"schema_version":"vit_llm_telemetry.v1","created_at":"2026-11-05T02:36:57.3494368Z","source":"ordinary_agent_semantic_entry","conversation_id":"dev_strip_silence_chat_20261105_103653","model":"deepseek-flash","message_count":2,"request_build_ms":0,"http_ms":2866,"total_ms":2866,"usage":{"input_tokens":706,"output_tokens":366,"cache_read_tokens":512,"cache_creation_tokens":0},"error":""}',
    '{"schema_version":"vit_llm_telemetry.v1","created_at":"2026-11-05T02:37:02.5389876Z","source":"ordinary_agent_semantic_entry","conversation_id":"llm_route_g3_r1_1793846219","model":"deepseek-flash","message_count":2,"request_build_ms":0,"http_ms":2596,"total_ms":2596,"usage":{"input_tokens":697,"output_tokens":308,"cache_read_tokens":512,"cache_creation_tokens":0},"error":""}',
    '{"schema_version":"vit_llm_telemetry.v1","created_at":"2026-11-05T02:37:04.9193797Z","source":"ordinary_agent_semantic_entry","conversation_id":"llm_route_g3_r1_1793846219","model":"deepseek-flash","message_count":2,"request_build_ms":0,"http_ms":1736,"total_ms":1736,"usage":{"input_tokens":698,"output_tokens":183,"cache_read_tokens":512,"cache_creation_tokens":0},"error":""}',
]

# Same schema with the message-loop face source (message_loop.go:555) — used
# for the mixed-control fixtures; the 251 archive itself has zero such rows.
MESSAGE_LOOP_ROW = json.dumps({
    "schema_version": "vit_llm_telemetry.v1",
    "created_at": "2026-11-05T02:37:10.0000000Z",
    "source": "message_loop",
    "conversation_id": "llm_route_g3_r1_1793846219",
    "model": "deepseek-flash",
    "message_count": 12,
    "usage": {"input_tokens": 4096, "output_tokens": 200},
    "error": "",
})

UNKNOWN_SOURCE_ROW = json.dumps({
    "schema_version": "vit_llm_telemetry.v1",
    "created_at": "2026-11-05T02:37:11.0000000Z",
    "source": "some_future_face",
    "conversation_id": "llm_route_g3_r1_1793846219",
    "model": "deepseek-flash",
    "message_count": 2,
    "usage": {"input_tokens": 100, "output_tokens": 10},
    "error": "",
})

RENDER_WITH_NEEDLES = (
    "system prompt render\ntools: automation.set_points / automation.get_curve\n"
)
RENDER_WITHOUT_NEEDLES = "system prompt render\nsome other guidance text\n"


class CheckPromptGuidanceThreeStateTest(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.dir = Path(self._tmp.name)
        self.render_ok = self.dir / "agent_prompt_render.jsonl"
        self.render_ok.write_text(RENDER_WITH_NEEDLES, encoding="utf-8")
        self.render_no_needle = self.dir / "render_no_needle.jsonl"
        self.render_no_needle.write_text(RENDER_WITHOUT_NEEDLES, encoding="utf-8")
        self.render_absent = self.dir / "never_created.jsonl"
        self.telemetry_all_arb = self.dir / "telemetry_all_arb.jsonl"
        self.telemetry_all_arb.write_text(
            "\n".join(ARCHIVED_SEMANTIC_ENTRY_ROWS) + "\n", encoding="utf-8")
        self.telemetry_mixed = self.dir / "telemetry_mixed.jsonl"
        self.telemetry_mixed.write_text(
            "\n".join(ARCHIVED_SEMANTIC_ENTRY_ROWS[:1] + [MESSAGE_LOOP_ROW]) + "\n",
            encoding="utf-8")
        self.telemetry_unknown = self.dir / "telemetry_unknown.jsonl"
        self.telemetry_unknown.write_text(
            "\n".join(ARCHIVED_SEMANTIC_ENTRY_ROWS[:1] + [UNKNOWN_SOURCE_ROW]) + "\n",
            encoding="utf-8")
        self.telemetry_empty = self.dir / "telemetry_empty.jsonl"
        self.telemetry_empty.write_text("", encoding="utf-8")
        self.telemetry_absent = self.dir / "telemetry_absent.jsonl"

    def tearDown(self) -> None:
        self._tmp.cleanup()

    # ---- state 1: file present + needle present = ok ---------------------
    def test_state_ok_file_and_needle_present(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_ok, self.telemetry_mixed)
        self.assertEqual(guidance["state"], "ok")
        self.assertIs(guidance["rendered"], True)
        self.assertTrue(legacy_check_prompt_guidance(self.render_ok))

    # ---- state 2: file present + needle absent = real defect, still fails
    def test_state_needle_missing_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_no_needle, self.telemetry_mixed)
        self.assertEqual(guidance["state"], "needle_missing")
        self.assertIs(guidance["rendered"], False)
        self.assertFalse(legacy_check_prompt_guidance(self.render_no_needle))
        summary: dict = {}
        self.assertTrue(smoke.apply_prompt_guidance_summary(summary, guidance))
        self.assertEqual(summary, {"prompt_guidance_rendered": False})

    # ---- state 3: file absent + all-arbitration telemetry = n/a ----------
    def test_state_na_file_absent_all_arbitration(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent, self.telemetry_all_arb)
        self.assertEqual(guidance["state"], "n_a_all_arbitration")
        self.assertIsNone(guidance["rendered"])
        self.assertTrue(guidance["telemetry"]["all_arbitration"])
        self.assertEqual(guidance["telemetry"]["rows"], 3)
        self.assertEqual(guidance["telemetry"]["semantic_entry_rows"], 3)
        self.assertEqual(guidance["telemetry"]["message_loop_rows"], 0)
        summary: dict = {}
        self.assertFalse(smoke.apply_prompt_guidance_summary(summary, guidance))
        self.assertEqual(summary["prompt_guidance_rendered"], "n/a (all-arbitration)")
        self.assertEqual(summary["prompt_guidance_n_a"]["reason"],
                         "prompt_render_file_absent_all_arbitration")
        self.assertEqual(summary["prompt_guidance_n_a"]["telemetry"]["rows"], 3)

    # ---- fail-closed: file absent without all-arbitration proof ----------
    def test_file_absent_mixed_telemetry_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent, self.telemetry_mixed)
        self.assertEqual(guidance["state"], "missing")
        self.assertIs(guidance["rendered"], False)

    def test_file_absent_unknown_source_row_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent, self.telemetry_unknown)
        self.assertEqual(guidance["state"], "missing")
        self.assertIs(guidance["rendered"], False)

    def test_file_absent_empty_telemetry_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent, self.telemetry_empty)
        self.assertEqual(guidance["state"], "missing")
        self.assertIs(guidance["rendered"], False)

    def test_file_absent_telemetry_path_absent_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent, self.telemetry_absent)
        self.assertEqual(guidance["state"], "missing")
        self.assertIs(guidance["rendered"], False)

    def test_file_absent_no_telemetry_argument_still_fails(self) -> None:
        guidance = smoke.check_prompt_guidance(self.render_absent)
        self.assertEqual(guidance["state"], "missing")
        self.assertIs(guidance["rendered"], False)

    # ---- control: every non-n/a form is byte-equivalent to pre-252 -------
    def test_control_equivalence_to_legacy_on_all_non_na_forms(self) -> None:
        render_files = [self.render_ok, self.render_no_needle, self.render_absent,
                        None]
        telemetry_files = [self.telemetry_all_arb, self.telemetry_mixed,
                           self.telemetry_unknown, self.telemetry_empty,
                           self.telemetry_absent, None]
        compared = 0
        for render in render_files:
            for telemetry in telemetry_files:
                guidance = smoke.check_prompt_guidance(render, telemetry)
                legacy = legacy_check_prompt_guidance(render)
                if guidance["state"] == "n_a_all_arbitration":
                    # the one intended behavior change, reachable only from
                    # exactly one input combination: render file expected but
                    # absent AND telemetry proves 100% semantic-entry rounds
                    self.assertIs(render, self.render_absent)
                    self.assertIs(telemetry, self.telemetry_all_arb)
                    continue
                summary: dict = {}
                stop_condition_2 = smoke.apply_prompt_guidance_summary(summary, guidance)
                self.assertEqual(
                    summary, {"prompt_guidance_rendered": legacy},
                    msg=f"summary shape diverged from legacy for render={render} "
                        f"telemetry={telemetry}")
                self.assertEqual(
                    stop_condition_2, (not legacy),
                    msg=f"stop-condition-2 decision diverged for render={render} "
                        f"telemetry={telemetry}")
                compared += 1
        self.assertGreater(compared, 20)

    # ---- classifier unit face -------------------------------------------
    def test_classify_counts_source_families(self) -> None:
        counts = smoke.classify_telemetry_sources(self.telemetry_mixed)
        self.assertTrue(counts["available"])
        self.assertEqual(counts["rows"], 2)
        self.assertEqual(counts["semantic_entry_rows"], 1)
        self.assertEqual(counts["message_loop_rows"], 1)
        self.assertEqual(counts["other_source_rows"], 0)
        self.assertFalse(counts["all_arbitration"])

    def test_classify_missing_and_empty_paths(self) -> None:
        self.assertFalse(smoke.classify_telemetry_sources(None)["available"])
        empty = smoke.classify_telemetry_sources(self.telemetry_empty)
        self.assertTrue(empty["available"])
        self.assertEqual(empty["rows"], 0)
        self.assertFalse(empty["all_arbitration"])
        absent = smoke.classify_telemetry_sources(self.telemetry_absent)
        self.assertFalse(absent["available"])


if __name__ == "__main__":
    unittest.main()
