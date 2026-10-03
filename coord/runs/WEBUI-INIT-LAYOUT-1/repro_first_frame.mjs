#!/usr/bin/env node
// WEBUI-INIT-LAYOUT-1 reproduction probe (PC execution side, 2026-10-03).
// Serves the built bundle statically (layout-only repro: no agent needed for
// the shell) and samples the FIRST mounted frame's layout facts at several
// viewports, plus a full-page screenshot per viewport.
import { createRequire } from "node:module";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";

const require = createRequire("D:/Vit_DAW/coord/runs/WEBUI-INIT-LAYOUT-1/repro_first_frame.mjs");
const PW = "C:/Users/timoz/AppData/Local/npm-cache/_npx/31e32ef8478fbf80/node_modules/playwright-core";
const chromium = require(PW).chromium;

const dist = "D:/Vit_DAW/agent/webui/dist";
const outDir = "D:/Vit_DAW/coord/runs/WEBUI-INIT-LAYOUT-1";
const MIME = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".json": "application/json" };

const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://localhost");
    let path = normalize(decodeURIComponent(url.pathname)).replace(/^([A-Za-z]:)?[\\/]+/, "");
    if (path === "app" || path.startsWith("app/") || path.startsWith("app\\")) path = path.slice(4) || "index.html";
    let file = join(dist, path);
    if (!file.startsWith(normalize(dist))) file = join(dist, "index.html");
    let body;
    try {
      body = await readFile(file);
    } catch {
      file = join(dist, "index.html");
      body = await readFile(file);
    }
    res.writeHead(200, { "content-type": MIME[extname(file)] || "application/octet-stream" });
    res.end(body);
  } catch (err) {
    res.writeHead(500);
    res.end(String(err));
  }
});
await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
const base = "http://127.0.0.1:" + server.address().port + "/";
console.log("[repro] serving", dist, "at", base);

const PROBE = () => {
  const rect = (sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: +r.top.toFixed(1), left: +r.left.toFixed(1), bottom: +r.bottom.toFixed(1), right: +r.right.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) };
  };
  const shell = document.querySelector(".app-shell");
  const shellCS = shell ? getComputedStyle(shell) : null;
  const ws = document.querySelector(".main-workspace");
  const wsCS = ws ? getComputedStyle(ws) : null;
  return {
    url: location.href,
    innerW: window.innerWidth,
    innerH: window.innerHeight,
    dpr: window.devicePixelRatio,
    shellRows: shellCS ? shellCS.gridTemplateRows : null,
    shellDisplay: shellCS ? shellCS.display : null,
    shellRect: rect(".app-shell"),
    workspaceGridRow: wsCS ? wsCS.gridRowStart + "/" + wsCS.gridRowEnd : null,
    workspaceRect: rect(".main-workspace"),
    sidebarRect: rect(".session-sidebar"),
    railRect: rect(".workbench-collapsed"),
    workbenchPanelRect: rect(".workbench-panel"),
    conversationPanelRect: rect(".conversation-panel"),
    composerRect: rect(".composer"),
    topStatusRect: rect(".top-status"),
    workspaceGridCls: document.querySelector(".workspace-grid")?.className || ""
  };
};

let browser = null;
for (const opts of [{ headless: true }, { channel: "msedge", headless: true }, { channel: "chrome", headless: true }]) {
  try {
    browser = await chromium.launch(opts);
    console.log("[repro] browser:", opts.channel || "bundled");
    break;
  } catch (err) {
    console.log("[repro] launch failed:", opts.channel || "bundled", String(err).split("\n")[0]);
  }
}
if (!browser) throw new Error("no browser");
const viewports = [
  { name: "user-derived-1089x578@1.75", width: 1089, height: 578, dsf: 1.75 },
  { name: "150pct-1270x674@1.5", width: 1270, height: 674, dsf: 1.5 },
  { name: "smoke-default-1600x900@1", width: 1600, height: 900, dsf: 1 }
];
for (const vp of viewports) {
  const ctx = await browser.newContext({ viewport: { width: vp.width, height: vp.height }, deviceScaleFactor: vp.dsf });
  const page = await ctx.newPage();
  await page.goto(base, { waitUntil: "domcontentloaded" });
  await page.waitForSelector(".app-shell", { timeout: 15000 });
  const sample = await page.evaluate(PROBE);
  await page.screenshot({ path: join(outDir, `repro_${vp.name.replace(/[^a-z0-9.-]+/gi, "_")}.png`) });
  console.log("\n=== viewport", vp.name, "==="); 
  console.log(JSON.stringify(sample, null, 1));
  await ctx.close();
}
await browser.close();
server.close();
console.log("\n[repro] done");
