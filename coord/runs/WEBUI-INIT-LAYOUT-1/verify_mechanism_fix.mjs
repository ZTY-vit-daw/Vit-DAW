#!/usr/bin/env node
// Mechanism + fix pre-verification for WEBUI-INIT-LAYOUT-1.
// 1) RED mechanism: grow .message-stream content -> the auto grid row grows
//    with it (this is why "one conversation round later it looks expanded").
// 2) GREEN preview: override grid-template-rows to `auto minmax(0, 1fr)` ->
//    the workspace fills the viewport on the very first frame.
import { createRequire } from "node:module";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize } from "node:path";

const require = createRequire("D:/Vit_DAW/coord/runs/WEBUI-INIT-LAYOUT-1/verify_mechanism_fix.mjs");
const chromium = require("C:/Users/timoz/AppData/Local/npm-cache/_npx/31e32ef8478fbf80/node_modules/playwright-core").chromium;

const dist = "D:/Vit_DAW/agent/webui/dist";
const MIME = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml" };
const server = createServer(async (req, res) => {
  try {
    let path = normalize(decodeURIComponent(new URL(req.url, "http://x").pathname)).replace(/^([A-Za-z]:)?[\\/]+/, "");
    if (path.startsWith("app/") || path.startsWith("app\\") || path === "app") path = path.slice(4) || "index.html";
    let file = join(dist, path);
    let body;
    try { body = await readFile(file); } catch { file = join(dist, "index.html"); body = await readFile(file); }
    res.writeHead(200, { "content-type": MIME[extname(file)] || "application/octet-stream" });
    res.end(body);
  } catch (err) { res.writeHead(500); res.end(String(err)); }
});
await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
const base = "http://127.0.0.1:" + server.address().port + "/";

const browser = await chromium.launch({ channel: "msedge", headless: true });
const ctx = await browser.newContext({ viewport: { width: 1089, height: 578 }, deviceScaleFactor: 1.75 });
const page = await ctx.newPage();
await page.goto(base, { waitUntil: "domcontentloaded" });
await page.waitForSelector(".app-shell");

const measure = () => page.evaluate(() => {
  const rect = (sel) => {
    const el = document.querySelector(sel);
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: +r.top.toFixed(1), bottom: +r.bottom.toFixed(1), h: +r.height.toFixed(1) };
  };
  return {
    shellRows: getComputedStyle(document.querySelector(".app-shell")).gridTemplateRows,
    workspace: rect(".main-workspace"),
    composer: rect(".composer")
  };
});

const before = await measure();
console.log("first frame (defect):", JSON.stringify(before));

// Mechanism: inject tall content as a conversation round would (messages grow
// the stream's content height). The auto row must grow with it.
await page.evaluate(() => {
  const stream = document.querySelector(".message-stream");
  const filler = document.createElement("div");
  filler.style.height = "900px";
  filler.textContent = "(simulated conversation round content)";
  stream.appendChild(filler);
});
const grown = await measure();
console.log("after content growth :", JSON.stringify(grown));

// Fix preview: the workspace must fill the viewport regardless of content.
await page.evaluate(() => {
  document.querySelector(".app-shell").style.gridTemplateRows = "auto minmax(0, 1fr)";
});
const fixed = await measure();
console.log("with fix (fills?)    :", JSON.stringify(fixed));
console.log("\nviewport innerH=578; fixed.workspace.bottom =", fixed.workspace.bottom, "(should equal ~578)");
console.log("mechanism: workspace grew", (grown.workspace.h - before.workspace.h).toFixed(1), "px when content grew (auto row follows content)");

await browser.close();
server.close();
