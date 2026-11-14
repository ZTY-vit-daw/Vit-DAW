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
  postLauncherChooseFile,
  postLauncherDirective,
  postLauncherResultAck,
  postLauncherSurfaceReady,
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
