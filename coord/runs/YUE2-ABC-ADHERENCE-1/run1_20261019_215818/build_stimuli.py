"""Build all stimuli for YUE2-ABC-ADHERENCE-1.

Leg A: 5 replay-native ABCs x seeds [11,22,33]
Leg B: smoke1 base + 4 perturbations x seeds [101,202] (base seeds [1,2])
Leg C: 2 handwritten plain ABCs x seeds [301,302]
Writes: stimuli/<id>.json (full /synth request) + stimuli/<id>.abc
"""
import json, os, re, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from abclib import NOTE_RE, CHORD_RE, STEP, PC_SHARP

DEP = 'D:/Vit_DAW/coord/runs/YUE2-PC-DEPLOY-1/run1_20261019_201652'
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'stimuli')
os.makedirs(OUT, exist_ok=True)


def midi_to_token(midi):
    pc = midi % 12
    name = PC_SHARP[pc]
    letter, acc = name[0], '^' if len(name) > 1 else ''
    oct_idx = (midi - 60) // 12  # 0 = uppercase octave starting at C(60)
    if oct_idx >= 1:
        return f"{acc}{letter.lower()}" + "'" * (oct_idx - 1)
    if oct_idx == 0:
        return f'{acc}{letter}'
    return f'{acc}{letter}' + ',' * (-oct_idx)


def parse_midi_token(tok):
    m = re.match(r"^(_{1,2}|\^{1,2}|=)?([a-gA-G])([,']*)(\d*)(-?)$", tok)
    if not m:
        return None
    acc, letter, marks = m.group(1) or '', m.group(2), m.group(3)
    base = STEP[letter.upper()] + {'': 0, '=': 0, '^': 1, '^^': 2, '_': -1, '__': -2}[acc]
    if letter.islower():
        base += 12
    base += 12 * (marks.count("'") - marks.count(','))
    return 60 + base


def transform(text, note_fn=None, chord_fn=None, section_filter=None,
              voice_filter=None, header_edits=None):
    """Stream-transform ABC text preserving formatting. Callables receive
    (voice, section, bar_idx, note_idx_in_section) context."""
    out_lines, ctx = [], {'voice': 'default', 'section': '', 'bar': 0, 'ni': 0}
    for raw in text.splitlines():
        line = raw.rstrip('\n')
        s = line.strip()
        if not s:
            out_lines.append(line)
            continue
        if s.startswith('%'):
            ctx['section'] = s.lstrip('%').strip()
            out_lines.append(line)
            continue
        hm = re.match(r'^([A-Za-z]):\s*(.*)$', s)
        if hm:
            key = hm.group(1)
            if key == 'V':
                ctx['voice'] = s[2:].strip()
            if header_edits and key in header_edits:
                out_lines.append(f'{key}: {header_edits[key]}')
            else:
                out_lines.append(line)
            continue
        res, i = [], 0
        while i < len(line):
            m = NOTE_RE.match(line, i)
            if m and m.group(2):
                tok = m.group(0)
                dur, tie = m.group(4) or '', m.group(5) or ''
                pm = parse_midi_token(tok)
                if pm is None:
                    res.append(tok)
                elif note_fn and (voice_filter is None or ctx['voice'] in voice_filter) \
                        and (section_filter is None or ctx['section'] in section_filter):
                    new = note_fn(pm, ctx)
                    ctx['ni'] += 1
                    res.append(midi_to_token(new) + dur + tie)
                else:
                    res.append(tok)
                i = m.end()
                continue
            cm = CHORD_RE.match(line, i)
            if cm:
                lab = cm.group(1)
                if chord_fn and (section_filter is None or ctx['section'] in section_filter):
                    lab = chord_fn(lab)
                res.append(f'"{lab}"')
                i = cm.end()
                continue
            if line[i] == '|':
                ctx['bar'] += 1
                ctx['ni'] = 0
            res.append(line[i])
            i += 1
        out_lines.append(''.join(res))
    return '\n'.join(out_lines) + '\n'


def transpose_chord(label, shift):
    m = re.match(r'([A-G])([#b]?)(.*)', label)
    if not m:
        return label
    pc = STEP[m.group(1)] + (1 if m.group(2) == '#' else -1 if m.group(2) == 'b' else 0)
    new = PC_SHARP[(pc + shift) % 12]
    return new + m.group(3)


def load_replay(name):
    j = json.load(open(f'{DEP}/{name}_replay.json', encoding='utf-8'))
    return j


def emit(tag, style, lyrics, abc, duration, seed, cot='full'):
    req = {'style': style, 'lyrics': lyrics, 'abc': abc, 'cot': cot,
           'duration': duration, 'seed': seed, 'steps': 32,
           'output_format': 'wav16'}
    json.dump(req, open(f'{OUT}/{tag}.json', 'w', encoding='utf-8'),
              ensure_ascii=False)
    open(f'{OUT}/{tag}.abc', 'w', encoding='utf-8').write(abc)
    return tag


A_SET = ['smoke1', 'smoke2', 'smoke3', 'smoke4', 'verify360']
for name in A_SET:
    j = load_replay(name)
    for seed in (11, 22, 33):
        emit(f'A_{name}_s{seed}', j['style'], j['lyrics'], j['abc'],
             min(j.get('duration') or 60, 180), seed)

base = load_replay('smoke1')
abc0 = base['abc']
# B0 base repeats
for seed in (1, 2):
    emit(f'B_base_s{seed}', base['style'], base['lyrics'], abc0, 60, seed)
# B1 transpose +3
abc_t = transform(abc0, note_fn=lambda m, c: m + 3,
                  chord_fn=lambda l: transpose_chord(l, 3),
                  header_edits={'K': 'Eb'})
for seed in (101, 202):
    emit(f'B_transp3_s{seed}', base['style'], base['lyrics'], abc_t, 60, seed)
# B2 chorus chords substitution
SUB = {'Dm7': 'F', 'G7sus4': 'C7', 'Em7': 'Am7', 'A7': 'Dm7', 'G7': 'C7', 'Cmaj7': 'G7'}
abc_c = transform(abc0, chord_fn=lambda l: SUB.get(l, l),
                  section_filter={'chorus'})
for seed in (101, 202):
    emit(f'B_chords_s{seed}', base['style'], base['lyrics'], abc_c, 60, seed)
# B3 tempo 106 -> 132
abc_q = transform(abc0, header_edits={'Q': '1/4=132'})
for seed in (101, 202):
    emit(f'B_tempo_s{seed}', base['style'], base['lyrics'], abc_q, 60, seed)
# B4 verse melody rewrite (ascending pentatonic in vocal register)
PAT = [69, 71, 72, 74, 76]
abc_m = transform(abc0, note_fn=lambda m, c: PAT[c['ni'] % len(PAT)],
                  section_filter={'verse'}, voice_filter={'Vocal'})
for seed in (101, 202):
    emit(f'B_melody_s{seed}', base['style'], base['lyrics'], abc_m, 60, seed)

PLAIN = [
    ('C1', 'G', '1/4=120', 'G2AB c2d2|e2dc B2A2|G2AB c2d2|e4 z4|'
     'd2B2 A2G2|E2G2 A2B2|c2BA G2E2|D4 z4|'
     'G2AB c2d2|e2dc B2A2|G2AB c2d2|e4 z4|'
     'd2e2 d2c2|B2c2 A2G2|E2D2 E2G2|A2B2 c4|'),
    ('C2', 'D', '1/4=96', 'F2A2 A2F2|E4 D4|F2A2 A2B2|A4 F4|'
     'E2F2 E2D2|C#4 E4|D2F2 E2D2|D4 z4|'
     'F2A2 A2F2|E4 D4|F2A2 A2B2|A4 d4|'
     'c2A2 F2E2|D2E2 F2A2|A2G2 F2E2|D4 z4|'),
]
for tag, key, tempo, body in PLAIN:
    abc = (f'X:1\nT:Plain {tag}\nM:4/4\nL:1/8\nQ:{tempo}\nK:{key}\n{body}\n')
    for seed in (301, 302):
        emit(f'C_{tag}_s{seed}', 'acoustic folk, guitar, instrumental',
             '[Inst]', abc, 45, seed, cot='melody')

made = sorted(os.listdir(OUT))
print(f'stimuli: {len(made)//2} files')
for f in made:
    if f.endswith('.json'):
        print(' ', f)
