/*
 * UI-ENTRY-1 入口 surface 桥单元测试（node 环境，stub window.chrome.webview）。
 *
 * 断言面：订阅只转发合法 vit_surface 载荷 / 解订干净 / ack 通道与 type 面 /
 * 桥不可用诚实降级。不含真实工程数据。
 */

import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ENTRY_SURFACE_ACK_CHANNEL,
  ENTRY_SURFACE_ACK_TYPE,
  ENTRY_SURFACE_MESSAGE_TYPE,
  postEntrySurfaceAck,
  subscribeEntrySurface
} from "./entrySurfaceBridge";

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

describe("entrySurfaceBridge 通道面", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("无 chrome.webview → 订阅空转且 ack 不发（诚实降级不伪造）", () => {
    vi.stubGlobal("window", {});
    const seen: string[] = [];
    const unsub = subscribeEntrySurface((surface) => seen.push(surface));
    expect(typeof unsub).toBe("function");
    unsub();
    expect(postEntrySurfaceAck("project")).toBe(false);
    expect(seen).toHaveLength(0);
  });

  it("订阅只转发 type=vit_surface 且 surface 非空的载荷，解订干净", () => {
    const { listeners } = stubWebview();
    const seen: string[] = [];
    const unsub = subscribeEntrySurface((surface) => seen.push(surface));
    expect(listeners).toHaveLength(1);
    listeners[0]({ data: { type: ENTRY_SURFACE_MESSAGE_TYPE, surface: "project" } } as MessageEvent);
    listeners[0]({ data: { type: "vit_launcher_result", surface: "project" } } as MessageEvent);
    listeners[0]({ data: { type: ENTRY_SURFACE_MESSAGE_TYPE, surface: "" } } as MessageEvent);
    listeners[0]({ data: { type: ENTRY_SURFACE_MESSAGE_TYPE, surface: 123 } } as MessageEvent);
    listeners[0]({ data: null } as MessageEvent);
    listeners[0]({ data: "string payload" } as MessageEvent);
    expect(seen).toEqual(["project"]);
    unsub();
    expect(listeners).toHaveLength(0);
  });

  it("postEntrySurfaceAck 走 ask_vit_entry 通道并携带 type/surface", () => {
    const { posted } = stubWebview();
    expect(postEntrySurfaceAck("project")).toBe(true);
    expect(posted).toHaveLength(1);
    expect(posted[0].channel).toBe(ENTRY_SURFACE_ACK_CHANNEL);
    expect(posted[0].type).toBe(ENTRY_SURFACE_ACK_TYPE);
    expect(posted[0].surface).toBe("project");
  });
});
