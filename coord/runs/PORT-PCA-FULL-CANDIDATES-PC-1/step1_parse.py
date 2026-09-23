# PORT-PCA-FULL-CANDIDATES-PC-1 step1 inventory parser (PC side)
# Parses 24 pca_jobs + attestation store + whitelist v5 + semantic index,
# emits step1_pc_pca_inventory.json with per-subject axis verdicts.
import json, os, glob, base64
from collections import Counter, defaultdict
from datetime import datetime

BASE = r'C:/Users/timoz/.vit/pca_certifications'
OUT = r'D:/Vit_DAW/coord/runs/PORT-PCA-FULL-CANDIDATES-PC-1'

def decode_ref(ref):
    payload = ref.split('.')[0]
    _, _, b = payload.partition('_')
    b += '=' * (-len(b) % 4)
    d = json.loads(base64.urlsafe_b64decode(b))
    return {'role': d.get('role'), 'param_id': str(d.get('param_id')),
            'band_key': d.get('band_key'), 'stage_key': d.get('stage_key')}

jobs = {}
for job in sorted(os.listdir(BASE)):
    sp = os.path.join(BASE, job, 'summary.json')
    if not os.path.isfile(sp):
        continue
    s = json.load(open(sp, encoding='utf-8'))
    for r in s.get('results', []):
        evp = glob.glob(os.path.join(BASE, job, 'cases', '*', 'evidence.json'))
        ev = json.load(open(evp[0], encoding='utf-8')) if evp else {}
        fr = ev.get('frozen_resolution', {})
        fwd = [decode_ref(c['control_ref']) for c in ev.get('selected_forward_controls', []) if 'control_ref' in c]
        ident = r.get('identifier')
        jobs[ident] = {
            'pca_job': job, 'name': r.get('plugin_name'),
            'form': 'Stereo' if 'Stereo' in (r.get('plugin_name') or '') else 'Mono',
            'family': r.get('processor_family') or r.get('expectation'),
            'family_field_missing': r.get('processor_family') is None,
            'expectation': r.get('expectation'), 'status': r.get('status'),
            'apply_status': r.get('apply_status'), 'restore_status': r.get('restore_status'),
            'topology_generation_stable': r.get('topology_generation_stable'),
            'manufacturer': fr.get('manufacturer'), 'plugin_path': fr.get('plugin_path'),
            'parameter_count': r.get('parameter_count'),
            'certified_roles': sorted(r.get('selected_roles') or []),
            'anchors': [{'role': f['role'], 'param_id': f['param_id'], 'band': f['band_key']} for f in fwd],
            'apply_tool': ev.get('apply_tool'),
            'evidence_path': evp[0] if evp else None,
            'summary_path': sp,
        }

att = json.load(open(r'C:/Users/timoz/.vit/processor_control_attestations.v2.json', encoding='utf-8'))
latest = {}
for a in att['attestations']:
    ident = a['subject']['identifier']
    ts = a.get('promoted_at') or a.get('issued_at')
    if ident not in latest or ts > latest[ident][0]:
        latest[ident] = (ts, a)
promoted = {k: v[1] for k, v in latest.items() if v[1]['status'] == 'promoted'}

wl = json.load(open(r'C:/Users/timoz/.vit/free_state_experiment_plugins.json', encoding='utf-8'))
wl_entries = {}
for fam, d in wl.items():
    if fam == 'schema_version':
        continue
    wl_entries[d.get('plugin_identifier')] = {'family_key': fam, 'name': d.get('plugin_name'),
                                              'manufacturer': d.get('manufacturer')}

sem = json.load(open(r'C:/Users/timoz/.vit/plugin_semantics.json', encoding='utf-8'))
sem_by_id = {e['identifier']: e for e in sem['entries'] if e.get('identifier')}

# frozen seven-family experiment axes (mac card frozen rule)
AXIS = {
    'de_esser': {'axis': 'threshold', 'role_names': {'threshold'}, 'att_axes': {'threshold_sensitivity'}},
    'limiter': {'axis': 'ceiling', 'role_names': {'ceiling'}, 'att_axes': {'output_ceiling'}},
    'gate_expander': {'axis': 'range', 'role_names': {'range'}, 'att_axes': {'attenuation_floor'}},
    'transient_shaper': {'axis': 'attack', 'role_names': {'attack', 'attack_duration'}, 'att_axes': {'envelope_timing'}},
    'multiband_dynamics': {'axis': 'band thresholds', 'role_names': set(), 'att_axes': {'band_dynamics'}},
}
FAM2WL = {'de_esser': 'de_esser', 'limiter': 'limiter', 'gate_expander': 'gate_expander',
          'transient_shaper': 'transient_shaper', 'multiband_dynamics': 'multiband',
          'compressor': 'broadband_compression'}

def axis_verdict(ident):
    job = jobs.get(ident)
    a = promoted.get(ident)
    fam = (job or {}).get('family') or (a['processor_family'] if a else None)
    if fam not in AXIS:
        return ('n/a', 'compressor 族不在七实验族轴规则内（broadband 映射待决策）' if fam == 'compressor' else '族不在规则内')
    ax = AXIS[fam]
    role_hit, att_hit = set(), set()
    if job:
        role_hit = ax['role_names'] & set(job['certified_roles'])
    if a:
        att_hit = ax['att_axes'] & {c['axis'] for c in a['coverage']}
    if role_hit or att_hit:
        return ('pass', 'roles=%s att=%s' % (sorted(role_hit) or '-', sorted(att_hit) or '-'))
    near = set()
    if a:
        cov = {c['axis'] for c in a['coverage']}
        near = cov & {'envelope_emphasis'}
    if ident in wl_entries or near:
        cov = sorted({c['axis'] for c in a['coverage']}) if a else '-'
        return ('ambiguous', 'v5_anchor=%s near_axis=%s cov=%s' % (ident in wl_entries, sorted(near), cov))
    cov = ','.join(job['certified_roles']) if job else (','.join(c['axis'] for c in a['coverage']) if a else '-')
    return ('fail', '认证角色/轴不含 %s（%s）' % (ax['axis'], cov))

rows = []
all_idents = sorted(set(list(promoted.keys()) + list(jobs.keys())))
for ident in all_idents:
    job = jobs.get(ident)
    a = promoted.get(ident)
    verdict, detail = axis_verdict(ident)
    fam = (job or {}).get('family') or (a['processor_family'] if a else '?')
    rows.append({
        'identifier': ident,
        'name': (job or a['subject'])['name'],
        'form': 'Stereo' if 'Stereo' in ((job or a['subject'])['name'] or '') else 'Mono',
        'manufacturer': (job or {}).get('manufacturer') or (a['subject'].get('manufacturer') if a else '?'),
        'family': fam,
        'whitelist_family': FAM2WL.get(fam),
        'pca_job': (job or {}).get('pca_job'),
        'promoted': a is not None,
        'promoted_at': a.get('promoted_at') if a else None,
        'certified': job is not None,
        'cert_status': (job or {}).get('status'),
        'certified_roles': (job or {}).get('certified_roles'),
        'anchors': (job or {}).get('anchors'),
        'attestation_axes': [c['axis'] for c in a['coverage']] if a else None,
        'axis_verdict': verdict, 'axis_detail': detail,
        'v5_entry': ident in wl_entries,
        'semantic_index_hit': ident in sem_by_id,
        'semantic_category': sem_by_id.get(ident, {}).get('category'),
    })

fam_counts = defaultdict(Counter)
for r in rows:
    key = r['whitelist_family']
    if key:
        fam_counts[key][r['axis_verdict']] += 1
        fam_counts[key]['total'] += 1

inventory = {
    'card': 'PORT-PCA-FULL-CANDIDATES-PC-1',
    'step': 'step1_inventory',
    'generated_at': datetime.now().isoformat(timespec='seconds'),
    'machine': 'PC (win32)',
    'sources': {
        'pca_certifications': {'dir': BASE, 'jobs': len(jobs),
                               'all_passed': all(j['status'] == 'passed' for j in jobs.values())},
        'attestation_store': {'file': r'C:\Users\timoz\.vit\processor_control_attestations.v2.json',
                              'revision': att['revision'], 'receipts': len(att['attestations']),
                              'distinct_subjects': len(latest), 'promoted': len(promoted)},
        'whitelist_v5': {'file': r'C:\Users\timoz\.vit\free_state_experiment_plugins.json', 'entries': len(wl_entries)},
        'semantic_index': {'file': r'C:\Users\timoz\.vit\plugin_semantics.json', 'entries': len(sem['entries']),
                           'summary': sem['summary']},
    },
    'frozen_axis_rule': {'static_eq': '增益带', 'de_esser': 'threshold', 'transient_shaper': 'attack',
                         'limiter': 'ceiling', 'gate_expander': 'range', 'multiband': 'band thresholds',
                         'broadband_compression': 'threshold'},
    'axis_name_map': {
        'threshold': {'job_role': 'threshold', 'attestation_axis': 'threshold_sensitivity'},
        'ceiling': {'job_role': 'ceiling', 'attestation_axis': 'output_ceiling'},
        'range': {'job_role': 'range', 'attestation_axis': 'attenuation_floor'},
        'attack': {'job_role': 'attack / attack_duration', 'attestation_axis': 'envelope_timing (近轴 envelope_emphasis)'},
        'band_thresholds': {'job_role': 'band_key=band_N + role=gain/attack', 'attestation_axis': 'band_dynamics + band_timing'},
    },
    'family_counts': {k: dict(v) for k, v in fam_counts.items()},
    'subjects': rows,
}
json.dump(inventory, open(os.path.join(OUT, 'step1_pc_pca_inventory.json'), 'w', encoding='utf-8'),
          ensure_ascii=False, indent=1)
print('JSON written')
print()
for fam in ['static_eq', 'broadband_compression', 'de_esser', 'limiter', 'gate_expander', 'transient_shaper', 'multiband']:
    c = fam_counts.get(fam, Counter())
    print('%-22s total=%-3d pass=%-3d ambiguous=%-3d fail=%-3d n/a=%d' % (
        fam, c.get('total', 0), c.get('pass', 0), c.get('ambiguous', 0), c.get('fail', 0), c.get('n/a', 0)))
print()
for r in sorted(rows, key=lambda x: (x['whitelist_family'] or '?', x['name'])):
    print('%-22s %-28s %-6s %-10s %-11s %s' % (
        r['whitelist_family'] or '?', r['name'][:28], r['form'], r['axis_verdict'],
        'promoted' if r['promoted'] else 'NOT-PROM', r['axis_detail'][:70]))
