#!/usr/bin/env python3
"""Generate a minimal Vit-DAW edit XML from natural-language style arguments."""

from __future__ import annotations

import argparse
import hashlib
import sys
import time
import xml.etree.ElementTree as ET
from typing import Iterable, List


class IdAllocator:
    """Allocates strictly increasing numeric IDs."""

    def __init__(self, start: int = 1001) -> None:
        self._next = start

    def take(self) -> int:
        value = self._next
        self._next += 1
        return value


def mood_to_bpm(mood: str) -> float:
    normalized = mood.strip().lower()
    if normalized in {"dark", "moody", "tense"}:
        return 92.0
    if normalized in {"bright", "happy", "uplifting"}:
        return 128.0
    if normalized in {"calm", "ambient", "soft"}:
        return 78.0
    if normalized in {"aggressive", "energetic", "intense"}:
        return 140.0
    return 120.0


def build_project_id(seed_text: str) -> str:
    digest = hashlib.sha1(seed_text.encode("utf-8")).hexdigest()[:7]
    return f"0/{digest}"


def append_plugin(parent: ET.Element, allocator: IdAllocator, plugin_type: str, **attrs: object) -> int:
    plugin_id = allocator.take()
    plugin = ET.SubElement(
        parent,
        "PLUGIN",
        {
            "type": plugin_type,
            "id": str(plugin_id),
            "enabled": "1",
            **{key: str(value) for key, value in attrs.items()},
        },
    )
    if plugin_type == "volume":
        ET.SubElement(plugin, "MODIFIERASSIGNMENTS")
    return plugin_id


def append_track(
    root: ET.Element,
    allocator: IdAllocator,
    name: str,
    *,
    vit_type: str | None = None,
    vit_intent: str | None = None,
    colour: str | None = None,
) -> int:
    track_id = allocator.take()
    attributes = {"id": str(track_id), "name": name}
    if vit_type:
        attributes["vit_type"] = vit_type
    if vit_intent:
        attributes["vit_intent"] = vit_intent
    if colour:
        attributes["colour"] = colour

    track = ET.SubElement(root, "TRACK", attributes)
    ET.SubElement(track, "MODIFIERS")

    output_devices = ET.SubElement(track, "OUTPUTDEVICES")
    ET.SubElement(output_devices, "DEVICE", {"name": "(default audio output)"})

    ET.SubElement(track, "CLIPSLOTS")
    append_plugin(track, allocator, "volume", remapOnTempoChange="1")
    append_plugin(track, allocator, "level")
    return track_id


def collect_numeric_ids(root: ET.Element) -> List[int]:
    values: List[int] = []
    for element in root.iter():
        element_id = element.attrib.get("id")
        if element_id and element_id.isdigit():
            values.append(int(element_id))
    return values


def validate_generated_xml(root: ET.Element) -> None:
    numeric_ids = collect_numeric_ids(root)

    if numeric_ids != sorted(numeric_ids):
        raise ValueError("Generated IDs are not strictly increasing.")

    if len(set(numeric_ids)) != len(numeric_ids):
        raise ValueError("Generated IDs are not unique.")

    ghost_tracks = [
        child
        for child in root.findall("TRACK")
        if child.attrib.get("vit_type") == "ghost"
    ]

    has_ghost_track = any(
        child.tag == "TRACK" and child.attrib.get("vit_type") == "ghost"
        for child in root.findall("TRACK")
    )
    if not has_ghost_track:
        raise ValueError("Generated XML does not contain a ghost track.")

    if len(ghost_tracks) < 2:
        raise ValueError("Generated XML must contain at least two ghost tracks.")


def indent_xml(element: ET.Element) -> None:
    ET.indent(element, space="  ")


def build_edit_xml(mood: str, prompt: str, audio_tracks: int) -> str:
    allocator = IdAllocator()
    timestamp = str(int(time.time() * 1000))
    seed_text = f"{mood}|{prompt}|{timestamp}"

    root = ET.Element(
        "EDIT",
        {
            "appVersion": "Unknown",
            "projectID": build_project_id(seed_text),
            "creationTime": timestamp,
            "modifiedBy": "gen_vit_project.py",
        },
    )

    ET.SubElement(root, "TRANSPORT", {"endToEnd": "1"})

    pitch_sequence = ET.SubElement(root, "PITCHSEQUENCE")
    ET.SubElement(pitch_sequence, "PITCH", {"startBeat": "0.0", "pitch": "60"})

    tempo_sequence = ET.SubElement(root, "TEMPOSEQUENCE")
    ET.SubElement(
        tempo_sequence,
        "TEMPO",
        {
            "startBeat": "0.0",
            "bpm": f"{mood_to_bpm(mood):.1f}",
            "curve": "1.0",
        },
    )
    ET.SubElement(
        tempo_sequence,
        "TIMESIG",
        {
            "numerator": "4",
            "denominator": "4",
            "startBeat": "0.0",
        },
    )

    ET.SubElement(root, "VIDEO")
    ET.SubElement(root, "CLICKTRACK", {"level": "0.6000000238418579"})

    master_volume = ET.SubElement(root, "MASTERVOLUME")
    append_plugin(master_volume, allocator, "volume", volume="0.6376281380653381")

    ET.SubElement(root, "RACKS")
    ET.SubElement(root, "MASTERPLUGINS")

    input_devices = ET.SubElement(root, "INPUTDEVICES")
    ET.SubElement(input_devices, "INPUTDEVICE", {"deviceID": "all_midi_in", "name": "All MIDI Ins"})

    ET.SubElement(root, "TRACKCOMPS")

    arranger_id = allocator.take()
    ET.SubElement(root, "ARRANGERTRACK", {"name": "Arranger", "id": str(arranger_id)})

    chord_id = allocator.take()
    ET.SubElement(root, "CHORDTRACK", {"name": "Chord", "id": str(chord_id)})

    marker_id = allocator.take()
    ET.SubElement(root, "MARKERTRACK", {"id": str(marker_id), "name": "Marker"})

    tempo_track_id = allocator.take()
    tempo_track = ET.SubElement(root, "TEMPOTRACK", {"name": "Global", "id": str(tempo_track_id)})
    ET.SubElement(tempo_track, "MODIFIERS")

    master_track_id = allocator.take()
    master_track = ET.SubElement(root, "MASTERTRACK", {"name": "Master", "id": str(master_track_id)})
    ET.SubElement(master_track, "MODIFIERS")

    total_tracks = max(audio_tracks, 3)
    track_specs = [
        {"name": "Audio Track 1", "vit_type": None, "vit_intent": None, "colour": None},
        {
            "name": "Ghost Layer A",
            "vit_type": "ghost",
            "vit_intent": "texture",
            "colour": "0xff00ffff",
        },
        {
            "name": "Ghost Layer B",
            "vit_type": "ghost",
            "vit_intent": "accent",
            "colour": "0xff00ffff",
        },
    ]
    for index in range(4, total_tracks + 1):
        track_specs.append(
            {
                "name": f"Audio Track {index}",
                "vit_type": None,
                "vit_intent": None,
                "colour": None,
            }
        )

    for index, spec in enumerate(track_specs, start=1):
        track_id = append_track(
            root,
            allocator,
            spec["name"],
            vit_type=spec["vit_type"],
            vit_intent=spec["vit_intent"],
            colour=spec["colour"],
        )
        ET.SubElement(
            input_devices,
            "INPUTDEVICE",
            {
                "deviceID": f"wavein_auto_{index:04d}",
                "name": f"Input {index}",
            },
        )
        last_input = input_devices[-1]
        ET.SubElement(
            last_input,
            "INPUTDEVICEDESTINATION",
            {
                "targetID": str(track_id),
                "targetIndex": "0",
                "armed": "1" if spec["vit_type"] != "ghost" and index == 1 else "0",
            },
        )

    ET.SubElement(root, "SCENES")

    if prompt.strip():
        metadata = ET.SubElement(root, "VITMETADATA")
        metadata.set("mood", mood.strip())
        metadata.set("prompt", prompt.strip())

    validate_generated_xml(root)
    indent_xml(root)
    return ET.tostring(root, encoding="unicode", xml_declaration=True)


def parse_args(argv: Iterable[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate a Vit-DAW project XML string.")
    parser.add_argument("--mood", default="neutral", help="Natural-language mood hint, e.g. dark")
    parser.add_argument("--prompt", default="", help="Free-form natural-language scene or idea")
    parser.add_argument(
        "--audio-tracks",
        type=int,
        default=3,
        help="Total number of tracks to generate; defaults to 1 audio track plus 2 ghost tracks",
    )
    parser.add_argument(
        "--output",
        default="D:/Vit_DAW/scripts/generated_project.xml",
        help="Output file path for the generated XML",
    )
    return parser.parse_args(list(argv))


def main(argv: Iterable[str]) -> int:
    args = parse_args(argv)
    xml_text = build_edit_xml(args.mood, args.prompt, max(args.audio_tracks, 1))

    with open(args.output, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(xml_text)
        handle.write("\n")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
