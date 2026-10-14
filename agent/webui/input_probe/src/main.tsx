// FE-CEF-KEYINPUT-RECON-1 三层输入事件探针
//
// 目标：在 CEF OSR 容器内对「首次点击→首次输入→切换输入框→后续输入→粘贴」采集
// 三层事件时间线，就地浮层可视化，无需 devtools。
//   焦点层（focus）：activeElement 变化、window focus/blur、document.hasFocus() 轮询、visibility；
//   页面事件层（page）：keydown/keyup/beforeinput/input/paste/composition 是否到达页面（含 target/时间戳）；
//   React 层（react）：受控 input onChange 是否执行、受控值是否被重渲染抹回（值快照对比）。
//
// 设计约束：
// - 时间线记录器与浮层全部为原生 JS——若 React 事件委托失效，浮层与记录必须照常工作；
// - 每条记录同步 console.log 镜像（[:INPROBE] 前缀）——CEF 宿主开 VIT_CEF_DEBUG=1 时经
//   console_message 信号落 Godot 日志，作为页面外第二条取证通道；
// - 时间线持久化到 localStorage，刷新/重载不丢；
// - 不依赖网络（无 CDN、无字体）；产品 dist 与设置页零关联。
import { useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";

type Layer = "focus" | "page" | "react" | "mark";

interface ProbeEvent {
  i: number; // 序号
  wall: number; // Date.now()
  perf: number; // performance.now()
  layer: Layer;
  type: string;
  target: string;
  detail?: Record<string, unknown>;
}

const STORAGE_KEY = "vit_input_probe_timeline_v1";
const MAX_EVENTS = 8000;
const POLL_MS = 300;

const sessionStart = Date.now();
const sessionId = Math.random().toString(36).slice(2, 8);
let events: ProbeEvent[] = [];
let seq = 0;
let consoleMirror = true;

function describeTarget(t: EventTarget | null): string {
  if (!(t instanceof Node)) return String(t);
  if (t instanceof Element) {
    const id = t.id ? `#${t.id}` : "";
    const cls = typeof t.className === "string" && t.className ? `.${t.className.split(/\s+/)[0]}` : "";
    return `<${t.tagName.toLowerCase()}${id}${cls} data-w="${t.getAttribute("data-w") ?? ""}">`;
  }
  return `node:${t.nodeName}`;
}

function load(): void {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as { events?: ProbeEvent[] };
      if (parsed.events) events = parsed.events;
    }
  } catch {
    /* 损坏即弃 */
  }
}

function persist(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ events: events.slice(-MAX_EVENTS) }));
  } catch {
    /* 存储满则忽略 */
  }
}

function record(layer: Layer, type: string, detail?: Record<string, unknown>, target?: string): void {
  const ev: ProbeEvent = {
    i: seq++,
    wall: Date.now(),
    perf: Math.round(performance.now() * 10) / 10,
    layer,
    type,
    target: target ?? "",
    detail,
  };
  events.push(ev);
  if (events.length > MAX_EVENTS * 2) events = events.slice(-MAX_EVENTS);
  appendOverlayRow(ev);
  if (consoleMirror) {
    const compact = { i: ev.i, wall: ev.wall, perf: ev.perf, l: layer, t: type, tg: ev.target, d: detail };
    // eslint-disable-next-line no-console
    console.log("[:INPROBE] " + JSON.stringify(compact));
  }
  persistThrottled();
}

let persistTimer: number | undefined;
function persistThrottled(): void {
  if (persistTimer !== undefined) return;
  persistTimer = window.setTimeout(() => {
    persistTimer = undefined;
    persist();
  }, 500);
}

// --- 原生浮层（不经 React） --------------------------------------------------

const LAYER_COLOR: Record<Layer, string> = {
  focus: "#f5b342",
  page: "#5db4f0",
  react: "#66d17a",
  mark: "#ff6b6b",
};

let overlayList: HTMLDivElement | null = null;
let statusLine: HTMLDivElement | null = null;

function fmtWall(wall: number): string {
  const d = new Date(wall);
  const p = (n: number, w = 2) => String(n).padStart(w, "0");
  return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
}

function appendOverlayRow(ev: ProbeEvent): void {
  if (!overlayList) return;
  const row = document.createElement("div");
  row.className = "ip-row";
  const tag = document.createElement("span");
  tag.textContent = `${ev.i} ${fmtWall(ev.wall)} [${ev.layer}] ${ev.type}`;
  tag.style.color = LAYER_COLOR[ev.layer];
  row.appendChild(tag);
  if (ev.target) {
    const tg = document.createElement("span");
    tg.textContent = " " + ev.target;
    tg.style.color = "#9aa4b2";
    row.appendChild(tg);
  }
  if (ev.detail && Object.keys(ev.detail).length) {
    const dt = document.createElement("span");
    dt.textContent = " " + JSON.stringify(ev.detail);
    dt.style.color = "#c7cdd6";
    row.appendChild(dt);
  }
  overlayList.insertBefore(row, overlayList.firstChild);
  while (overlayList.childElementCount > 400) overlayList.removeChild(overlayList.lastChild!);
}

function buildOverlay(): void {
  const panel = document.createElement("div");
  panel.setAttribute("data-probe-overlay", "1");
  panel.style.cssText = [
    "position:fixed", "top:8px", "right:8px", "width:min(560px,46vw)", "max-height:92vh",
    "display:flex", "flex-direction:column", "z-index:2147483647", "pointer-events:auto",
    "background:rgba(12,15,20,0.93)", "color:#e8ebf0", "font:11px/1.45 ui-monospace,Menlo,monospace",
    "border:1px solid #2a3140", "border-radius:8px", "padding:8px", "box-sizing:border-box",
  ].join(";");
  const head = document.createElement("div");
  head.style.cssText = "font-weight:bold;margin-bottom:4px;color:#fff;";
  head.textContent = `输入探针 FE-CEF-KEYINPUT-RECON-1 session=${sessionId}`;
  statusLine = document.createElement("div");
  statusLine.style.cssText = "color:#f5b342;margin-bottom:4px;white-space:pre-wrap;";
  overlayList = document.createElement("div");
  overlayList.style.cssText = "overflow:auto;flex:1;min-height:120px;max-height:60vh;";
  panel.appendChild(head);
  panel.appendChild(statusLine);
  panel.appendChild(overlayList);
  document.body.appendChild(panel);
}

function updateStatus(): void {
  if (!statusLine) return;
  const ae = document.activeElement;
  const c = counts();
  statusLine.textContent =
    `hasFocus=${document.hasFocus()} visibility=${document.visibilityState} active=${describeTarget(ae)} ` +
    `keydown=${c.keydown} input=${c.input} onChange=${c.onChange} paste=${c.paste} err=${c.error}`;
}

function counts(): Record<string, number> {
  const c: Record<string, number> = { keydown: 0, input: 0, onChange: 0, paste: 0, error: 0 };
  for (let k = events.length - 1, seen = 0; k >= 0 && seen < 2000; k--, seen++) {
    const ev = events[k];
    if (ev.type === "keydown") c.keydown++;
    else if (ev.type === "input") c.input++;
    else if (ev.type === "onChange.exec") c.onChange++;
    else if (ev.type === "paste") c.paste++;
    else if (ev.type === "pageerror" || ev.type === "unhandledrejection") c.error++;
  }
  return c;
}

// --- 第一/二层挂接（原生，capture） -------------------------------------------

function hookGlobalListeners(): void {
  const w = window as Window & { __inputProbe?: unknown };
  window.addEventListener("focus", () => record("focus", "window.focus"), true);
  window.addEventListener("blur", () => record("focus", "window.blur"), true);
  document.addEventListener("focusin", (e) => {
    record("focus", "focusin", { to: describeTarget(e.target) }, describeTarget(e.target));
  }, true);
  document.addEventListener("focusout", (e) => {
    record("focus", "focusout", { from: describeTarget(e.target) }, describeTarget(e.target));
  }, true);

  window.addEventListener("keydown", (e) => {
    record("page", "keydown", {
      key: e.key, code: e.code, keyCode: e.keyCode,
      ctrl: e.ctrlKey, meta: e.metaKey, alt: e.altKey, shift: e.shiftKey,
      repeat: e.repeat, isComposing: e.isComposing, trusted: e.isTrusted,
    }, describeTarget(e.target));
  }, true);
  window.addEventListener("keyup", (e) => {
    record("page", "keyup", { key: e.key, code: e.code }, describeTarget(e.target));
  }, true);

  window.addEventListener("pointerdown", (e) => {
    record("page", "pointerdown", { x: e.clientX, y: e.clientY, button: e.button, trusted: e.isTrusted }, describeTarget(e.target));
  }, true);

  document.addEventListener("beforeinput", (e) => {
    const ie = e as InputEvent;
    record("page", "beforeinput", { inputType: ie.inputType, data: ie.data, isComposing: ie.isComposing }, describeTarget(e.target));
  }, true);
  document.addEventListener("input", (e) => {
    const t = e.target as HTMLInputElement | HTMLTextAreaElement | null;
    record("page", "input", {
      inputType: (e as InputEvent).inputType,
      data: (e as InputEvent).data,
      domValue: t && "value" in t ? t.value : undefined,
    }, describeTarget(e.target));
  }, true);
  window.addEventListener("paste", (e) => {
    let len: number | undefined;
    let types = "";
    try {
      const cd = (e as ClipboardEvent).clipboardData;
      if (cd) {
        types = cd.types ? Array.from(cd.types).join(",") : "";
        const text = cd.getData("text/plain");
        len = text.length;
      }
    } catch {
      /* 拒绝访问也如实记录 */
    }
    record("page", "paste", { textLen: len, types, trusted: e.isTrusted }, describeTarget(e.target));
  }, true);

  window.addEventListener("compositionstart", (e) => {
    record("page", "compositionstart", {}, describeTarget(e.target));
  }, true);
  window.addEventListener("compositionend", (e) => {
    record("page", "compositionend", { data: (e as CompositionEvent).data }, describeTarget(e.target));
  }, true);

  window.addEventListener("error", (e) => {
    const msg = e instanceof ErrorEvent ? e.message : `resource:${describeTarget(e.target)}`;
    record("page", "pageerror", { message: msg });
  }, true);
  window.addEventListener("unhandledrejection", (e) => {
    record("page", "unhandledrejection", { reason: String((e as PromiseRejectionEvent).reason) });
  }, true);

  w.__inputProbe = {
    sessionId,
    sessionStart,
    // 取事件数组用函数（活绑定）；直接暴露数组引用会在 clear() 后变陈旧。
    events: () => events,
    mark: (label: string) => record("mark", "user-mark", { label }),
    clear: () => {
      // 原地清空而非重绑：重绑会让已有引用与闭包脱钩。
      events.length = 0;
      seq = 0;
      persist();
      if (overlayList) overlayList.textContent = "";
      record("mark", "cleared");
    },
    dump: () => JSON.stringify(dumpObject(), null, 1),
    setConsoleMirror: (on: boolean) => { consoleMirror = on; },
  };
}

function dumpObject() {
  return {
    probe: "FE-CEF-KEYINPUT-RECON-1",
    sessionId,
    sessionStart,
    dumpAt: Date.now(),
    ua: navigator.userAgent,
    platform: navigator.platform,
    href: location.href,
    dpr: window.devicePixelRatio,
    events,
  };
}

// --- 焦点层轮询（变化才记录） -------------------------------------------------

let lastHasFocus: boolean | null = null;
let lastActive: string | null = null;
let lastVisibility: string | null = null;

function startFocusPoll(): void {
  window.setInterval(() => {
    const hf = document.hasFocus();
    if (hf !== lastHasFocus) {
      lastHasFocus = hf;
      record("focus", "hasFocus.change", { hasFocus: hf });
    }
    const ae = describeTarget(document.activeElement);
    if (ae !== lastActive) {
      lastActive = ae;
      record("focus", "activeElement.change", { active: ae });
    }
    const vis = document.visibilityState;
    if (vis !== lastVisibility) {
      lastVisibility = vis;
      record("focus", "visibility.change", { visibility: vis });
    }
    updateStatus();
  }, POLL_MS);
}

// --- React 层受控组件（第三层测试对象） ---------------------------------------

interface ControlledProps {
  id: string;
  label: string;
}

function ControlledInput({ id, label }: ControlledProps) {
  const [value, setValue] = useState("");
  const valueRef = useRef(value);
  const elRef = useRef<HTMLInputElement | null>(null);
  valueRef.current = value;

  // 抹回检测：原生 input 事件后下一帧对比 DOM 值与 React 状态。
  useEffect(() => {
    const el = elRef.current;
    if (!el) return;
    const onNativeInput = () => {
      const domAtInput = el.value;
      const stateAtInput = valueRef.current;
      // 用 setTimeout 而非 rAF：后台/遮挡视口 rAF 可能停转，探针必须环境稳健。
      window.setTimeout(() => {
        const state = valueRef.current;
        const dom = el.value;
        let verdict: string;
        if (dom === state) {
          verdict = domAtInput === stateAtInput ? "no-change" : "state-synced";
        } else {
          verdict = dom.length < domAtInput.length || dom === stateAtInput ? "wiped-back" : "dom-drift";
        }
        record("react", "value.check", {
          widget: id, verdict, dom, state, domAtInput, stateAtInput,
        }, describeTarget(el));
      });
    };
    el.addEventListener("input", onNativeInput);
    return () => el.removeEventListener("input", onNativeInput);
  }, [id]);

  useEffect(() => {
    record("react", "commit", { widget: id, value });
  }, [value, id]);

  return (
    <label style={{ display: "block", margin: "10px 0" }}>
      <span style={{ display: "inline-block", width: "190px" }}>{label}</span>
      <input
        id={id}
        data-w={id}
        ref={elRef}
        value={value}
        onChange={(e) => {
          record("react", "onChange.exec", { widget: id, from: valueRef.current, to: e.target.value }, describeTarget(e.target));
          setValue(e.target.value);
        }}
        placeholder="React 受控输入"
        style={{ width: "260px" }}
      />
      <span style={{ marginLeft: "8px", color: "#666" }}>state=&quot;{value}&quot;</span>
    </label>
  );
}

function ProbeApp() {
  const nativeRef = useRef<HTMLInputElement | null>(null);
  const [jsonView, setJsonView] = useState("");

  useEffect(() => {
    record("mark", "probe.ready", { href: location.href, ua: navigator.userAgent });
    updateStatus();
  }, []);

  const mark = (label: string) => record("mark", "user-mark", { label });

  const copyJson = async () => {
    const text = JSON.stringify(dumpObject());
    try {
      await navigator.clipboard.writeText(text);
      mark("copy-json.ok");
    } catch {
      setJsonView(text);
      mark("copy-json.fallback-textarea");
    }
  };

  const downloadJson = () => {
    try {
      const blob = new Blob([JSON.stringify(dumpObject(), null, 1)], { type: "application/json" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `input_probe_${sessionId}_${new Date().toISOString().replace(/[:.]/g, "-")}.json`;
      a.click();
      mark("download-json.attempt");
    } catch (err) {
      mark("download-json.failed");
      record("page", "download.error", { message: String(err) });
    }
  };

  return (
    <div style={{ fontFamily: "system-ui, sans-serif", margin: "16px", maxWidth: 760 }}>
      <h2 style={{ marginTop: 0 }}>CEF 输入会话探针（FE-CEF-KEYINPUT-RECON-1）</h2>
      <p style={{ color: "#444" }}>
        复现序列：①点击 A 框输入几个字 → ②点击 B 框输入几个字 → ③点回 A 框再输入 → ④在任意框 ⌘V 粘贴。
        每步之间可按一次「标记」。右侧浮层为时间线（事件到没到、焦点在哪层，就地可见，无需 devtools）。
      </p>

      <fieldset style={{ border: "1px solid #bbb", padding: "8px 12px" }}>
        <legend>测试组件</legend>
        <label style={{ display: "block", margin: "10px 0" }}>
          <span style={{ display: "inline-block", width: "190px" }}>A · 原生输入框（非 React）</span>
          <input id="native-a" data-w="native-a" ref={nativeRef} defaultValue="" placeholder="原生 uncontrolled" style={{ width: "260px" }} />
        </label>
        <ControlledInput id="react-b" label="B · React 受控框①" />
        <ControlledInput id="react-c" label="C · React 受控框②" />
      </fieldset>

      <div style={{ margin: "12px 0" }}>
        {[
          ["标记：刚完成①", "step-1"],
          ["标记：刚完成②", "step-2"],
          ["标记：刚完成③", "step-3"],
          ["标记：刚完成④粘贴", "step-4"],
        ].map(([text, label]) => (
          <button key={label} onClick={() => mark(label)} style={{ margin: "0 8px 8px 0" }}>{text}</button>
        ))}
        <button onClick={copyJson} style={{ margin: "0 8px 8px 0" }}>复制 JSON</button>
        <button onClick={downloadJson} style={{ margin: "0 8px 8px 0" }}>下载 JSON</button>
        <button
          onClick={() => { if (nativeRef.current) nativeRef.current.focus(); mark("refocus-native-a"); }}
          style={{ margin: "0 8px 8px 0" }}
        >
          程序聚焦 A 框
        </button>
        <button
          onClick={() => { const w = window as unknown as { __inputProbe?: { clear: () => void } }; w.__inputProbe?.clear(); }}
        >清空时间线</button>
      </div>

      {jsonView ? (
        <textarea
          readOnly
          value={jsonView}
          onFocus={(e) => e.currentTarget.select()}
          style={{ width: "100%", height: "180px", font: "11px ui-monospace, monospace" }}
        />
      ) : null}
    </div>
  );
}

// --- 启动 ----------------------------------------------------------------------

load();
const rootEl = document.getElementById("root")!;
buildOverlay();
hookGlobalListeners();
record("mark", "probe.load", { sessionId, restored: events.length });
startFocusPoll();
createRoot(rootEl).render(<ProbeApp />);
