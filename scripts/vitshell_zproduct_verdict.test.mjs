// Synthetic classifier inputs only: no window APIs, stack, LLM, or fixture writes.
import assert from 'node:assert/strict';
import { classify } from './vitshell_zproduct_verdict.mjs';

function input() {
  const window = {hwnd:20,pid:200,visible:true,iconic:false,zoomed:false,below_reference:true,
    rect_ok:true,monitor_ok:true,dpi:96,rect:[16,16,1904,1064],workarea:[0,0,1920,1080],
    target_focus_ok:true,target_focus_hwnd:0,target_focus_pid:0};
  const native = {...window,stage:'show_return',tick_ms:1100,reference_hwnd:10,reference_pid:100,binding_valid:true,
    foreground_hwnd:10,foreground_pid:100,focus_ok:true,focus_hwnd:11,focus_pid:100,correction_attempted:false};
  const samples=[];
  for(let t=900;t<=3400;t+=10) samples.push({tick_ms:t,reference_hwnd:10,reference_pid:100,foreground_hwnd:10,foreground_pid:100,
    focus_ok:true,focus_hwnd:11,focus_pid:100,engines:t<1100?[]:[{...window}]});
  return {condition:'shell',condition_met:true,shell_pid:100,shell_hwnd:10,godot_pid:200,expected_foreground_hwnd:10,expected_foreground_pid:100,
    open_tick_ms:1000,native:{samples:[native],message_interval:{start_tick_ms:1050,end_tick_ms:1110,activate_count:0,setfocus_count:0}},
    samples,chain_opened:true,guard_ok:true,project_dom_ok:true};
}
const cases = [
  ['defined positive',d=>{},'PASS'],
  ['permitted correction',d=>{d.native.samples[0].below_reference=false;d.native.samples.push({...d.native.samples[0],stage:'correction_return',tick_ms:1105,below_reference:true,correction_attempted:true});},'PASS'],
  ['natural condition absent',d=>{d.condition_met=false;},'not_exercised'],
  ['missing first snapshot',d=>{d.native.samples=[];},'coverage_gap'],
  ['correction-only snapshot',d=>{d.native.samples[0].stage='correction_return';},'coverage_gap'],
  ['invalid binding',d=>{d.native.samples[0].binding_valid=false;},'coverage_gap'],
  ['wrong bound PID',d=>{d.native.samples[0].pid=201;},'coverage_gap'],
  ['wrong reference HWND',d=>{d.native.samples[0].reference_hwnd=12;},'coverage_gap'],
  ['unknown focus',d=>{d.native.samples[0].focus_ok=false;},'coverage_gap'],
  ['missing message interval',d=>{delete d.native.message_interval;},'coverage_gap'],
  ['negative message count',d=>{d.native.message_interval.activate_count=-1;},'coverage_gap'],
  ['activation between samples',d=>{d.native.message_interval.activate_count=1;},'product_failure'],
  ['setfocus between samples',d=>{d.native.message_interval.setfocus_count=1;},'product_failure'],
  ['native Godot foreground',d=>{d.native.samples[0].foreground_pid=200;},'product_failure'],
  ['poll Godot foreground',d=>{d.samples[30].foreground_pid=200;},'product_failure'],
  ['poll Godot thread focus',d=>{d.samples[30].engines[0].target_focus_pid=200;},'product_failure'],
  ['missing pre-open coverage',d=>{d.samples=d.samples.filter(s=>s.tick_ms>=1000);},'coverage_gap'],
  ['no stable tail',d=>{d.samples=d.samples.filter(s=>s.tick_ms<1800);},'coverage_gap'],
  ['poll hole',d=>{d.samples.splice(30,15);},'coverage_gap'],
  ['poll missing inventory',d=>{delete d.samples[0].engines;},'coverage_gap'],
  ['missing stable bound HWND',d=>{d.samples[50].engines=[];},'coverage_gap'],
  ['ordinary posture failure',d=>{d.native.samples[0].zoomed=true;},'product_failure'],
  ['wrong geometry',d=>{d.native.samples[0].rect[0]=0;},'product_failure'],
  ['uncorrected initial z',d=>{d.native.samples[0].below_reference=false;},'product_failure'],
  ['third correction sample',d=>{d.native.samples.push({...d.native.samples[0]},{...d.native.samples[0]});},'product_failure'],
  ['DOM failure',d=>{d.project_dom_ok=false;},'product_failure'],
  ['observer failure',d=>{d.observer_error='read failure';},'environment_failure'],
  ['external preserved',d=>{d.condition='external';d.expected_foreground_hwnd=30;d.expected_foreground_pid=300;for(const s of [...d.samples,...d.native.samples]){s.foreground_hwnd=30;s.foreground_pid=300;}},'PASS'],
  ['external changed',d=>{d.condition='external';d.expected_foreground_hwnd=30;d.expected_foreground_pid=300;},'product_failure'],
];
for(const [name,mutate,expected] of cases) {const d=input();mutate(d);assert.equal(classify(d).classification,expected,name);}
console.log(`${cases.length} classifier checks passed; synthetic only, no runtime evidence`);
