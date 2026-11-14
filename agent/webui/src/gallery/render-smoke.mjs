/*
 * UI-FOUNDATION-1 浏览器渲染烟测（npm run test:render）。
 *
 * 验收面（卡面）：
 *  1. gallery DOM 断言：23 symbol、16/20/24px × 浅/深/选中矩阵、各组件全状态、
 *     V 标不进标题栏样例、fixture 标注在卷；
 *  2. 1440×900 截图（视口 + 全页）；
 *  3. 对比度：正文 ≥4.5:1、图形 ≥3:1（页面实测，含祖先底色合成）；
 *  4. 无裁切 / 无横向溢出；最小字号 ≥13px；
 *  5. 无外部网络请求（仅本机 dev server 与 data: URL）。
 *
 * 运行方式：node src/gallery/render-smoke.mjs（自起 vite dev server，随机端口）。
 * 工件：coord/runs/UI-FOUNDATION-1/{render-smoke-report.json, screens/*.png}
 * 退出码 0 = 全部通过；任何断言失败退出码 1。
 *
 * UI-LAUNCHER-1（345）扩展：追加 launcher fixture 页（gallery.html?surface=launcher）
 * 渲染烟测——标题栏 Vit 文本/抽屉真实 recent 数/建议/输入/加载/错误/空态
 * DOM 断言 + 对比度/字号/裁切扫描 + 1440×900 截图；工件落
 * coord/runs/UI-LAUNCHER-1/{render-smoke-launcher-report.json, screens/*.png}。
 * 全离线 fixture（零真实端点请求）；344 的 gallery 断言与工件路径不变。
 */

import { chromium } from "playwright";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const here = dirname(fileURLToPath(import.meta.url)); // agent/webui/src/gallery
const webuiRoot = resolve(here, "../.."); // agent/webui
const repoRoot = resolve(webuiRoot, "../.."); // 仓库根
const outDir = resolve(repoRoot, "coord/runs/UI-FOUNDATION-1");
const screensDir = resolve(outDir, "screens");
// UI-LAUNCHER-1（345）：launcher 渲染烟测工件独立落卡目录（344 产物路径不变）。
const launcherOutDir = resolve(repoRoot, "coord/runs/UI-LAUNCHER-1");
const launcherScreensDir = resolve(launcherOutDir, "screens");

/** 与 src/design/sprite.tsx 同源（23 symbol）。 */
const SYMBOL_IDS = [
  "i-project", "i-new", "i-open", "i-minimize", "i-send", "i-stop",
  "i-track", "i-bus", "i-fx", "i-macro", "i-ab", "i-play", "i-record",
  "i-render", "i-undo", "i-snapshot", "i-attach", "i-idle", "i-busy",
  "i-done", "i-error", "i-alert", "i-vit",
];

function contrastRatio(rgbA, rgbB) {
  const lin = (c) => {
    const v = c / 255;
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  };
  const lum = ([r, g, b]) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
  const [a, b] = [lum(rgbA), lum(rgbB)].sort((x, y) => y - x);
  return (a + 0.05) / (b + 0.05);
}

function parseColor(s) {
  // "rgb(r, g, b)" | "rgba(r, g, b, a)" | "transparent"
  const m = /rgba?\(([^)]+)\)/.exec(s);
  if (!m) return null;
  const parts = m[1].split(",").map((x) => parseFloat(x.trim()));
  if (parts.length === 3) return [parts[0], parts[1], parts[2], 1];
  return [parts[0], parts[1], parts[2], parts[3]];
}

function composite(fg, bg) {
  const [fr, fg_, fb, fa] = fg;
  const [br, bgc, bb, ba] = bg;
  const outA = fa + ba * (1 - fa);
  if (outA === 0) return [0, 0, 0, 0];
  const mix = (f, b) => Math.round((f * fa + b * ba * (1 - fa)) / outA);
  return [mix(fr, br), mix(fg_, bgc), mix(fb, bb), outA];
}

async function main() {
  mkdirSync(screensDir, { recursive: true });
  const failures = [];
  const notes = [];

  // ---- 1. 自起 vite dev server（显式 host；端口被占时 vite 自动 +1 并在 resolvedUrls 报告实际值） ----
  const server = await createServer({
    root: webuiRoot,
    logLevel: "error",
    server: { port: 5391, strictPort: false, host: "127.0.0.1" },
  });
  await server.listen();
  const base = (server.resolvedUrls?.local ?? [])[0] ?? "";
  if (!base) throw new Error("vite dev server 未返回 resolvedUrls");
  const galleryUrl = `${base.replace(/\/$/, "").replace("localhost", "127.0.0.1")}/gallery.html`;
  notes.push(`devServer=${galleryUrl}`);

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    reducedMotion: "reduce", // 设计要求减少动态偏好下停转；截图即静帧
  });
  const page = await context.newPage();

  const requests = [];
  page.on("request", (r) => requests.push(r.url()));

  await page.goto(galleryUrl, { waitUntil: "networkidle" });
  await page.waitForSelector(".v5-gal-root");
  await page.waitForTimeout(200);

  const origin = new URL(page.url()).origin;
  const externalRequests = requests.filter((u) => {
    if (u.startsWith("data:") || u.startsWith("blob:")) return false;
    return !u.startsWith(origin);
  });

  // ---- 2. DOM 断言 ----
  const dom = await page.evaluate((ids) => {
    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => [...document.querySelectorAll(sel)];
    const out = {};

    out.spriteMounted = !!$(".v5-sprite");
    out.symbolCount = $$(".v5-sprite symbol").length;
    out.symbolIdsInDom = $$(".v5-sprite symbol").map((s) => s.id).sort();
    out.expectedIds = [...ids].sort();

    // 图标矩阵：每枚 symbol 在 light/dark × 16/20/24 与 selected 上下文各至少一次
    out.matrix = {};
    for (const id of ids) {
      const cell = {};
      for (const bg of ["light", "dark"]) {
        for (const size of [16, 20, 24]) {
          cell[`${bg}-${size}`] = $$(
            `[data-bg="${bg}"] svg[data-icon-size="${size}"] use[href="#${id}"]`
          ).length;
        }
      }
      cell["selected-20"] = $$(
        `[data-bg="selected"] svg[data-icon-size="20"] use[href="#${id}"]`
      ).length;
      out.matrix[id] = cell;
    }

    // 组件全状态
    out.macroStates = $$("[data-macro-state]").map((e) => e.getAttribute("data-macro-state"));
    out.ringStates = ["idle", "busy", "done", "error"].map(
      (st) => $$(`.v5-ring--${st}`).length
    );
    out.statusCardStates = $$("[data-status-card-state]").map((e) =>
      e.getAttribute("data-status-card-state")
    );
    out.actionConsequences = $$("[data-action-consequence]").map((e) =>
      e.getAttribute("data-action-consequence")
    );
    out.inputStates = $$("[data-input-state]").map((e) => e.getAttribute("data-input-state"));
    out.buttonVariants = ["primary", "secondary", "danger", "text"].map(
      (v) => $$(`.v5-btn--${v}`).length
    );
    out.messageAuthors = $$("[data-message-author]").map((e) =>
      e.getAttribute("data-message-author")
    );
    out.snapshotReceipt = $$("[data-snapshot-receipt]").length;
    out.fixtureNotice = !!$('[data-fixture="notice"]');
    out.contextTargets = $$("[data-gallery='context-target'] .v5-context-target").length;
    out.selectedContextTargets = $$(
      "[data-gallery='context-target'] .v5-context-target.is-selected"
    ).length;

    // V 标纪律：标题栏样例内不得出现 i-vit；组件样例中 i-vit 只在 OS 身份样例出现；
    // S3 图标矩阵（资产文档）必须含 i-vit 样张，不计入组件样例。
    out.vitInTitlebar = $$('[data-gallery="titlebar"] use[href="#i-vit"]').length;
    out.vitMarkSamples = $$("[data-vit-mark]").length;
    out.osSamples = $$("[data-os-sample]").length;
    out.vitInIconMatrix = $$('[data-gallery="icon-matrix"] use[href="#i-vit"]').length;
    out.vitOutsideOsSample = $$("use[href='#i-vit']").filter(
      (u) =>
        !u.closest("[data-os-sample]") &&
        !u.closest('[data-gallery="icon-matrix"]')
    ).length;

    // token 面板与页面主结构
    out.tokenSwatches = $$(".v5-gal-token").length;
    out.sectionCount = $$("[data-gallery]").length;
    out.disabledApplyButtons = $$(
      "[data-gallery='macro-cards'] .v5-btn--primary:disabled"
    ).length;
    return out;
  }, SYMBOL_IDS);

  const domAssert = (name, ok, detail) => {
    if (!ok) failures.push(`DOM: ${name} — ${detail}`);
  };

  domAssert("sprite mounted", dom.spriteMounted, ".v5-sprite 不存在");
  domAssert(
    "23 symbols",
    dom.symbolCount === 23 && JSON.stringify(dom.symbolIdsInDom) === JSON.stringify(dom.expectedIds),
    `count=${dom.symbolCount} ids=${JSON.stringify(dom.symbolIdsInDom)}`
  );
  for (const id of SYMBOL_IDS) {
    const m = dom.matrix[id] ?? {};
    const missing = Object.entries(m)
      .filter(([, n]) => n < 1)
      .map(([k]) => k);
    domAssert(`matrix ${id}`, missing.length === 0, `缺上下文: ${missing.join(",")}`);
  }
  const stateSet = (arr) => new Set(arr);
  domAssert(
    "macro 四态",
    stateSet(dom.macroStates).size === 4 &&
      ["default", "auditioning", "applied", "failed"].every((s) => stateSet(dom.macroStates).has(s)),
    JSON.stringify(dom.macroStates)
  );
  domAssert(
    "状态环四态",
    dom.ringStates.length === 4 && dom.ringStates.every((n) => n >= 1),
    JSON.stringify(dom.ringStates)
  );
  domAssert(
    "状态卡三态",
    ["running", "completed", "failed"].every((s) => stateSet(dom.statusCardStates).has(s)),
    JSON.stringify(dom.statusCardStates)
  );
  domAssert(
    "动作卡三后果",
    ["reversible", "confirm", "irreversible"].every((s) =>
      stateSet(dom.actionConsequences).has(s)
    ),
    JSON.stringify(dom.actionConsequences)
  );
  domAssert(
    "输入框三态",
    ["default", "disabled", "busy"].every((s) => stateSet(dom.inputStates).has(s)),
    JSON.stringify(dom.inputStates)
  );
  domAssert(
    "按钮四变体",
    dom.buttonVariants.length === 4 && dom.buttonVariants.every((n) => n >= 1),
    JSON.stringify(dom.buttonVariants)
  );
  domAssert(
    "消息双作者",
    stateSet(dom.messageAuthors).has("user") && stateSet(dom.messageAuthors).has("vit"),
    JSON.stringify(dom.messageAuthors)
  );
  domAssert("快照回执", dom.snapshotReceipt >= 1, "缺快照回执");
  domAssert("fixture 标注", dom.fixtureNotice, "缺 data-fixture=notice");
  domAssert("上下文目标含选中态", dom.contextTargets >= 2 && dom.selectedContextTargets >= 1,
    `targets=${dom.contextTargets} selected=${dom.selectedContextTargets}`);
  domAssert("V 标不进标题栏", dom.vitInTitlebar === 0, `titlebar 内 i-vit=${dom.vitInTitlebar}`);
  domAssert(
    "V 标只在 OS 身份样例（S3 矩阵样张除外）",
    dom.vitMarkSamples >= 2 &&
      dom.osSamples >= 2 &&
      dom.vitOutsideOsSample === 0 &&
      dom.vitInIconMatrix >= 1,
    `marks=${dom.vitMarkSamples} os=${dom.osSamples} outside=${dom.vitOutsideOsSample} matrix=${dom.vitInIconMatrix}`
  );
  domAssert("未选择时应用禁用", dom.disabledApplyButtons >= 1, "缺禁用应用按钮");

  // ---- 3. 对比度 / 字号 / 裁切扫描 ----
  // UI-LAUNCHER-1（345）：同一扫描函数复用于 launcher fixture 页（见 §7）；
  // gallery 页行为不变。
  const scanFn = () =>
    page.evaluate(() => {
    const results = { text: [], graphics: [], tooSmall: [], clipped: [] };

    const parseColor = (s) => {
      const m = /rgba?\(([^)]+)\)/.exec(s);
      if (!m) return null;
      const p = m[1].split(",").map((x) => parseFloat(x.trim()));
      return p.length === 3 ? [p[0], p[1], p[2], 1] : [p[0], p[1], p[2], p[3]];
    };
    const composite = (fg, bg) => {
      const [fr, fgc, fb, fa] = fg;
      const [br, bgc, bb, ba] = bg;
      const outA = fa + ba * (1 - fa);
      if (outA === 0) return [0, 0, 0, 0];
      const mix = (f, b) => Math.round((f * fa + b * ba * (1 - fa)) / outA);
      return [mix(fr, br), mix(fgc, bgc), mix(fb, bb), outA];
    };
    const lin = (c) => {
      const v = c / 255;
      return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
    };
    const lum = ([r, g, b]) => 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
    const ratio = (a, b) => {
      const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
      return (hi + 0.05) / (lo + 0.05);
    };
    const fmt = (c) => `rgb(${c[0]},${c[1]},${c[2]})`;

    // 有效底色：沿祖先链合成（含自身）
    const effectiveBg = (el) => {
      let acc = null;
      let node = el;
      while (node && node.nodeType === 1) {
        const bg = parseColor(getComputedStyle(node).backgroundColor);
        if (bg && bg[3] > 0) {
          acc = acc ? composite(acc, bg) : bg;
          if (acc[3] >= 1) return acc;
        }
        node = node.parentElement;
      }
      return acc ? composite(acc, [255, 255, 255, 1]) : [255, 255, 255, 1];
    };

    const visible = (el) => {
      const cs = getComputedStyle(el);
      if (cs.visibility === "hidden" || cs.display === "none") return false;
      const rect = el.getBoundingClientRect();
      return rect.width > 0 && rect.height > 0;
    };

    // 正文对比度：所有可见文本节点
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    let minRatio = Infinity;
    let minSample = null;
    while (walker.nextNode()) {
      const node = walker.currentNode;
      const text = node.textContent.trim();
      if (!text) continue;
      const el = node.parentElement;
      if (!el || el.closest(".v5-sprite") || el.closest("svg")) continue;
      if (!visible(el)) continue;
      const cs = getComputedStyle(el);
      let fg = parseColor(cs.color);
      if (!fg) continue;
      const bg = effectiveBg(el);
      if (fg[3] < 1) fg = composite(fg, bg);
      const r = ratio(fg, bg);
      if (r < minRatio) {
        minRatio = r;
        minSample = { text: text.slice(0, 24), ratio: Number(r.toFixed(3)), fg: fmt(fg), bg: fmt(bg) };
      }
      if (r < 4.5) {
        results.text.push({ text: text.slice(0, 40), ratio: Number(r.toFixed(3)), fg: fmt(fg), bg: fmt(bg), tag: el.tagName });
      }
      // placeholder 颜色
      if (el.matches("input, textarea")) {
        const pcs = getComputedStyle(el, "::placeholder");
        const pfg = parseColor(pcs.color);
        if (pfg) {
          const pr = ratio(pfg[3] < 1 ? composite(pfg, bg) : pfg, bg);
          if (pr < 4.5) {
            results.text.push({ text: `placeholder:${text.slice(0, 30)}`, ratio: Number(pr.toFixed(3)), fg: fmt(pfg), bg: fmt(bg), tag: "placeholder" });
          }
        }
      }
      // 最小字号 13px
      const fs = parseFloat(cs.fontSize);
      if (fs < 13) {
        results.tooSmall.push({ text: text.slice(0, 24), fontSize: fs, tag: el.tagName });
      }
    }
    results.textMin = minSample;

    // 图形对比度：[data-graphic] 元素（icon 描边 / 曲线 / 进度条 / 状态点 / chip 边）
    for (const el of document.querySelectorAll("[data-graphic]")) {
      if (!visible(el)) continue;
      const cs = getComputedStyle(el);
      const selfBg = parseColor(cs.backgroundColor);
      // 图形色优先级：stroke > border > 自身 bg（对父底）> color
      let fg = null;
      let kind = null;
      if (cs.stroke && cs.stroke !== "none" && parseColor(cs.stroke)) {
        fg = parseColor(cs.stroke);
        kind = "stroke";
      } else if (
        cs.borderTopWidth !== "0px" &&
        cs.borderTopStyle !== "none" &&
        parseColor(cs.borderTopColor)
      ) {
        fg = parseColor(cs.borderTopColor);
        kind = "border";
      } else if (selfBg && selfBg[3] >= 1) {
        fg = selfBg;
        kind = "fill";
      } else {
        fg = parseColor(cs.color);
        kind = "color";
      }
      if (!fg) continue;
      // 参照底色：stroke/color 相对「元素所在的表面」（可含自身 bg）；
      // border/fill 相对「周围的表面」（不含自身，元素自身填充不是它的背景）。
      const bg =
        kind === "border" || kind === "fill"
          ? effectiveBg(el.parentElement)
          : effectiveBg(el);
      const eff = fg[3] < 1 ? composite(fg, bg) : fg;
      const r = ratio(eff, bg);
      const rec = {
        kind: el.getAttribute("data-graphic"),
        mode: kind,
        ratio: Number(r.toFixed(3)),
        fg: fmt(eff),
        bg: fmt(bg),
      };
      results.graphics.push(rec);
      if (r < 3) results.graphicsFail = results.graphicsFail ?? [];
      if (r < 3) (results.graphicsFail ??= []).push(rec);
    }

    // 裁切 / 溢出：hidden|auto|scroll|clip 容器内容超出；文档横向溢出
    for (const el of document.querySelectorAll("body *")) {
      const cs = getComputedStyle(el);
      const clips = [cs.overflowX, cs.overflowY].some((v) =>
        ["hidden", "auto", "scroll", "clip"].includes(v)
      );
      if (!clips) continue;
      if (
        el.scrollWidth > el.clientWidth + 1 ||
        el.scrollHeight > el.clientHeight + 1
      ) {
        results.clipped.push({
          tag: el.tagName,
          cls: (el.className?.baseVal ?? el.className ?? "").toString().slice(0, 60),
          scrollW: el.scrollWidth,
          clientW: el.clientWidth,
          scrollH: el.scrollHeight,
          clientH: el.clientHeight,
        });
      }
    }
    results.docScrollWidth = document.documentElement.scrollWidth;
    return results;
    });
  const scan = await scanFn();

  const contrastAssert = (name, ok, detail) => {
    if (!ok) failures.push(`CONTRAST: ${name} — ${detail}`);
  };
  contrastAssert("正文 ≥4.5:1", scan.text.length === 0, JSON.stringify(scan.text.slice(0, 6)));
  contrastAssert(
    "图形 ≥3:1",
    !scan.graphicsFail || scan.graphicsFail.length === 0,
    JSON.stringify((scan.graphicsFail ?? []).slice(0, 6))
  );
  contrastAssert(
    "最小字号 ≥13px",
    scan.tooSmall.length === 0,
    JSON.stringify(scan.tooSmall.slice(0, 6))
  );

  if (scan.clipped.length > 0) failures.push(`CLIP: 裁切/溢出 ${JSON.stringify(scan.clipped.slice(0, 6))}`);
  if (scan.docScrollWidth > 1440)
    failures.push(`OVERFLOW: 文档横向溢出 scrollWidth=${scan.docScrollWidth} > 1440`);
  if (externalRequests.length > 0)
    failures.push(`NETWORK: 外部请求 ${JSON.stringify(externalRequests.slice(0, 6))}`);

  // ---- 4. reduced-motion 动效停转检查 ----
  const motion = await page.evaluate(() => {
    const ring = document.querySelector(".v5-ring--busy");
    if (!ring) return { found: false, anim: null };
    const cs = getComputedStyle(ring);
    return { found: true, anim: `${cs.animationName} ${cs.animationDuration} ${cs.animationPlayState}` };
  });
  if (!motion.found || motion.anim.startsWith("none")) {
    notes.push(`reduced-motion: busy 环 animation=${motion.anim ?? "element-missing"}`);
  } else {
    failures.push(`MOTION: reduced-motion 下 busy 环仍在动 ${motion.anim}`);
  }

  // ---- 5. 截图（1440×900 视口 + 全页） ----
  const viewportShot = resolve(screensDir, "gallery-viewport-1440x900.png");
  const fullShot = resolve(screensDir, "gallery-full.png");
  await page.screenshot({ path: viewportShot, type: "png" });
  await page.screenshot({ path: fullShot, type: "png", fullPage: true });

  // ---- 6. 报告 ----
  const report = {
    run: "UI-FOUNDATION-1 render smoke",
    date: new Date().toISOString(),
    url: galleryUrl,
    viewport: "1440x900@1",
    reducedMotion: "reduce",
    externalRequestCount: externalRequests.length,
    externalRequests,
    dom,
    textContrastMin: scan.textMin,
    graphicChecks: scan.graphics.length,
    graphicMinRatio: Math.min(...scan.graphics.map((g) => g.ratio)),
    clipIssues: scan.clipped,
    docScrollWidth: scan.docScrollWidth,
    fontTooSmall: scan.tooSmall,
    failures,
    notes,
    screenshots: [viewportShot, fullShot],
    result: failures.length === 0 ? "PASS" : "FAIL",
  };
  const reportPath = resolve(outDir, "render-smoke-report.json");
  writeFileSync(reportPath, JSON.stringify(report, null, 2), "utf-8");

  // ---- 7. UI-LAUNCHER-1（345）：launcher fixture 页渲染烟测 ----
  // gallery.html?surface=launcher 全离线 fixture 渲染（零真实端点请求）；
  // live surface 的真实端点消费由 ?surface=launcher 主面承担（端到端待 343/348）。
  const launcherFailures = [];
  const launcherNotes = [];
  mkdirSync(launcherScreensDir, { recursive: true });
  const launcherUrl = `${galleryUrl}?surface=launcher`;
  const launcherRequests = [];
  const onLauncherRequest = (r) => launcherRequests.push(r.url());
  page.on("request", onLauncherRequest);
  await page.goto(launcherUrl, { waitUntil: "networkidle" });
  await page.waitForSelector("[data-launcher-root]");
  await page.waitForTimeout(200);
  const launcherOrigin = new URL(page.url()).origin;
  const launcherExternalRequests = launcherRequests.filter((u) => {
    if (u.startsWith("data:") || u.startsWith("blob:")) return false;
    return !u.startsWith(launcherOrigin);
  });
  page.off("request", onLauncherRequest);
  launcherNotes.push(`launcherUrl=${launcherUrl}`);

  const ldom = await page.evaluate(() => {
    const $ = (sel) => document.querySelector(sel);
    const $$ = (sel) => [...document.querySelectorAll(sel)];
    // 就绪全尺寸样例 = DOM 中首个 launcher root；样例计数只在该 root 内取，
    // 不与加载/错误/空态紧凑样例混计。
    const rootEl = $("[data-launcher-root]");
    const within = (sel) => [...(rootEl?.querySelectorAll(sel) ?? [])];
    const titlebar = $("[data-launcher-titlebar]");
    const recents = within("[data-launcher-recent]");
    const directive = within("[data-launcher-directive]")[0] ?? null;
    const suggestions = within("[data-launcher-suggestion]");
    const out = {};
    out.rootState = rootEl?.getAttribute("data-launcher-state") ?? "(missing)";
    out.fixtureNotices = {
      page: !!$('[data-fixture="notice"]'),
      surface: !!$('[data-fixture="launcher-surface"]'),
    };
    out.titlebarPresent = !!titlebar;
    out.titlebarBrandText =
      titlebar?.querySelector(".v5-launcher-titlebar-brand")?.textContent?.trim() ?? "";
    out.titlebarFullText = titlebar?.textContent?.trim() ?? "";
    out.titlebarVitUse = $$("[data-launcher-titlebar] use[href='#i-vit']").length;
    out.titlebarProjectBadge = $$("[data-launcher-titlebar] [data-project-badge]").length;
    out.drawerState = $("[data-launcher-drawer]")?.getAttribute("data-drawer-state") ?? "(missing)";
    out.recentsInDom = recents.length;
    out.declaredRecentsCount =
      $("[data-launcher-recents-count]")?.getAttribute("data-launcher-recents-count") ?? "";
    out.recentsLabeledFixture = recents.map((e) => (e.textContent ?? "").includes("fixture"));
    out.greetingPresent = !!$("[data-launcher-greeting]");
    out.threadPresent = !!$("[data-launcher-thread]");
    out.resultCards = within("[data-launcher-card]").length;
    out.directiveState = directive?.getAttribute("data-launcher-directive-state") ?? "(missing)";
    out.directiveHeadline = directive?.querySelector(".v5-launcher-directive-headline")?.textContent ?? "";
    out.directiveNote = directive?.querySelector(".v5-launcher-directive-note")?.textContent ?? "";
    out.suggestions = suggestions.length;
    out.inputPresent = !!$("[data-launcher-input] input");
    out.loadingSample = !!$('[data-launcher-sample="loading"] [data-drawer-state="loading"]');
    out.loadingText = $("[data-launcher-drawer-loading]")?.textContent ?? "";
    out.errorSample = !!$('[data-launcher-sample="error"] [data-launcher-agent-error]');
    out.errorRetryButtons = $$("[data-launcher-retry]").length;
    out.drawerErrorRetryButtons = $$("[data-launcher-drawer-retry]").length;
    out.emptySample = !!$('[data-launcher-sample="empty"] [data-drawer-state="empty"]');
    out.emptyText = $("[data-launcher-drawer-empty]")?.textContent ?? "";
    return out;
  });

  const launcherAssert = (name, ok, detail) => {
    if (!ok) launcherFailures.push(`LAUNCHER: ${name} — ${detail}`);
  };

  launcherAssert("surface root ready", ldom.rootState === "ready", ldom.rootState);
  launcherAssert(
    "fixture 标注（页面 + surface）",
    ldom.fixtureNotices.page && ldom.fixtureNotices.surface,
    JSON.stringify(ldom.fixtureNotices)
  );
  launcherAssert(
    "标题栏 = Vit 文本（V 品牌标/工程标不进标题栏）",
    ldom.titlebarPresent &&
      ldom.titlebarBrandText === "Vit" &&
      ldom.titlebarVitUse === 0 &&
      ldom.titlebarProjectBadge === 0,
    `brand=${ldom.titlebarBrandText} vitUse=${ldom.titlebarVitUse} badge=${ldom.titlebarProjectBadge}`
  );
  launcherAssert(
    "抽屉 ready + 真实 recent 数一致（声明数 = DOM 行数 = 3 fixture）",
    ldom.drawerState === "ready" &&
      ldom.recentsInDom === 3 &&
      ldom.declaredRecentsCount === String(ldom.recentsInDom),
    `state=${ldom.drawerState} dom=${ldom.recentsInDom} declared=${ldom.declaredRecentsCount}`
  );
  launcherAssert(
    "recent 行全部带 fixture 标注（不冒充真实工程）",
    ldom.recentsLabeledFixture.length === 3 && ldom.recentsLabeledFixture.every(Boolean),
    JSON.stringify(ldom.recentsLabeledFixture)
  );
  launcherAssert("问候语在卷", ldom.greetingPresent, "缺 data-launcher-greeting");
  launcherAssert("会话线程在卷", ldom.threadPresent, "缺 data-launcher-thread");
  launcherAssert("结果卡数据展示（3 张 fixture 卡）", ldom.resultCards === 3, `cards=${ldom.resultCards}`);
  launcherAssert(
    "directive = 待执行占位（准备打开…，不虚报已打开）",
    ldom.directiveState === "pending" &&
      ldom.directiveHeadline.startsWith("准备打开") &&
      ldom.directiveNote.includes("不执行"),
    `state=${ldom.directiveState} headline=${ldom.directiveHeadline} note=${ldom.directiveNote}`
  );
  launcherAssert("上下文建议 3–4 个", ldom.suggestions >= 3 && ldom.suggestions <= 4, `count=${ldom.suggestions}`);
  launcherAssert("输入框在卷", ldom.inputPresent, "缺 data-launcher-input input");
  launcherAssert("加载态样例", ldom.loadingSample && ldom.loadingText.includes("正在读取"), ldom.loadingText);
  launcherAssert(
    "错误态样例（agent 不可用 banner + 双重试按钮）",
    ldom.errorSample && ldom.errorRetryButtons >= 1 && ldom.drawerErrorRetryButtons >= 1,
    `agent=${ldom.errorSample} retry=${ldom.errorRetryButtons} drawerRetry=${ldom.drawerErrorRetryButtons}`
  );
  launcherAssert("空态样例", ldom.emptySample && ldom.emptyText.includes("还没有"), ldom.emptyText);

  const lscan = await scanFn();
  const launcherContrastAssert = (name, ok, detail) => {
    if (!ok) launcherFailures.push(`LAUNCHER-CONTRAST: ${name} — ${detail}`);
  };
  launcherContrastAssert("正文 ≥4.5:1", lscan.text.length === 0, JSON.stringify(lscan.text.slice(0, 6)));
  launcherContrastAssert(
    "图形 ≥3:1",
    !lscan.graphicsFail || lscan.graphicsFail.length === 0,
    JSON.stringify((lscan.graphicsFail ?? []).slice(0, 6))
  );
  launcherContrastAssert("最小字号 ≥13px", lscan.tooSmall.length === 0, JSON.stringify(lscan.tooSmall.slice(0, 6)));
  if (lscan.clipped.length > 0)
    launcherFailures.push(`LAUNCHER-CLIP: 裁切/溢出 ${JSON.stringify(lscan.clipped.slice(0, 6))}`);
  if (lscan.docScrollWidth > 1440)
    launcherFailures.push(`LAUNCHER-OVERFLOW: 文档横向溢出 scrollWidth=${lscan.docScrollWidth} > 1440`);
  if (launcherExternalRequests.length > 0)
    launcherFailures.push(`LAUNCHER-NETWORK: fixture 页出现外部请求 ${JSON.stringify(launcherExternalRequests.slice(0, 6))}`);

  // launcher 页截图（1440×900 视口 + 全页）→ 卡目录工件
  const launcherViewportShot = resolve(launcherScreensDir, "launcher-viewport-1440x900.png");
  const launcherFullShot = resolve(launcherScreensDir, "launcher-full.png");
  await page.screenshot({ path: launcherViewportShot, type: "png" });
  await page.screenshot({ path: launcherFullShot, type: "png", fullPage: true });

  const launcherReport = {
    run: "UI-LAUNCHER-1 launcher render smoke",
    date: new Date().toISOString(),
    url: launcherUrl,
    viewport: "1440x900@1",
    reducedMotion: "reduce",
    externalRequestCount: launcherExternalRequests.length,
    dom: ldom,
    textContrastMin: lscan.textMin,
    graphicChecks: lscan.graphics.length,
    graphicMinRatio: Math.min(...lscan.graphics.map((g) => g.ratio)),
    clipIssues: lscan.clipped,
    docScrollWidth: lscan.docScrollWidth,
    fontTooSmall: lscan.tooSmall,
    failures: launcherFailures,
    notes: launcherNotes,
    screenshots: [launcherViewportShot, launcherFullShot],
    foundationReport: reportPath,
    result: launcherFailures.length === 0 ? "PASS" : "FAIL",
  };
  const launcherReportPath = resolve(launcherOutDir, "render-smoke-launcher-report.json");
  writeFileSync(launcherReportPath, JSON.stringify(launcherReport, null, 2), "utf-8");

  await browser.close();
  await server.close();

  console.log(`[test:render] url=${galleryUrl}`);
  console.log(`[test:render] symbols=${dom.symbolCount} matrixOK=${SYMBOL_IDS.every((i) => Object.values(dom.matrix[i] ?? {}).every((n) => n >= 1))}`);
  console.log(`[test:render] textMin=${report.textContrastMin?.ratio} graphicMin=${report.graphicMinRatio} graphics=${report.graphicChecks}`);
  console.log(`[test:render] external=${externalRequests.length} clip=${scan.clipped.length} tooSmall=${scan.tooSmall.length}`);
  console.log(`[test:render] report=${reportPath}`);
  console.log(`[test:render] RESULT=${report.result}`);
  console.log(`[test:render] launcherReport=${launcherReportPath}`);
  console.log(`[test:render] LAUNCHER_RESULT=${launcherReport.result}`);
  const allFailures = [...failures, ...launcherFailures];
  if (allFailures.length > 0) {
    for (const f of allFailures) console.error(`[test:render] FAIL: ${f}`);
    process.exit(1);
  }
  process.exit(0);
}

main().catch((err) => {
  console.error("[test:render] ABORT:", err);
  process.exit(1);
});
