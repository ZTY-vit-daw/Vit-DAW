// 手动诊断：浏览器侧 score 面板装载失败原因（live stack 7878）。
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chromium } = require("C:/Users/timoz/wt_score1/agent/webui/node_modules/playwright/index.mjs");

const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
const page = await context.newPage();
page.on("console", (msg) => console.log("[browser console]", msg.type(), msg.text().slice(0, 200)));
page.on("response", async (res) => {
  if (res.url().includes("/agent/invoke") || res.url().includes("/agent/ui/state")) {
    let body = "";
    try { body = (await res.text()).slice(0, 300); } catch {}
    console.log("[net]", res.status(), res.request().method(), res.url().slice(0, 80), body);
  }
});
page.on("pageerror", (err) => console.log("[pageerror]", String(err).slice(0, 300)));
await page.goto("http://127.0.0.1:7878/app/", { waitUntil: "domcontentloaded" });
await page.waitForTimeout(2500);
await page.click('[data-workbench-tab="score"]');
await page.waitForSelector("[data-score-root]", { timeout: 15000 });
for (let i = 0; i < 6; i++) {
  await page.waitForTimeout(1000);
  const status = await page.getAttribute("[data-score-root]", "data-score-status");
  const model = await page.getAttribute("[data-score-root]", "data-score-model");
  console.log("[t+" + (i + 1) + "s] status:", status, "| notes:", model ? JSON.parse(model).notes.length : "?");
}
await browser.close();
