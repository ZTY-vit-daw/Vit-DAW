// G0-SCORE-SPIKE-1 / Spike B: Verovio 6.4.1（MEI 模型归库方）
// 零栈：node 无头 wasm；npm 包仅装于本 scratch；不碰生产 src/。
// 与 Spike A 同一 3 小节 2/4 内容（附点/休止/切分延音/跨小节线延音）。
// 腿：
//  B1 MEI(xml:id) -> renderToSVG：id 可寻址 + 拍号/休止/附点渲染
//  B2 renderToTimemap：noteId->时间映射（游标时钟）
//  B3 renderToMIDI：base64 SMF -> 解析；延音链合并 = 内核形状（pitch/start/len 集合）
//  B4 编辑往返：MEI 字符串手术（删除延音对/改时值/去附点拍内改值）-> loadData -> 重渲染差异
import createVerovioModule from 'verovio/wasm';
import { VerovioToolkit } from 'verovio/esm';
import fs from 'node:fs';
import path from 'node:path';

const ART = path.join(import.meta.dirname, 'artifacts');
fs.mkdirSync(ART, { recursive: true });
const results = { legs: {}, timings: {}, assertions: 0 };
function ok(name, cond, detail) {
  results.assertions += 1;
  if (!cond) throw new Error(`ASSERT FAIL [${name}] ${detail ?? ''}`);
}

const MEI = `<?xml version="1.0" encoding="UTF-8"?>
<mei xmlns="http://www.music-encoding.org/ns/mei" meiversion="5.0">
  <meiHead/>
  <music>
    <body>
      <mdiv>
        <score>
          <scoreDef>
            <staffGrp>
              <staffDef n="1" lines="5" clef.shape="G" clef.line="2" meter.count="2" meter.unit="4"/>
            </staffGrp>
          </scoreDef>
          <section>
            <measure n="1">
              <staff n="1"><layer n="1">
                <note xml:id="n1" pname="c" oct="5" dur="8" dots="1"/>
                <note xml:id="n2" pname="g" oct="4" dur="16"/>
                <rest dur="8"/>
                <note xml:id="n3" pname="b" oct="4" dur="8"/>
              </layer></staff>
            </measure>
            <measure n="2">
              <staff n="1"><layer n="1">
                <rest dur="8"/>
                <note xml:id="n4a" pname="e" oct="5" dur="8" tie="i"/>
                <note xml:id="n4b" pname="e" oct="5" dur="8" tie="t"/>
                <note xml:id="n5a" pname="g" oct="5" dur="8" tie="i"/>
              </layer></staff>
            </measure>
            <measure n="3">
              <staff n="1"><layer n="1">
                <note xml:id="n5b" pname="g" oct="5" dur="8" tie="t"/>
                <rest dur="8"/>
                <rest dur="4"/>
              </layer></staff>
            </measure>
          </section>
        </score>
      </mdiv>
    </body>
  </music>
</mei>`;

// 内核期望（与 Spike A 的 kernel 模型一致；quarter 为单位）：延音链在 MIDI 中应合并
const KERNEL_EXPECT = [
  [72, 0.0, 0.75],   // n1 附点八分
  [67, 0.75, 0.25],  // n2 十六分
  [71, 1.5, 0.5],    // n3 八分
  [76, 2.5, 1.0],    // n4a+n4b 延音合并（切分）
  [79, 3.5, 1.0],    // n5a+n5b 跨小节线延音合并
];

const t0 = performance.now();
const VerovioModule = await createVerovioModule();
const tk = new VerovioToolkit(VerovioModule);
results.timings.wasm_init_ms = Math.round(performance.now() - t0);
ok('B0-version', /^6\./.test(tk.getVersion() ?? ''), `version=${tk.getVersion()}`);

tk.setOptions({ scale: 40, pageHeight: 600, pageWidth: 1100, adjustPageHeight: true, breaks: 'auto' });

// ---------- B1 渲染 + id 可寻址 ----------
let t = performance.now();
ok('B1-load', tk.loadData(MEI), 'loadData failed');
const svg1 = tk.renderToSVG(1);
results.timings.render3bars_ms = Math.round(performance.now() - t);
fs.writeFileSync(path.join(ART, 'state0-initial.svg'), svg1);
for (const id of ['n1', 'n2', 'n3', 'n4a', 'n4b', 'n5a', 'n5b']) {
  ok(`B1-id-${id}`, svg1.includes(`id="${id}"`), `svg misses id=${id}`);
}
ok('B1-timesig', /2/.test(svg1) && svg1.includes('meterSig'), 'no meter signature rendered');
ok('B1-dot', (svg1.match(/class="dots?"/g) || []).length >= 1, 'no dot glyph');
ok('B1-tie', (svg1.match(/class="tie"/g) || []).length >= 2, `tie count=${(svg1.match(/class="tie"/g) || []).length}`);
ok('B1-rest', svg1.includes('rest'), 'no rest glyph');
results.legs.B1_render = `PASS (SVG ${svg1.length}B；7 note id 全部可寻址；拍号/附点/延音/休止在场)`;

// ---------- B2 timemap（游标时钟） ----------
const tmRaw = tk.renderToTimemap({ includeMeasures: true, includeRests: true });
const timemap = typeof tmRaw === 'string' ? JSON.parse(tmRaw) : tmRaw;
ok('B2-tm-shape', Array.isArray(timemap) && timemap.length >= 5, `tm=${JSON.stringify(timemap).slice(0, 200)}`);
let timeMono = true, lastT = -1;
for (const entry of timemap) if (entry.on < lastT) timeMono = false; else lastT = entry.on;
ok('B2-mono', timeMono, 'timemap on-times not monotonic');
const allNoteIds = new Set();
for (const entry of timemap) for (const ev of entry.on ?? []) if (Array.isArray(ev)) allNoteIds.add(ev[1]); else if (typeof ev === 'string') allNoteIds.add(ev);
ok('B2-ids-in-tm', ['n1', 'n4a', 'n5b'].every(x => [...allNoteIds].some(y => y.includes(x))), `ids=${[...allNoteIds].join(',')}`);
const measureCount = timemap.filter(e => e.measureOn !== undefined).length;
ok('B2-measures', measureCount >= 3, `measureOn entries=${measureCount}`);
const qstamps = timemap.map(e => e.qstamp);
ok('B2-qstamp', qstamps.every(q => typeof q === 'number') && qstamps[qstamps.length - 1] >= 5, `qstamps=${JSON.stringify(qstamps)}`);
results.legs.B2_timemap = `PASS (${timemap.length} 节点，tstamp 单调；measureOn 锚 ${measureCount}；qstamp 直接可作 beat 坐标；noteId 披露)`;

// ---------- B3 renderToMIDI ----------
const b64 = tk.renderToMIDI();
const smf = Buffer.from(b64, 'base64');
fs.writeFileSync(path.join(ART, 'verovio.mid'), smf);
// 最小 SMF 解析（沿用 Spike A 同构解析器）
function parseSMF(buf) {
  let pos = 0;
  const u32 = () => { const v = buf.readUInt32BE(pos); pos += 4; return v; };
  if (buf.toString('ascii', 0, 4) !== 'MThd') throw new Error('not SMF');
  pos = 4; u32(); pos += 2; const ntrks = buf.readUInt16BE(pos); pos += 2; const division = buf.readUInt16BE(pos); pos += 2;
  const notes = []; const open = new Map();
  for (let trk = 0; trk < ntrks; trk++) {
    if (buf.toString('ascii', pos, pos + 4) !== 'MTrk') throw new Error('bad track');
    pos += 4; const len = u32(); const end = pos + len; let tick = 0; let running = 0;
    while (pos < end) {
      let d = 0, b; do { b = buf[pos++]; d = (d << 7) | (b & 127); } while (b & 128);
      tick += d;
      let st = buf[pos];
      if (st < 0x80) st = running; else { pos++; running = st; }
      if (st === 0xFF) { const type = buf[pos++]; let l = 0; do { b = buf[pos++]; l = (l << 7) | (b & 127); } while (b & 128); pos += l; }
      else if (st >= 0xF0) { let l = 0; do { b = buf[pos++]; l = (l << 7) | (b & 127); } while (b & 128); pos += l; }
      else {
        const n = st & 0xF0, ch = st & 0x0F;
        if (n === 0x90 && buf[pos + 1] > 0) { open.set(ch * 128 + buf[pos], { p: buf[pos], t: tick }); pos += 2; }
        else if (n === 0x80 || n === 0x90) { const key = ch * 128 + buf[pos]; const o = open.get(key); if (o) { notes.push({ pitch: o.p, start: o.t, end: tick }); open.delete(key); } pos += 2; }
        else if (n === 0xC0 || n === 0xD0) pos += 1;
        else pos += 2;
      }
    }
    pos = end;
  }
  return { division, notes };
}
const parsed = parseSMF(smf);
ok('B3-division', parsed.division > 0, `division=${parsed.division}`);
// 绝对起点取决于 Verovio 默认 tempo（记谱 q 单位换算：tick/division），做比例归一比较
const q = parsed.division;
const got = parsed.notes.map(n => [n.pitch, n.start / q, (n.end - n.start) / q]).sort((a, b) => a[1] - b[1]);
ok('B3-count', got.length === KERNEL_EXPECT.length, `got=${JSON.stringify(got)}`);
// Verovio MIDI 引擎对每个音符施加固定 note-off gap（实测 ≈0.00833q = 4/480 tick）：
// 时值不精确回传，MEI 才是时值真值——记为选型发现。
const gaps = got.map((x, i) => KERNEL_EXPECT[i][2] - x[2]);
ok('B3-gap-uniform', gaps.every(g => g > 0 && g < 0.02), `gaps=${JSON.stringify(gaps)}`);
ok('B3-set', got.every((x, i) => x[0] === KERNEL_EXPECT[i][0] && Math.abs(x[1] - KERNEL_EXPECT[i][1]) < 1e-6 && Math.abs(x[2] - KERNEL_EXPECT[i][2]) < 0.02),
  `got=${JSON.stringify(got)} expect=${JSON.stringify(KERNEL_EXPECT)}`);
results.legs.B3_midi = `PASS (SMF ${smf.length}B division=${parsed.division}；延音链已合并；pitch/start 全等；时值带统一 note-off gap≈${gaps[0].toFixed(5)}q——W18 须以 MEI 为时值真值)`;

// ---------- B4 编辑往返（MEI 字符串手术 -> loadData -> 重渲染差异） ----------
function rerender(mei, name) {
  ok(`B4-load-${name}`, tk.loadData(mei), `loadData ${name} failed`);
  const svg = tk.renderToSVG(1);
  fs.writeFileSync(path.join(ART, `state-${name}.svg`), svg);
  return svg;
}
const countTies = (s) => (s.match(/class="tie"/g) || []).length;
const ties0 = countTies(svg1);

// e1 删除延音对 n4a/n4b -> 两枚八分休止
const mei1 = MEI.replace('<note xml:id="n4a" pname="e" oct="5" dur="8" tie="i"/>', '<rest dur="8"/>')
  .replace('<note xml:id="n4b" pname="e" oct="5" dur="8" tie="t"/>', '<rest dur="8"/>');
ok('B4-e1-differs', mei1 !== MEI, 'replace no-op');
const svg1e = rerender(mei1, 'del-tiepair');
ok('B4-e1-ids-gone', !svg1e.includes('id="n4a"') && !svg1e.includes('id="n4b"'), 'n4 ids still present');
ok('B4-e1-ties', countTies(svg1e) === ties0 - 1, `ties ${countTies(svg1e)} vs ${ties0}`);

// e2 改时值：n3 八分 -> 四分（同时删其前八分休止，保持小节合法）
const mei2 = MEI.replace('<rest dur="8"/>\n                <note xml:id="n3" pname="b" oct="4" dur="8"/>',
  '<note xml:id="n3" pname="b" oct="4" dur="4"/>');
ok('B4-e2-differs', mei2 !== MEI, 'replace no-op');
const svg2e = rerender(mei2, 'resize-n3');
ok('B4-e2-id-kept', svg2e.includes('id="n3"'), 'n3 id lost');

// e3 去附点：n1 附点八分 -> 八分，尾随补一枚十六分休止
const mei3 = MEI.replace('<note xml:id="n1" pname="c" oct="5" dur="8" dots="1"/>',
  '<note xml:id="n1" pname="c" oct="5" dur="8"/><rest dur="16"/>');
ok('B4-e3-differs', mei3 !== MEI, 'replace no-op');
const svg3e = rerender(mei3, 'undot-n1');
ok('B4-e3-dot-gone', (svg3e.match(/class="dots?"/g) || []).length === 0, `dots=${(svg3e.match(/class="dots?"/g) || []).length}`);
results.legs.B4_edit = 'PASS (字符串手术 3 例：删延音对/改时值/去附点，重载重渲染差异正确)';

fs.writeFileSync(path.join(ART, 'results.json'), JSON.stringify(results, null, 2));
console.log('SPIKE B (Verovio 6.4.1 / MEI): ALL PASS');
console.log(JSON.stringify(results, null, 2));
