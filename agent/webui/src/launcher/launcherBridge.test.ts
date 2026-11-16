/*
 * UI-BRIDGE-1 launcher 桥客户端单元测试（node 环境，stub window.chrome.webview）。
 *
 * fixture 纪律：工程路径均为测试 fixture；断言面=通道/type/request_id/回执过滤，
 * 不含真实工程数据。
 */

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createLauncherBridgeRequestID,
  isLauncherBridgeAvailable,
  LAUNCHER_BRIDGE_CHANNEL,
  LAUNCHER_BRIDGE_RESULT_TYPE,
  LAUNCHER_GOVERNANCE_REQUIRED_TYPE,
  postLauncherChooseFile,
  postLauncherDirective,
  postLauncherGovernanceChoice,
  postLauncherResultAck,
  postLauncherSurfaceReady,
  subscribeGovernanceRequests,
  subscribeLauncherResults,
  type LauncherBridgeResult
} from "./launcherBridge";

type Posted = Record<string, unknown>;
type Listener = (event: MessageEvent) => void;

function stubWebview(): { posted: Posted[]; listeners: Listener[] } {
  const posted: Posted[] = [];
  const listeners: Listener[] = [];
  vi.stubGlobal("window", {
    chrome: {
      webview: {
        postMessage: (message: unknown) => {
          posted.push(message as Posted);
        },
        addEventListener: (_type: string, listener: Listener) => {
          listeners.push(listener);
        },
        removeEventListener: (_type: string, listener: Listener) => {
          const index = listeners.indexOf(listener);
          if (index >= 0) listeners.splice(index, 1);
        }
      }
    }
  });
  return { posted, listeners };
}

describe("launcherBridge 通道面", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("无 chrome.webview → 桥不可用且 post 全部返回 false（诚实降级不伪造）", () => {
    vi.stubGlobal("window", {});
    expect(isLauncherBridgeAvailable()).toBe(false);
    expect(postLauncherDirective({ action: "open_project" }, "req_x")).toBe(false);
    expect(postLauncherSurfaceReady()).toBe(false);
    expect(postLauncherChooseFile("req_y")).toBe(false);
    const unsub = subscribeLauncherResults(() => {});
    expect(typeof unsub).toBe("function");
    unsub(); // 空订阅安全
  });

  it("postLauncherDirective 走 ask_vit_launcher 通道并携带 type/request_id/directive", () => {
    const { posted } = stubWebview();
    expect(isLauncherBridgeAvailable()).toBe(true);
    expect(postLauncherDirective({ action: "open_project", file_path: "D:/fixtures/甲.vit" }, "req_42")).toBe(true);
    expect(posted).toHaveLength(1);
    expect(posted[0].channel).toBe(LAUNCHER_BRIDGE_CHANNEL);
    expect(posted[0].type).toBe("launcher_directive");
    expect(posted[0].request_id).toBe("req_42");
    expect(posted[0].directive).toEqual({ action: "open_project", file_path: "D:/fixtures/甲.vit" });
  });

  it("surface_ready / choose_file / result_ack 的 type 面", () => {
    const { posted } = stubWebview();
    expect(postLauncherSurfaceReady()).toBe(true);
    expect(posted[0].type).toBe("launcher_surface_ready");
    expect(posted[0].surface).toBe("launcher");
    expect(postLauncherChooseFile("req_f")).toBe(true);
    expect(posted[1].type).toBe("launcher_choose_file");
    expect(posted[1].request_id).toBe("req_f");
    expect(postLauncherResultAck("req_f", "opened")).toBe(true);
    expect(posted[2].type).toBe("launcher_result_ack");
    expect(posted[2].status).toBe("opened");
  });

  it("subscribeLauncherResults 只转发 vit_launcher_result 且解订干净", () => {
    const { listeners } = stubWebview();
    const seen: LauncherBridgeResult[] = [];
    const unsub = subscribeLauncherResults((result) => seen.push(result));
    expect(listeners).toHaveLength(1);
    listeners[0]({ data: { type: "vit_os_notify" } } as MessageEvent);
    listeners[0]({ data: null } as MessageEvent);
    listeners[0]({ data: "string payload" } as MessageEvent);
    listeners[0]({
      data: { type: LAUNCHER_BRIDGE_RESULT_TYPE, request_id: "r1", status: "opened", action: "open_project", message: "已进入《甲》" }
    } as MessageEvent);
    expect(seen).toHaveLength(1);
    expect(seen[0].request_id).toBe("r1");
    expect(seen[0].status).toBe("opened");
    expect(seen[0].message).toBe("已进入《甲》");
    unsub();
    expect(listeners).toHaveLength(0);
  });

  it("request_id 形态与区分度", () => {
    expect(createLauncherBridgeRequestID(1700000000000)).toMatch(/^webui_launcher_br_/);
    expect(createLauncherBridgeRequestID(1)).not.toBe(createLauncherBridgeRequestID(2));
  });
});

// ── SHELL-2：页内治理桥面（vit_governance_required 订阅 + launcher_governance_choice 回投）──

describe("launcherBridge 页内治理面（SHELL-2）", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("postLauncherGovernanceChoice 走 ask_vit_launcher 通道并携带 save/switch/cancel", () => {
    const { posted } = stubWebview();
    expect(postLauncherGovernanceChoice("req_g1", "save")).toBe(true);
    expect(postLauncherGovernanceChoice("req_g1", "switch")).toBe(true);
    expect(postLauncherGovernanceChoice("req_g1", "cancel")).toBe(true);
    expect(posted).toHaveLength(3);
    for (const item of posted) {
      expect(item.channel).toBe(LAUNCHER_BRIDGE_CHANNEL);
      expect(item.type).toBe("launcher_governance_choice");
    }
    expect(posted[0].request_id).toBe("req_g1");
    expect(posted[0].choice).toBe("save");
    expect(posted[1].choice).toBe("switch");
    expect(posted[2].choice).toBe("cancel");
  });

  it("subscribeGovernanceRequests 只转发 vit_governance_required 且 request_id 非空", () => {
    const { listeners } = stubWebview();
    const seen: Array<{ request_id: string; action: string; current_project: string }> = [];
    const unsub = subscribeGovernanceRequests((request) => seen.push(request));
    expect(listeners).toHaveLength(1);
    listeners[0]({ data: { type: "vit_launcher_result", request_id: "x" } } as MessageEvent);
    listeners[0]({ data: { type: LAUNCHER_GOVERNANCE_REQUIRED_TYPE } } as MessageEvent); // 缺 request_id → 过滤
    listeners[0]({
      data: { type: LAUNCHER_GOVERNANCE_REQUIRED_TYPE, request_id: "req_g9", action: "open_project", current_project: "D:/fixtures/甲.vit" }
    } as MessageEvent);
    expect(seen).toHaveLength(1);
    expect(seen[0].request_id).toBe("req_g9");
    expect(seen[0].action).toBe("open_project");
    expect(seen[0].current_project).toBe("D:/fixtures/甲.vit");
    unsub();
    expect(listeners).toHaveLength(0);
  });

  it("无 chrome.webview → 治理订阅空转且回投返回 false（诚实降级）", () => {
    vi.stubGlobal("window", {});
    expect(postLauncherGovernanceChoice("req_g2", "switch")).toBe(false);
    const unsub = subscribeGovernanceRequests(() => {});
    expect(typeof unsub).toBe("function");
    unsub();
  });
});
