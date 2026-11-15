#!/usr/bin/env python3
"""SRC-TEMPLATE-1 (card 373) course template builder, 2026-11-15.

Deterministic, zero-stack generator for the fixed-healthy-source course
template (DG1-B subject). Reads the Surge XT plugin element (identifier +
proven non-silent init-patch state blob) from the 309 evidence fixture and
emits the portable template XML for the music-creation-unit2 lesson:

  coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/
      template.v1.xml   (clear-XML Tracktion EDIT; open/save/reopen all take
                         the kernel's normal clear-XML branches)

Provenance contract (audit H5 / STACKGATE DG1-B):
  - Surge XT state blob = 309 E4 fixture verbatim
    (coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/e4_surge_xt/
     waveshell_stackleg_fixture_render_direct_surge_xt.vit, saved by the
    kernel 2026-11-09; same-state render measured -12.6 dBFS offline /
    -10.97 dB realtime — WAVESHELL-OFFRENDER-FIX-1 receipt item 1/6).
  - Waves WaveShell instruments stay excluded (309 M1 RT-SILENT).
  - VST3 program change is not on the exposed parameter face (VSTI-PARAM-
    PROBE-1 item 3), so "program pinning" = pinning the plugin state blob in
    the template file (recorded as sha256 in template_manifest.json).

Template shape mirrors design.v1.yaml workspace (revision 3):
  timing 2/4, key C, tempo 80 (design tempo_bpm null -> template default 80,
  W01 compiler override seam documented in the manifest), tracks
  reference/melody(我的旋律)/rhythm(辅助律动（鼓）)/theme(我的八小节主题)/
  result(我的成曲版本).  Instruments: Surge XT on melody/theme/rhythm tracks.
  Melody clip = placeholder pitches (C4=60) per "节奏稿=占位音高" ruling;
  rhythm clip = GM-percussion-notated 2/4 groove (at least one usable set).

Determinism: no timestamps, no random ids (note ids = md5 of stable keys),
fixed item ids.  Same inputs -> byte-identical output (sha256 printed).

Usage: python scripts/src_template_build.py [--repo-root D:/Vit_DAW]
"""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from pathlib import Path

SURGE_FIXTURE_REL = (
    "coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/"
    "e4_surge_xt/waveshell_stackleg_fixture_render_direct_surge_xt.vit"
)
TEMPLATE_REL = (
    "coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml"
)

TEMPLATE_UUID = "vitproj_srctemplate1melodydrumv1a0000000001"
KEY_SIGNATURE = "C"
TEMPO_BPM = "80.0"
NUMERATOR, DENOMINATOR = 2, 4

# Track layout: (design track id, name, has_instrument, clip alias or None)
TRACKS = [
    ("reference-track", "参考与示范", False, None),
    ("melody-track", "我的旋律", True, "rhythm-lab"),
    ("rhythm-track", "辅助律动（鼓）", True, "groove-2-4-basic"),
    ("theme-track", "我的八小节主题", True, "eight-bar-theme"),
    ("result-track", "我的成曲版本", False, None),
]

PLACEHOLDER_PITCH = 60  # C4 — 占位音高 per REVISIONS-2026-11-12 ruling
GROOVE_KICK, GROOVE_SNARE, GROOVE_HAT = 36, 38, 42  # GM percussion notation


def stable_note_id(clip_alias: str, index: int) -> str:
    digest = hashlib.md5(
        ("src-template-v1/" + clip_alias + "/" + str(index)).encode("utf-8")
    ).hexdigest()
    return "note_" + digest


def melody_placeholder_notes(clip_alias: str, beats: int) -> list[dict]:
    """8 quarter notes at placeholder pitch (rhythm-lab: 2/4 x 4 bars)."""
    return [
        {"p": PLACEHOLDER_PITCH, "b": float(i), "l": 1.0, "v": 90}
        for i in range(beats)
    ]


def theme_placeholder_notes(clip_alias: str, beats: int) -> list[dict]:
    """16 quarter notes at placeholder pitch (eight-bar-theme: 2/4 x 8 bars)."""
    return [
        {"p": PLACEHOLDER_PITCH, "b": float(i), "l": 1.0, "v": 90}
        for i in range(beats)
    ]


def groove_notes(clip_alias: str) -> list[dict]:
    """One 2/4 x 4-bar auxiliary groove, GM percussion notation.

    Surge XT init patch is chromatic (no GM drum kit): 36/38/42 render as
    pitched synth tones in the low register.  The GM keys pin the *notation*
    semantics so a future GM percussion source plays the same groove intact
    (declared in template_manifest.json; timbre judgement = M2 human node).
    """
    notes: list[dict] = []
    for bar in range(4):
        base = bar * 2.0
        notes.append({"p": GROOVE_KICK, "b": base + 0.0, "l": 0.5, "v": 105})
        notes.append({"p": GROOVE_HAT, "b": base + 0.5, "l": 0.25, "v": 60})
        notes.append({"p": GROOVE_KICK, "b": base + 1.0, "l": 0.5, "v": 95})
        if bar < 3:
            notes.append({"p": GROOVE_SNARE, "b": base + 1.5, "l": 0.5, "v": 90})
        else:
            # bar 4 small fill
            notes.append({"p": GROOVE_SNARE, "b": base + 1.25, "l": 0.25, "v": 80})
            notes.append({"p": GROOVE_SNARE, "b": base + 1.5, "l": 0.25, "v": 85})
            notes.append({"p": GROOVE_HAT, "b": base + 1.75, "l": 0.1, "v": 70})
    return notes


CLIP_SPECS = {
    "rhythm-lab": (8.0, melody_placeholder_notes, 1, 0),     # channelNumber, note c
    "groove-2-4-basic": (8.0, groove_notes, 10, 9),
    "eight-bar-theme": (16.0, theme_placeholder_notes, 1, 0),
}

# Item ids (unique across the edit; kernel service ids 1001-1006 preserved).
TRACK_BASE = {
    "reference-track": 1100,
    "melody-track": 1110,
    "rhythm-track": 1120,
    "theme-track": 1130,
    "result-track": 1140,
}


def extract_surge_plugin(fixture_path: Path) -> dict:
    text = fixture_path.read_text(encoding="utf-8")
    match = re.search(
        r'<PLUGIN type="vst" ([^>]*name="Surge XT"[^>]*?)>\s*<MODIFIERASSIGNMENTS/>',
        text,
        re.S,
    )
    if not match:
        raise SystemExit("surge plugin element not found in fixture: " + str(fixture_path))
    attrs = match.group(1)
    def attr(name: str) -> str:
        m = re.search(name + r'="([^"]*)"', attrs)
        if not m:
            raise SystemExit("surge plugin attribute missing: " + name)
        return m.group(1)
    state = attr("state")
    layout = attr("base64:layout")
    return {
        "uniqueId": attr("uniqueId"),
        "uid": attr("uid"),
        "filename": attr("filename"),
        "name": attr("name"),
        "manufacturer": attr("manufacturer"),
        "state": state,
        "layout": layout,
        "state_sha256": hashlib.sha256(state.encode("ascii")).hexdigest(),
    }


def note_xml(note: dict, clip_alias: str, index: int, channel: int) -> str:
    return (
        '        <NOTE p="{p}" b="{b}" l="{l}" v="{v}" c="{c}" '
        'vit_note_id="{nid}"/>'.format(
            p=note["p"],
            b=note["b"],
            l=note["l"],
            v=note["v"],
            c=channel,
            nid=stable_note_id(clip_alias, index),
        )
    )


def clip_xml(alias: str, clip_id: int) -> str:
    length, note_fn, channel_number, note_channel = CLIP_SPECS[alias]
    notes = note_fn(alias, int(length)) if alias != "groove-2-4-basic" else note_fn(alias)
    body = "\n".join(
        note_xml(note, alias, i, note_channel) for i, note in enumerate(notes)
    )
    return (
        '    <MIDICLIP name="{alias}" start="0.0" length="{len}" offset="0.0" id="{cid}"\n'
        '              sync="0" currentTake="0" speed="1.0" colour="ff55ff00">\n'
        "      <AUTOMATIONCURVES/>\n"
        "      <QUANTISATION/>\n"
        "      <GROOVE/>\n"
        '      <SEQUENCE ver="1" channelNumber="{ch}">\n'
        "{body}\n"
        "      </SEQUENCE>\n"
        "    </MIDICLIP>\n"
    ).format(alias=alias, len=length, cid=clip_id, ch=channel_number, body=body)


def surge_plugin_xml(surge: dict, plugin_id: int) -> str:
    return (
        '    <PLUGIN type="vst" uniqueId="{uid_hex}" uid="{inst_uid}" filename="{fname}"\n'
        '            name="{name}" manufacturer="{manu}" id="{pid}" enabled="1"\n'
        '            remapOnTempoChange="1" vit_template_role="instrument" state="{state}"\n'
        '            base64:layout="{layout}">\n'
        "      <MODIFIERASSIGNMENTS/>\n"
        "    </PLUGIN>\n"
    ).format(
        uid_hex=surge["uniqueId"],
        inst_uid=surge["uid"],
        fname=surge["filename"],
        name=surge["name"],
        manu=surge["manufacturer"],
        pid=plugin_id,
        state=surge["state"],
        layout=surge["layout"],
    )


def track_xml(design_id: str, name: str, has_instrument: bool, alias, surge: dict) -> str:
    base = TRACK_BASE[design_id]
    parts = ['  <TRACK name="{n}" id="{tid}">'.format(n=name, tid=base)]
    parts.append("    <MODIFIERS/>")
    parts.append("    <CLIPSLOTS/>")
    if alias is not None:
        parts.append(clip_xml(alias, base + 1).rstrip("\n"))
    if has_instrument:
        parts.append(surge_plugin_xml(surge, base + 2).rstrip("\n"))
    parts.append(
        '    <PLUGIN type="volume" id="{vid}" enabled="1" remapOnTempoChange="1">\n'
        "      <MODIFIERASSIGNMENTS/>\n"
        "    </PLUGIN>".format(vid=base + 3)
    )
    parts.append('    <PLUGIN type="level" id="{lid}" enabled="1"/>'.format(lid=base + 4))
    parts.append("    <OUTPUTDEVICES>")
    parts.append('      <DEVICE name="(default audio output)"/>')
    parts.append("    </OUTPUTDEVICES>")
    parts.append("  </TRACK>")
    return "\n".join(parts) + "\n"


def build_template(surge: dict) -> str:
    tracks = "".join(
        track_xml(design_id, name, has_inst, alias, surge)
        for design_id, name, has_inst, alias in TRACKS
    )
    return (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        "\n"
        '<EDIT appVersion="Unknown" projectID="0/1c5f8d0" creationTime="0" modifiedBy="timoz"\n'
        '      vit_project_uuid="{uuid}"\n'
        '      vit_music_key_signature="{key}"\n'
        '      lastSignificantChange="1a1c00b0264">\n'
        '  <TRANSPORT endToEnd="1" position="0.0"/>\n'
        "  <TEMPOSEQUENCE>\n"
        '    <TEMPO startBeat="0.0" bpm="{bpm}" curve="1.0"/>\n'
        '    <TIMESIG numerator="{num}" denominator="{den}" startBeat="0.0"/>\n'
        "  </TEMPOSEQUENCE>\n"
        "  <VIDEO/>\n"
        '  <CLICKTRACK level="0.6000000238418579"/>\n'
        "  <MASTERVOLUME>\n"
        '    <PLUGIN type="volume" id="1001" enabled="1" volume="0.6376281516">\n'
        "      <MODIFIERASSIGNMENTS/>\n"
        "    </PLUGIN>\n"
        "  </MASTERVOLUME>\n"
        "  <RACKS/>\n"
        "  <MASTERPLUGINS/>\n"
        "  <INPUTDEVICES>\n"
        '    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins"/>\n'
        "  </INPUTDEVICES>\n"
        "  <TRACKCOMPS/>\n"
        "  <SCENES/>\n"
        "  <PITCHSEQUENCE>\n"
        '    <PITCH startBeat="0.0" pitch="60"/>\n'
        "  </PITCHSEQUENCE>\n"
        '  <VIT_AUDIO_SETTINGS migration_state="defaulted_new_project" defaulted_origin="new_project"\n'
        '                      schema_version="vit_project_audio_settings.v1" sample_rate_hz="48000"\n'
        '                      record_file_type="WAV/BWF" record_bit_depth="24" pcm_format="int24"\n'
        '                      internal_processing_format="unknown" import_sample_rate_policy="ask"\n'
        '                      import_bit_depth_policy="keep_source" media_copy_policy="reference_original"\n'
        '                      channel_import_policy="preserve_interleaved" render_default_sample_rate_hz="48000"\n'
        '                      render_default_bit_depth="24" render_default_file_type="WAV"\n'
        '                      dither_policy="unknown"/>\n'
        '  <ARRANGERTRACK name="Arranger" id="1002"/>\n'
        '  <CHORDTRACK name="Chord" id="1003"/>\n'
        '  <MARKERTRACK id="1004" name="Marker"/>\n'
        '  <TEMPOTRACK name="Global" id="1005">\n'
        "    <MODIFIERS/>\n"
        "  </TEMPOTRACK>\n"
        '  <MASTERTRACK name="Master" id="1006">\n'
        "    <MODIFIERS/>\n"
        "  </MASTERTRACK>\n"
        "{tracks}"
        "</EDIT>\n"
    ).format(
        uuid=TEMPLATE_UUID,
        key=KEY_SIGNATURE,
        bpm=TEMPO_BPM,
        num=NUMERATOR,
        den=DENOMINATOR,
        tracks=tracks,
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="SRC-TEMPLATE-1 course template builder")
    parser.add_argument("--repo-root", default=None)
    parser.add_argument("--stdout", action="store_true")
    args = parser.parse_args()

    repo = Path(args.repo_root).resolve() if args.repo_root else Path(__file__).resolve().parents[1]
    fixture = repo / SURGE_FIXTURE_REL
    surge = extract_surge_plugin(fixture)
    template = build_template(surge)

    if args.stdout:
        sys.stdout.write(template)
        return 0

    target = repo / TEMPLATE_REL
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(template, encoding="utf-8", newline="\n")
    print("template written:", target)
    print("bytes:", target.stat().st_size)
    # Canonical-form hash (LF): stable across git autocrlf checkout modes.
    canonical = target.read_bytes().replace(b"\r\n", b"\n")
    print("sha256(canonical LF):", hashlib.sha256(canonical).hexdigest())
    print("surge state sha256:", surge["state_sha256"])
    print("surge identifier: VST3-Surge XT-{0}-{1}".format(surge["uniqueId"], surge["uid"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
