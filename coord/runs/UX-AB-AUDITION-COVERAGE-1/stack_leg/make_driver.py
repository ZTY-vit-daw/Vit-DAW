from pathlib import Path

here = Path(__file__).parent
src = here / "uxab177_driver_base.py"
text = src.read_text(encoding="utf-8")
fragment = (here / "topo_stage_fragment.py").read_text(encoding="utf-8").rstrip("\n")

start = text.index("    # ---- second stage: audition session extraction + judgment probes -------")
end = text.index('    signals["judgment_stage"] = judgment_stage')
text = text[:start] + fragment + "\n\n" + text[end:]

text = text.replace(
    'for key in ("dynamic_session_found", "probe_wrong_turn", "probe_fake_experiment_session",',
    'for key in ("topo_session_found", "ticket_session_found", "connect_leg_dispatched",',
)
text = text.replace(
    '                "probe_correct", "goal_after_probes", "goal_after_fallback",\n                "experiment_judgment_fallback_status"):',
    '                "load_leg_dispatched", "probe_wrong_turn", "probe_topo_correct"):',
)

doc_open = text.index('"""')
doc_close = text.index('"""', doc_open + 3) + 3
old_doc = text[doc_open:doc_close]
lines = [
    '"""UX-AB-AUDITION-COVERAGE-1 (card 177) real-stack reproduction leg, 2026-10-23.',
    '',
    'Adapted from aba174_driver.py (card 174; 146/170 driver family) with the',
    'assertion faces swapped to this card\'s target: the manual-session 场次⑤ form',
    '(bare utterance "给人声加点空间感", full access, cards auto-answered) must,',
    'when the chain dispatches the direct bus-topology legs (track.send.connect',
    'auto_create + rack_add_node -- the forensically-confirmed non-ticket effect',
    'mutation path), mount an equivalent audition session in the',
    'audition:dynamic:topo_* family (the 174 bracket only covered the semantic',
    'ticket parameter write). Capture only; assertions live in assert_uxab177.py.',
    '',
    'Run budget (pre-declared): <=3 rounds; retries only for env interrupts or a',
    'round that never exercises the direct-legs form (no track.send.connect in the',
    'executed surface). Success = assert script exit 0. LLM turns follow the',
    "stack's configured model (deepseek-flash family per 174/175 legs).",
    '',
    'Exit codes: 0 = round captured (classification happens in the assert script);',
    '2 = environment-level abort (fixture / authority / transport), exit-2 shape.',
    '"""',
]
text = text.replace(old_doc, "\n".join(lines), 1)

out = here / "uxab177_driver.py"
out.write_text(text, encoding="utf-8", newline="\n")
print("written", out.name, len(text.splitlines()), "lines")
