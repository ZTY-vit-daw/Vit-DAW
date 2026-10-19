"""Aggregate metrics + verdict for YUE2-ABC-ADHERENCE-1 (thresholds pre-declared)."""
import json, os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from abclib import compare, parse, pearson, bar_mean_pitches, melody_series

HERE = os.path.dirname(os.path.abspath(__file__))


def load(tag):
    inp = open(f'{HERE}/stimuli/{tag}.abc', encoding='utf-8').read()
    try:
        tr = open(f'{HERE}/transcriptions/{tag}.abc', encoding='utf-8').read()
    except FileNotFoundError:
        return None
    rep = None
    try:
        rep = json.load(open(f'{HERE}/renders/{tag}_replay.json', encoding='utf-8'))
    except Exception:
        pass
    return {'input': inp, 'transcribed': tr, 'replay': rep}


def cross_seed_consistency(tags):
    """pairwise melody-contour correlation among transcriptions of same ABC."""
    series = {}
    for t in tags:
        d = load(t)
        if not d:
            return None
        mi = bar_mean_pitches(parse(d['transcribed']))
        v = max(mi, key=lambda x: len(mi[x])) if mi else None
        series[t] = mi.get(v, []) if v else []
    keys = list(series)
    cors = []
    for i in range(len(keys)):
        for j in range(i + 1, len(keys)):
            c = pearson(series[keys[i]], series[keys[j]])
            if c is not None:
                cors.append(round(c, 3))
    return cors


def main():
    rows = []
    a_groups = {}
    for f in sorted(os.listdir(f'{HERE}/stimuli')):
        if not f.endswith('.abc'):
            continue
        tag = f[:-4]
        d = load(tag)
        if not d:
            rows.append({'tag': tag, 'status': 'MISSING'})
            continue
        m = compare(d['input'], d['transcribed'])
        echo = None
        if d['replay'] and 'abc' in d['replay']:
            echo = d['replay']['abc'].strip() == d['input'].strip()
        row = {'tag': tag, 'status': 'ok', 'replay_echoes_input': echo,
               'm1': m['m1'], 'm2': m['m2'], 'm3': m['m3'], 'm4': m['m4'],
               'm5': m['m5']}
        rows.append(row)
        if tag.startswith('A_'):
            base = tag.rsplit('_s', 1)[0]
            a_groups.setdefault(base, []).append(tag)
    json.dump(rows, open(f'{HERE}/metrics_rows.json', 'w', encoding='utf-8'),
              ensure_ascii=False, indent=1)

    print('=== per-run rows ===')
    for r in rows:
        if r['status'] != 'ok':
            print(f"{r['tag']:22s} {r['status']}")
            continue
        m2 = r['m2']
        m3d = r['m3']['direct'] or {}
        m4 = r['m4']
        print(f"{r['tag']:22s} echo={r['replay_echoes_input']} "
              f"K:{r['m1']['in']}->{r['m1']['out']}({'Y' if r['m1']['match'] else 'N'}) "
              f"BPM:{m2.get('in')}->{m2.get('out')}({'Y' if m2.get('match') else 'N'}) "
              f"chord_root:{'-/%.2f' % m3d['root'] if m3d and m3d.get('root') is not None else '-'} "
              f"corr:{'-' if m4['contour_corr'] is None else format(m4['contour_corr'], '.2f')} "
              f"ivl:{'-' if m4['interval_agree'] is None else format(m4['interval_agree'], '.2f')}")

    print('\n=== Leg A aggregate ===')
    ok = [r for r in rows if r['status'] == 'ok' and r['tag'].startswith('A_')]
    if ok:
        km = [r for r in ok if r['m1']['match']]
        tm = [r for r in ok if r['m2']['match'] is True]
        m3r = [r['m3']['direct']['root'] for r in ok if r['m3']['direct']]
        m3bt = [r['m3']['best_transposition'] for r in ok if r['m3']['best_transposition'] is not None]
        m4c = sorted(r['m4']['contour_corr'] for r in ok if r['m4']['contour_corr'] is not None)
        print(f'key match: {len(km)}/{len(ok)} = {len(km)/len(ok):.0%} (gate >=90%)')
        print(f'tempo +-8%: {len(tm)}/{len(ok)} = {len(tm)/len(ok):.0%} (gate >=80%)')
        if m3r:
            print(f'chord root direct: mean {sum(m3r)/len(m3r):.2f} (gate >=0.70)')
        if m3bt:
            print(f'chord root best-transposition: mean {sum(m3bt)/len(m3bt):.2f} (gate >=0.85)')
        if m4c:
            print(f'contour corr: median {m4c[len(m4c)//2]:.2f} (gate >=0.70)')
    print('\n=== cross-seed consistency (same ABC, 3 seeds) ===')
    for base, tags in sorted(a_groups.items()):
        if len(tags) >= 2:
            print(base, cross_seed_consistency(tags))

    print('\n=== Leg B dose-response ===')
    B_MAP = {
        'B_transp3': 'expect K->Eb(-match to shifted), chords shift +3 (best_transposition==0, direct low)',
        'B_chords': 'expect transcription chords match SUBSTITUTED labels',
        'B_tempo': 'expect BPM ~132 (+/-8%)',
        'B_melody': 'expect contour corr to pentatonic input beats corr to base input',
    }
    for prefix, expect in B_MAP.items():
        rs = [r for r in rows if r['status'] == 'ok' and r['tag'].startswith(prefix)]
        base_rs = [r for r in rows if r['status'] == 'ok' and r['tag'].startswith('B_base')]
        for r in rs:
            print(f"{r['tag']:22s} K:{r['m1']['in']}->{r['m1']['out']} "
                  f"BPM:{r['m2'].get('in')}->{r['m2'].get('out')} "
                  f"chord:{'%.2f' % r['m3']['direct']['root'] if r['m3']['direct'] and r['m3']['direct'].get('root') is not None else '-'}/shift{r['m3']['best_shift']} "
                  f"corr:{'-' if r['m4']['contour_corr'] is None else format(r['m4']['contour_corr'],'.2f')}")
        print(f'  ({prefix}) {expect}')

    print('\n=== Leg C plain-dialect ===')
    for r in rows:
        if r['status'] == 'ok' and r['tag'].startswith('C_'):
            print(f"{r['tag']:22s} K:{r['m1']['in']}->{r['m1']['out']} "
                  f"BPM:{r['m2'].get('in')}->{r['m2'].get('out')} "
                  f"corr:{'-' if r['m4']['contour_corr'] is None else format(r['m4']['contour_corr'],'.2f')} "
                  f"notes:{r['m4']['n_notes_in']}->{r['m4']['n_notes_out']}")


if __name__ == '__main__':
    main()
