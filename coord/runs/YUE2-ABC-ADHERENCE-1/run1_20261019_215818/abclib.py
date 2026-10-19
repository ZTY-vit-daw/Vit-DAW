"""YuE2-dialect ABC parser + adherence metrics for YUE2-ABC-ADHERENCE-1.

Dialect handled (as emitted in yue-server replay 'abc' field):
  headers X:/T:/M:/L:/Q:/K:/V:, section comments '%name', voice lines 'V: <name>',
  chord labels '"Dm7"' applying to following notes in the bar, notes like
  ^c'4 / d,8 / =e2, rests z/Z with duration, ties '-', bars '|'.
Plain single-voice ABC (no V:, no chords) parses too (Leg C).
"""
import re

NOTE_RE = re.compile(r"(__|_|=|\^|\^\^)?([a-gA-G])([,']*)(\d*)(~?-?)")
CHORD_RE = re.compile(r'"([^"]+)"')
REST_RE = re.compile(r'[zZ](\d*)')
STEP = {'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
ACC = {'': 0, '=': 0, '^': 1, '^^': 2, '_': -1, '__': -2}
QUALITY_RE = re.compile(r'([A-G])(.*)')
PC_SHARP = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']


def midi_of(acc, letter, octmarks):
    base = STEP[letter.upper()]
    val = base + ACC[acc]
    if letter.islower():
        val += 12  # abc: lowercase = octave up
    val += 12 * (octmarks.count("'") - octmarks.count(','))
    return 60 + val  # C in abc middle octave == MIDI 60


class Bar:
    __slots__ = ('chord', 'notes', 'section', '_open_tie')

    def __init__(self):
        self.chord = None      # last chord label seen before/in this bar
        self.notes = []        # list of midi (ties collapsed: tie flag on prior)
        self.section = ''


class ABC:
    def __init__(self):
        self.headers = {}      # K, Q, M, L ...
        self.voices = {}       # name -> [Bar]
        self.cur_voice = 'default'
        self.sections = []     # ordered (name, first_bar_index_in_cur_voice)


def _bar_list(doc, voice):
    return doc.voices.setdefault(voice, [])


def parse(text):
    doc = ABC()
    pending_section = ''
    pending_chord = None
    for raw in text.splitlines():
        line = raw.strip()
        if not line:
            continue
        if line.startswith('%'):
            pending_section = line.lstrip('%').strip()
            continue
        m = re.match(r'^([A-Za-z]):\s*(.*)$', line)
        if m and not line.startswith('V:'):
            key, val = m.group(1), m.group(2).strip()
            if key == 'V':
                continue
            doc.headers[key] = val
            if key == 'Q':
                mm = re.search(r'(\d+)\s*$', val)
                if mm:
                    doc.headers['BPM'] = int(mm.group(1))
            continue
        if line.startswith('V:'):
            doc.cur_voice = line[2:].strip() or 'default'
            continue
        # music line
        bars = _bar_list(doc, doc.cur_voice)
        if not bars or bars[-1].notes or bars[-1].chord:
            bars.append(Bar())
        cur = bars[-1]
        if pending_section:
            cur.section = pending_section
            pending_section = ''
        i = 0
        while i < len(line):
            ch = line[i]
            if ch == '|':
                cur = Bar()
                if pending_section:
                    cur.section = pending_section
                    pending_section = ''
                bars.append(cur)
                i += 1
            elif ch == '"':
                m = CHORD_RE.match(line, i)
                if m:
                    cur.chord = m.group(1)
                    pending_chord = m.group(1)
                    i = m.end()
                else:
                    i += 1
            elif ch in 'zZ':
                m = REST_RE.match(line, i)
                i = m.end()  # rests: skip (not counted as notes)
            else:
                m = NOTE_RE.match(line, i)
                if m and m.group(2):
                    midi = midi_of(m.group(1) or '', m.group(2), m.group(3))
                    tie = m.group(5) == '-'
                    if cur.notes and line[i - 1:i] == '-' or (cur.notes and tie and getattr(cur, '_open_tie', False)):
                        pass  # tie continuation: do not append duplicate
                    else:
                        cur.notes.append(midi)
                    cur._open_tie = tie
                    i = m.end()
                else:
                    i += 1  # space, decoration, anything unknown
    return doc


def chord_root_pc(label):
    m = QUALITY_RE.match(label.strip())
    if not m:
        return None
    return STEP[m.group(1)] % 12


def chord_quality(label):
    m = QUALITY_RE.match(label.strip())
    return m.group(2).strip() if m else ''


def key_pc(header):
    if not header:
        return None
    m = re.match(r'([A-G])([#b]?)', header.strip())
    if not m:
        return None
    pc = STEP[m.group(1)]
    if m.group(2) == '#':
        pc += 1
    elif m.group(2) == 'b':
        pc -= 1
    return pc % 12


def melody_series(doc, voice='Vocal'):
    """Flat list of (bar_index, midi) over the given voice (falls back to
    whichever voice has the most notes)."""
    if voice not in doc.voices or not any(b.notes for b in doc.voices[voice]):
        voice = max(doc.voices, key=lambda v: sum(len(b.notes) for b in doc.voices[v]))
    out = []
    for bi, bar in enumerate(doc.voices[voice]):
        for n in bar.notes:
            out.append((bi, n))
    return out


def bar_mean_pitches(doc):
    """voice -> [mean midi per bar with notes]"""
    res = {}
    for v, bars in doc.voices.items():
        res[v] = [sum(b.notes) / len(b.notes) for b in bars if b.notes]
    return res


def chord_seq(doc, voice='Vocal'):
    out = []
    for bi, bar in enumerate(doc.voices.get(voice, [])):
        if bar.chord:
            out.append((bi, bar.chord))
    return out


# ---------------- metrics ----------------

def pearson(xs, ys):
    n = min(len(xs), len(ys))
    if n < 3:
        return None
    xs, ys = xs[:n], ys[:n]
    mx, my = sum(xs) / n, sum(ys) / n
    num = sum((x - mx) * (y - my) for x, y in zip(xs, ys))
    dx = sum((x - mx) ** 2 for x in xs) ** .5
    dy = sum((y - my) ** 2 for y in ys) ** .5
    if dx == 0 or dy == 0:
        return None
    return num / (dx * dy)


def m1_key(inp, out):
    a, b = key_pc(inp.headers.get('K')), key_pc(out.headers.get('K'))
    return {'in': a, 'out': b, 'match': a is not None and a == b}


def m2_tempo(inp, out):
    a, b = inp.headers.get('BPM'), out.headers.get('BPM')
    if not a or not b:
        return {'in': a, 'out': b, 'match': None}
    return {'in': a, 'out': b, 'rel': (b - a) / a,
            'match': abs(b - a) / a <= 0.08}


def _chord_match(inp, out, shift=0):
    a = [(bi, (chord_root_pc(c) + shift) % 12, chord_quality(c)) for bi, c in chord_seq(inp)]
    b = [(bi, chord_root_pc(c), chord_quality(c)) for bi, c in chord_seq(out)]
    if not a or not b:
        return None
    # align by ordinal position (bar indices may drift after realization)
    n = min(len(a), len(b))
    if n < 4:
        return None
    root_ok = sum(1 for i in range(n) if a[i][1] == b[i][1]) / n
    both = sum(1 for i in range(n) if a[i][1] == b[i][1] and a[i][2] == b[i][2]) / n
    return {'root': root_ok, 'root_quality': both}


def m3_chords(inp, out):
    direct = _chord_match(inp, out, 0)
    best_shift, best = 0, direct['root'] if direct else -1
    for s in range(1, 12):
        r = _chord_match(inp, out, s)
        if r and r['root'] > best:
            best_shift, best = s, r['root']
    return {'direct': direct, 'best_transposition': best,
            'best_shift': best_shift}


def m4_melody(inp, out):
    mi = bar_mean_pitches(inp)
    mo = bar_mean_pitches(out)
    vi = max(mi, key=lambda v: len(mi[v]))
    vo = max(mo, key=lambda v: len(mo[v]))
    corr = pearson(mi[vi], mo[vo])
    # interval sign agreement on the flat melody series
    si = [n for _, n in melody_series(inp)]
    so = [n for _, n in melody_series(out)]
    n = min(len(si), len(so))
    if n >= 4:
        pairs = [(si[i + 1] - si[i], so[i + 1] - so[i]) for i in range(n - 1)]
        usable = [(x, y) for x, y in pairs if x != 0 or y != 0]
        agree = sum(1 for x, y in usable if (x > 0) == (y > 0)) / len(usable) if usable else None
    else:
        agree = None
    return {'contour_corr': corr, 'interval_agree': agree,
            'n_notes_in': len(si), 'n_notes_out': len(so)}


def m5_structure(inp, out):
    sec_in = [b.section for v in inp.voices.values() for b in v if b.section]
    sec_out = [b.section for v in out.voices.values() for b in v if b.section]
    from collections import Counter
    ci, co = Counter(sec_in), Counter(sec_out)
    return {'sections_in': dict(ci), 'sections_out': dict(co),
            'section_names_match': set(ci) == set(co) if sec_out else None}


def compare(inp_text, out_text):
    inp, out = parse(inp_text), parse(out_text)
    return {'m1': m1_key(inp, out), 'm2': m2_tempo(inp, out),
            'm3': m3_chords(inp, out), 'm4': m4_melody(inp, out),
            'm5': m5_structure(inp, out)}
