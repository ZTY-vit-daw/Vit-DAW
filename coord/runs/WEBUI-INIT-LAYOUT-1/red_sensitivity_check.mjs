#!/usr/bin/env node
// RED sensitivity spot-check for the L1 initial-layout group
// (WEBUI-INIT-LAYOUT-1): inject the pre-fix 3-track grid value into the FIXED
// bundle via addStyleTag (identical selector, later cascade position == the
// pre-fix computed value) and verify the group's two load-bearing thresholds
// actually go red: track count != child count, workspace bottom != viewport.
import { createRequire } from "node:module";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { writeFileSync } from "node:fs";
import { extname, join, normalize } from "node:path";

const require = createRequire("D:/Vit_DAW/coord/runs/WEBUI-INIT-LAYOUT-1/red_sensitivity_check.mjs");
const chromium = require("C:/Users/timoz/AppData/Local/npm-cache/_npx/31e32ef8478fbf80/node_modules/playwright-core").chromium;
const dist = "D:/Vit_DAW/agent/webui/dist";
const outDir = "D:/Vit_DAW/coord/runs/WEBUI-INIT-LAYOUT-1";
const MIME = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml" };
const server = createServer(async (req, res) => {
  let path = normalize(decodeURIComponent(new URL(req.url, "http://x").pathname)).replace(/^([A-Za-z]:)?[\\/]+/, "");
  if (path === "app" || path.startsWith("app/") || path.startsWith("app\\")) path = path.slice(4) || "index.html";
  let file = join(dist, path);
  let body;
  try { body = await readFile(file); } catch { file = join(dist, "index.html"); body = await readFile(file); }
  res.writeHead(200, { "content-type": MIME[extname(file)] || "application/octet-stream" });
  res.end(body);
});
await new Promise((ok) => server.listen(0, "127.0.0.1", ok));
const browser = await chromium.launch({ channel: "msedge", headless: true });
const ctx = await browser.newContext({ viewport: { width: 1600, height: 900 }, deviceScaleFactor: 1 });
const page = await ctx.newPage();
await page.goto("http://127.0.0.1:" + server.address().port + "/", { waitUntil: "domcontentloaded" });
await page.waitForSelector(".app-shell .composer");
await page.addStyleTag({ content: ".app-shell { grid-template-rows: auto auto minmax(0, 1fr) !important; }" });
const sample = await page.evaluate(() => {
  const rectOf = (el) => {
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: +r.top.toFixed(1), bottom: +r.bottom.toFixed(1), w: +r.width.toFixed(1), h: +r.height.toFixed(1) };
  };
  const shell = document.querySelector(".app-shell");
  const cs = shell ? getComputedStyle(shell) : null;
  return {
    innerW: window.innerWidth,
    innerH: window.innerHeight,
    shellRows: cs ? cs.gridTemplateRows : "",
    shellChildCount: shell ? shell.children.length : 0,
    workspace: rectOf(document.querySelector(".main-workspace")),
    composer: rectOf(document.querySelector(".composer"))
  };
});
await page.screenshot({ path: join(outDir, "red_oldrows_1600x900.png") });
const tracks = sample.shellRows.split(/\s+/).filter(Boolean);
const failures = [];
if (tracks.length !== sample.shellChildCount) {
  failures.push("L1 tracks: " + tracks.length + " track(s) [" + sample.shellRows + "] vs " + sample.shellChildCount + " children");
}
const slack = sample.innerH - sample.workspace.bottom;
if (Math.abs(slack) > 1.5) {
  failures.push("L1 fill: workspace bottom=" + sample.workspace.bottom + " vs innerH=" + sample.innerH + " (slack " + slack.toFixed(1) + "px)");
}
console.log("RED sample:", JSON.stringify(sample));
console.log("RED failures L1 would raise:", JSON.stringify(failures, null, 1));
writeFileSync(join(outDir, "red_oldrows_sample.json"), JSON.stringify({ sample, failures }, null, 2), "utf-8");
console.log(failures.length >= 2 ? "RED SENSITIVITY OK (group goes red on the pre-fix grid)" : "RED SENSITIVITY FAILED (group would not catch the regression)");
process.exitCode = failures.length >= 2 ? 0 : 1;
await browser.close();
server.close();
