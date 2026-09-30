#!/usr/bin/env node
// O1c mini repro: boot -> wait scope -> send long reply -> reload -> sample + full storage dump.
import { createRequire } from "node:module";
import { writeFileSync } from "node:fs";

const require = createRequire(import.meta.url);
const here = "D:/Vit_DAW_worktrees/opt-observe-output-1/scripts";
const agentBase = "http://127.0.0.1:7899";
const conversationId = "webui_o1mini";
const outDir = "D:/Vit_DAW_worktrees/opt-observe-output-1/artifacts/o1debug";

let pw;
for (const cand of [
  "D:/Vit_DAW_worktrees/opt-observe-output-1/agent/webui/node_modules/playwright-core",
  "D:/Vit_DAW_worktrees/opt-observe-output-1/agent/webui/node_modules/playwright"
]) {
  try { pw = require(cand); break; } catch {}
}
if (!pw) {
  const { execSync } = await import("node:child_process");
  const npxRoots = [process.env.LOCALAPPDATA + "\\npm-cache\\_npx"];
  const fs = await import("node:fs");
  outer: for (const root of npxRoots) {
    if (!fs.existsSync(root)) continue;
    for (const dir of fs.readdirSync(root)) {
      const p = root + "\\" + dir + "\\node_modules\\playwright-core";
      if (fs.existsSync(p + "\\package.json")) { pw = require(p); break outer; }
    }
  }
}
if (!pw) throw new Error("no playwright-core found");
const { chromium } = pw;

const lead = "OBSLEAD 观察结论：Track 2 主唱在 2.1kHz 附近有约 3.2dB 的峰值堆积。";
const restLines = [];
for (let i = 1; i <= 16; i++) restLines.push("证据条目 OBSREST" + i + "：第 " + i + " 频段证据细节。");
restLines.push("OBSRESTFINAL 证据尾行：折叠区最后一行。");
const longReply = {
  status: "ok", conversation_id: conversationId, reply: lead + "\n\n" + restLines.join("\n"),
  needs_confirmation: false, goal_status: "completed",
  turn_id: "run_o1mini", run_id: "run_o1mini", goal_id: "run_o1mini", commands: []
};

const probe = (marker) => {
  const rows = Array.from(document.querySelectorAll(".message-row.assistant"));
  const row = rows.find((el) => (el.textContent || "").indexOf(marker) >= 0) || null;
  if (!row) return null;
  const details = row.querySelector(".observe-output-details");
  return {
    layered: Boolean(details),
    open: details ? details.hasAttribute("open") : null,
    visible: (row.innerText || "").replace(/\s+/g, " ").trim().slice(0, 200)
  };
};
const dumpStorage = () => {
  const out = {};
  for (const key of Object.keys(window.localStorage)) out[key] = window.localStorage.getItem(key) || "";
  return out;
};

const browser = await chromium.launch({ channel: "msedge", headless: true });
const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
await context.route("**/agent/events*", async (route) => {
  await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body: JSON.stringify({ status: "ok", events: [], next_seq: 0 }) });
});
await context.route("**/agent/chat*", async (route) => {
  await route.fulfill({ status: 200, contentType: "application/json; charset=utf-8", body: JSON.stringify(longReply) });
});
const page = await context.newPage();
const errors = [];
page.on("pageerror", (e) => errors.push("pageerror: " + String(e).slice(0, 300)));

await page.goto(agentBase + "/app/?conversation_id=" + encodeURIComponent(conversationId), { waitUntil: "domcontentloaded" });
await page.waitForTimeout(3500); // uiState + scope materialization
const composer = page.locator(".composer textarea").first();
await composer.waitFor({ state: "visible", timeout: 15000 });
await composer.fill("Track 2 的 2.1kHz 峰值从哪来？");
await composer.press("Enter");
await page.locator(".observe-output-details").first().waitFor({ timeout: 15000 }).catch(() => {});
await page.waitForTimeout(500);
const before = await page.evaluate(probe, "OBSRESTFINAL");
console.log("BEFORE RELOAD:", JSON.stringify(before));

const storageBefore = await page.evaluate(dumpStorage);
writeFileSync(outDir + "/mini-storage-before.json", JSON.stringify(storageBefore, null, 2), "utf-8");

await page.reload({ waitUntil: "domcontentloaded" });
const samples = [];
for (let i = 0; i < 12; i++) {
  await page.waitForTimeout(2000);
  samples.push(await page.evaluate(probe, "OBSRESTFINAL"));
}
console.log("AFTER RELOAD SAMPLES:", samples.map((s) => (s ? "layered=" + s.layered + " open=" + s.open : "null")).join(" | "));

const storageAfter = await page.evaluate(dumpStorage);
const flow = await page.evaluate(() => Array.from(document.querySelectorAll(".message-row")).map((row) => ({
  cls: row.className, text: (row.textContent || "").replace(/\s+/g, " ").trim().slice(0, 80)
})));
console.log("FLOW AFTER RELOAD:", JSON.stringify(flow, null, 1).slice(0, 2000));
console.log("PAGE ERRORS:", JSON.stringify(errors));
writeFileSync(outDir + "/mini-storage-after.json", JSON.stringify(storageAfter, null, 2), "utf-8");

// bucket analysis
for (const [name, data] of [["before", storageBefore], ["after", storageAfter]]) {
  for (const [k, v] of Object.entries(data)) {
    if (k.indexOf("conversation_messages") < 0) continue;
    try {
      const b = JSON.parse(v);
      const msgs = b.messages || [];
      console.log(name, "bucket:", k.slice(0, 80), "n=", msgs.length,
        "hasOBS=", msgs.some((m) => (m.content || "").indexOf("OBSRESTFINAL") >= 0),
        "saved_at=", b.saved_at);
    } catch (e) { console.log(name, "bucket UNPARSEABLE:", k.slice(0, 80)); }
  }
}
await browser.close();
