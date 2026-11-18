// vitshell_combo_cdp.mjs — SHELL-WIN-COMBO-1 (414) presentation assertions.
//
// Reuses the pinned Playwright under agent/webui and connects over the VitShell
// WebView2 CDP port used by vitshell_open_chain_smoke.ps1. This script only
// reads the rendered DOM after the real open action: it does not click, focus,
// type, or change any host/window state.
//
// Phase:
//   probe-presentation  read the real launch feedback produced by the 409/410/
//                       411/412 chain: request/seq/terminal/progress, rendered
//                       loading steps, completed card, native-focus hint.
//
// Verdict JSON: { phase, ok, checks, observed, error }. Exit 0 whenever the
// page was read (check failures are reported in ok=false); exit 1 on connect
// or evaluation failure. The PowerShell companion owns the product verdict.

import { createRequire } from "node:module";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

function arg(name, fallback = "") {
  const idx = process.argv.indexOf("--" + name);
  if (idx >= 0 && idx + 1 < process.argv.length) return process.argv[idx + 1];
  return fallback;
}

const cdpPort = arg("cdp-port", "9333");
const phase = arg("phase", "probe-presentation");
const outPath = arg("out", "");
const timeoutMs = Number(arg("timeout-sec", "30")) * 1000;
const webuiDir = arg("webui-dir", join(process.cwd(), "agent/webui"));
const requireFromWebui = createRequire(join(webuiDir, "package.json"));
const { chromium } = requireFromWebui("playwright");

function finish(verdict, code) {
  if (outPath) writeFileSync(outPath, JSON.stringify(verdict, null, 2));
  console.log((verdict.ok ? "CDP_OK " : "CDP_FAIL ") + JSON.stringify(verdict));
  process.exit(code);
}

if (phase !== "probe-presentation") {
  finish({ phase, ok: false, error: "unknown phase: " + phase }, 1);
}

const browser = await chromium
  .connectOverCDP("http://127.0.0.1:" + cdpPort, { timeout: 20000 })
  .catch((error) => {
    finish({ phase, ok: false, error: "connectOverCDP failed: " + error.message }, 1);
  });

try {
  let page = null;
  for (const context of browser.contexts()) {
    const pages = context.pages();
    if (pages.length > 0) { page = pages[0]; break; }
  }
  if (!page) finish({ phase, ok: false, error: "no page over CDP" }, 1);

  const feedback = page.locator("[data-window-launch-feedback]:visible").first();
  await feedback.waitFor({ state: "visible", timeout: 30000 });

  let head = null;
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    head = await feedback.evaluate((el) => ({
      request: el.getAttribute("data-window-launch-request") || "",
      seq: Number(el.getAttribute("data-window-launch-seq") || "0"),
      terminal: el.getAttribute("data-window-launch-terminal") || "",
      progress: el.getAttribute("data-window-launch-progress") || "",
    }));
    if (head.terminal && head.terminal !== "pending") break;
    await new Promise((resolve) => setTimeout(resolve, 200));
  }

  const observed = await page.evaluate(() => {
    const f = document.querySelector("[data-window-launch-feedback]");
    const surface = document.querySelector("[data-project-root]") || document.querySelector("[data-launcher-root]");
    if (!f) return null;
    return {
      nativeFocus: surface ? (surface.getAttribute("data-native-focus") || "unknown") : "unknown",
      hintCount: document.querySelectorAll("[data-native-focus-hint]").length,
      stepCount: f.querySelectorAll("[data-loading-step-status]").length,
      completedCount: f.querySelectorAll('[data-loading-phase="completed"]').length,
      loadingPhase: (f.querySelector("[data-loading-phase]") || {}).getAttribute
        ? (f.querySelector("[data-loading-phase]").getAttribute("data-loading-phase") || "")
        : "",
      limitedCount: f.querySelectorAll("[data-window-launch-limited]").length,
      text: (f.innerText || "").slice(0, 800),
      url: location.href,
      sentinel: window.__vit_no_reload ?? null,
    };
  });

  if (!head || !observed) {
    finish({ phase, ok: false, error: "feedback node missing or unreadable", head, observed }, 1);
  }

  const forbidden = /已读取工程文件|已恢复轨道|连接音频设备|\d+%/.test(observed.text);
  const checks = {
    request_present: head.request.length > 0,
    seq_positive: Number.isFinite(head.seq) && head.seq >= 1,
    progress_unknown: head.progress === "unknown",
    terminal_ready: head.terminal === "ready",
    terminal_is_not_limited_fallback: observed.limitedCount === 0,
    completed_card: observed.completedCount === 1,
    real_steps_rendered: observed.stepCount >= 1 && observed.stepCount <= 8,
    no_fake_progress_text: !forbidden,
    shell_condition_no_daw_focus_claim: observed.nativeFocus !== "daw",
    shell_condition_no_focus_hint: observed.hintCount === 0,
    native_focus_observed: observed.nativeFocus === "shell" || observed.nativeFocus === "unknown",
    same_window_no_reload: observed.sentinel !== null,
  };
  const failed = Object.entries(checks).filter(([, value]) => !value).map(([name]) => name);
  const ok = failed.length === 0;
  const verdict = {
    phase,
    ok,
    error: ok ? "" : "failed checks: " + failed.join(","),
    checks,
    observed: { ...observed, request: head.request, seq: head.seq, terminal: head.terminal, progress: head.progress },
  };
  if (!ok) {
    verdict.coverage_gap = head.terminal !== "ready"
      ? "terminal=" + head.terminal + " (workspace_ready not delivered in this run)"
      : "";
  }
  finish(verdict, 0);
} catch (error) {
  finish({ phase, ok: false, error: String(error && error.message ? error.message : error) }, 1);
}
