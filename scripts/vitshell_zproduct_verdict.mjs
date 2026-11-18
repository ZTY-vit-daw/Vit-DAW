// Offline classifier shared by the one-shot product leg and its negative checks.
import { readFileSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function classify(d) {
  const result = (classification, reason) => ({ classification, reason,
    coverage: 'Native first-show + optional correction; own-thread WM_ACTIVATE/WM_SETFOCUS only during message_interval; polling before open through stable. Poll gaps and rendered frames are not covered. No whole-open-interval zero-activation claim.' });
  const gap = reason => result('coverage_gap', reason);
  const fail = reason => result('product_failure', reason);
  const samples = d.samples ?? [], native = d.native?.samples ?? [], interval = d.native?.message_interval;
  const integer = v => Number.isSafeInteger(v) && v >= 0;
  if (!d.condition_met) return result('not_exercised', 'natural foreground condition absent');
  if (d.observer_error) return result('environment_failure', d.observer_error);
  if (!Number.isSafeInteger(d.shell_pid) || !d.shell_hwnd || !d.godot_pid) return gap('missing process/window binding');
  const first = native[0];
  if (!first || first.stage !== 'show_return' || first.correction_attempted) return gap('missing initial pre-correction native snapshot');
  if (native.length > 2 || (native.length === 2 && (native[1].stage !== 'correction_return' || !native[1].correction_attempted))) return fail('unexpected correction trace');
  const identity = s => s.binding_valid === true && s.reference_hwnd === d.shell_hwnd && s.reference_pid === d.shell_pid && s.pid === d.godot_pid && s.hwnd === first.hwnd;
  if (!native.every(identity)) return gap('native launch binding mismatch/invalid');
  if (!interval || !interval.start_tick_ms || ![interval.start_tick_ms,interval.end_tick_ms,interval.activate_count,interval.setfocus_count].every(integer) || interval.start_tick_ms > first.tick_ms || interval.end_tick_ms < native.at(-1).tick_ms) return gap('missing subclass message interval');
  const focusKnown = s => s.focus_ok === true && s.target_focus_ok === true && [s.focus_hwnd,s.focus_pid,s.target_focus_hwnd,s.target_focus_pid,s.foreground_hwnd,s.foreground_pid,s.tick_ms].every(integer);
  if (!native.every(focusKnown)) return gap('native focus identity unavailable');
  if (interval.activate_count || interval.setfocus_count) return fail('own-thread activation/focus message observed');
  const foreground = d.expected_foreground_hwnd;
  if (!foreground || (d.condition === 'shell' && foreground !== d.shell_hwnd) ||
      (d.condition === 'external' && (!d.expected_foreground_pid || d.expected_foreground_pid === d.shell_pid))) return gap('foreground identity not bound');
  const steals = s => s.foreground_pid === d.godot_pid || s.focus_pid === d.godot_pid || s.target_focus_pid === d.godot_pid;
  if (native.some(steals) || samples.some(s => steals(s) || (s.engines ?? []).some(w => w.pid === d.godot_pid && w.target_focus_pid === d.godot_pid))) return fail('Godot foreground/focus observed');
  if ([...native, ...samples].some(s => s.foreground_hwnd !== foreground || s.foreground_pid !== d.expected_foreground_pid)) return fail('natural foreground changed during observation');
  if (!samples.length || samples[0].tick_ms >= d.open_tick_ms || samples[0].engines?.length ||
      !integer(d.open_tick_ms) || d.open_tick_ms > first.tick_ms || samples.at(-1).tick_ms < interval.end_tick_ms + 2000) return gap('pre-open/initial/stable coverage incomplete');
  if (samples.some((s,i) => !integer(s.tick_ms) || !s.focus_ok || !Array.isArray(s.engines) || ![s.foreground_hwnd,s.foreground_pid,s.focus_hwnd,s.focus_pid].every(integer) || s.reference_hwnd !== d.shell_hwnd || s.reference_pid !== d.shell_pid ||
      (i && (s.tick_ms < samples[i-1].tick_ms || s.tick_ms - samples[i-1].tick_ms > 100)))) return gap('polling gap over 100ms or unknown reference/focus');
  const geometry = s => {
    if (!s.rect_ok || !s.monitor_ok || !s.dpi || !s.rect || !s.workarea || s.rect.length !== 4 || s.workarea.length !== 4 || ![...s.rect,...s.workarea].every(Number.isFinite)) return false;
    const inset = Math.round(16 * s.dpi / 96), [l,t,r,b] = s.workarea;
    return s.rect.every((v,i) => Math.abs(v - [l+inset,t+inset,r-inset,b-inset][i]) <= 2);
  };
  if (native.some(s => !s.rect_ok || !s.monitor_ok || !s.dpi)) return gap('native geometry unavailable');
  if (native.some(s => !s.visible || s.iconic || s.zoomed || !geometry(s))) return fail('native ordinary geometry incorrect');
  // First ordering and the permitted correction are reported independently.
  if (!native.at(-1).below_reference) return fail('final native ordering not below reference');
  const stable = samples.filter(s => s.tick_ms >= native.at(-1).tick_ms + 100);
  if (stable.length < 2 || stable.some(s => {
    const w = (s.engines ?? []).filter(w => w.hwnd === first.hwnd && w.pid === d.godot_pid);
    return w.length !== 1 || !w[0].target_focus_ok;
  })) return gap('bound Godot HWND/focus missing in stable samples');
  if (stable.some(s => { const w=s.engines.find(w => w.hwnd===first.hwnd); return !w.visible || w.iconic || w.zoomed || !w.below_reference; })) return fail('stable posture/ordering incorrect');
  if (!d.chain_opened || !d.guard_ok || !d.project_dom_ok) return fail('product launch/DOM chain did not complete');
  return { ...result('PASS', 'defined native, message-interval and polling assertions passed'),
    initial_below_reference: first.below_reference, correction_attempted: native.length === 2,
    message_interval: interval, sample_count: samples.length };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const verdict = classify(JSON.parse(readFileSync(process.argv[2], 'utf8').replace(/^\uFEFF/, '')));
  writeFileSync(process.argv[3], JSON.stringify(verdict, null, 2));
  console.log(JSON.stringify(verdict));
  process.exitCode = verdict.classification === 'PASS' ? 0 : verdict.classification === 'not_exercised' ? 3 : verdict.classification === 'coverage_gap' ? 4 : 1;
}
