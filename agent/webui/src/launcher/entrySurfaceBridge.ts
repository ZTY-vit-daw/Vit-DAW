/*
 * UI-ENTRY-1 入口 surface 切换桥（宿主 → React 单向 + React 取证回执）。
 *
 * 方向与通道（手测 v3 方案 B：启动态唯一门面=React 启动台）：
 * - 宿主 → React：host.post_web_message(JSON)，页面经 chrome.webview "message" 事件收到
 *   {type:"vit_surface", surface:"project"} —— 进工程展开 DAW 时宿主通知 React 切到
 *   project surface。**surface 切换走本桥（React 内部 setState），禁止整页导航**——
 *   343 不变量：同一 HWND 同一页面，navigate_count 不增。
 * - React → 宿主（取证回执）：window.chrome.webview.postMessage
 *   {channel:"ask_vit_entry", type:"vit_surface_ack", surface} —— React 真收并处理
 *   后回投，宿主日志留痕（AgentChatWindow.surface.ack 行）；烟测以该行为 React 侧
 *   在场证明，不以宿主单方 post 记录充当。
 *
 * 纪律：URL 初始 surface 语义保留（浏览器 dev 直开不变）；桥不可用=订阅空转+回执
 * 不发（诚实降级不伪造）；surface 值校验交给消费方白名单（本模块只透传非空字符串）。
 */

/** 宿主 → React surface 切换消息 type。 */
export const ENTRY_SURFACE_MESSAGE_TYPE = "vit_surface";
/** React → 宿主取证回执通道/type。 */
export const ENTRY_SURFACE_ACK_CHANNEL = "ask_vit_entry";
export const ENTRY_SURFACE_ACK_TYPE = "vit_surface_ack";

type BridgeWebview = {
  postMessage: (message: unknown) => void;
  addEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
  removeEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
};

function entryWebview(): BridgeWebview | null {
  const hostWindow = window as unknown as { chrome?: { webview?: BridgeWebview } };
  return hostWindow.chrome?.webview ?? null;
}

/**
 * 订阅宿主 surface 切换消息；只转发 type==vit_surface 且 surface 非空的载荷，
 * 返回解订函数。桥不可用=空订阅（listener 不挂）。
 */
export function subscribeEntrySurface(handler: (surface: string) => void): () => void {
  const webview = entryWebview();
  if (!webview?.addEventListener) {
    return () => {};
  }
  const listener = (event: MessageEvent) => {
    const data: unknown = event.data;
    if (!data || typeof data !== "object") {
      return;
    }
    const record = data as Record<string, unknown>;
    if (record.type !== ENTRY_SURFACE_MESSAGE_TYPE) {
      return;
    }
    const surface = typeof record.surface === "string" ? record.surface : "";
    if (!surface) {
      return;
    }
    handler(surface);
  };
  webview.addEventListener("message", listener);
  return () => {
    webview.removeEventListener?.("message", listener);
  };
}

/** 取证回执（React 真收并接受切换后调用；桥不可用=不发，诚实降级）。 */
export function postEntrySurfaceAck(surface: string): boolean {
  const webview = entryWebview();
  if (!webview?.postMessage) {
    return false;
  }
  try {
    webview.postMessage({
      channel: ENTRY_SURFACE_ACK_CHANNEL,
      type: ENTRY_SURFACE_ACK_TYPE,
      surface
    });
    return true;
  } catch {
    return false;
  }
}
