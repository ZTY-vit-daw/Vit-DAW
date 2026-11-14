/*
 * UI-BRIDGE-1 launcher 桥客户端（React → Godot 宿主）。
 *
 * 方向与通道（R0-CHAT-WINDOW-RECON-1 §Q4 + agent_chat_window.gd 转发面）：
 * - React → 宿主：window.chrome.webview.postMessage({channel:"ask_vit_launcher", ...})。
 *   宿主链：WebView2 WebMessageReceived → VitWebViewHost.web_message_received →
 *   AgentChatWindow._on_host_web_message → current_scene._on_agent_webui_web_message
 *   （start_page 阶段=start_page 桥 handler；dock 阶段=dock 降级 handler）。
 * - 宿主 → React：host.post_web_message(JSON)，页面经 chrome.webview "message"
 *   事件收到 {type:"vit_launcher_result", ...}。
 *
 * 纪律（卡面）：React 只发请求与呈现结果；不伪造「已打开/已应用」——一切终态
 * 以宿主 vit_launcher_result 回执为准；桥不可用给诚实文案（不静默）。
 * 烟测通道（launcher_result_ack）仅在 URL launcher_smoke=1 时发送（取证回执，
 * 生产零行为）。
 */

/** React → 宿主请求通道（卡面建议名）。 */
export const LAUNCHER_BRIDGE_CHANNEL = "ask_vit_launcher";
/** 宿主 → React 结果回执 type（post_web_message 载荷）。 */
export const LAUNCHER_BRIDGE_RESULT_TYPE = "vit_launcher_result";
/**
 * SHELL-2：宿主 → React 页内治理请求 type（agent/对话发起的工程切换，跟随发起方
 * 裁定——React 三键确认卡，不出 Godot 弹窗）。载荷：{type, request_id, action,
 * current_project}。
 */
export const LAUNCHER_GOVERNANCE_REQUIRED_TYPE = "vit_governance_required";

/** 桥请求 type 面。 */
export type LauncherBridgeRequestType =
  | "launcher_surface_ready"
  | "launcher_directive"
  | "launcher_choose_file"
  | "launcher_result_ack"
  | "launcher_governance_choice";

/** 宿主回执状态面（全 honest：opened 只能来自宿主回执）。 */
export type LauncherBridgeResultStatus =
  | "opened"
  | "failed"
  | "canceled"
  | "unsupported"
  | "invalid"
  | "dialog_shown";

/** 宿主回执（vit_launcher_result 载荷）。 */
export interface LauncherBridgeResult {
  type?: string;
  request_id?: string;
  status?: string;
  action?: string;
  project_name?: string;
  file_path?: string;
  scene?: string;
  governance?: string;
  message?: string;
}

type BridgeWebview = {
  postMessage: (message: unknown) => void;
  addEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
  removeEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
};

/** WebView2 注入面（浏览器 dev 无 chrome.webview → 桥不可用，诚实呈现）。 */
function launcherWebview(): BridgeWebview | null {
  const hostWindow = window as unknown as { chrome?: { webview?: BridgeWebview } };
  return hostWindow.chrome?.webview ?? null;
}

/** 桥可用性（存在 chrome.webview.postMessage 才算可用）。 */
export function isLauncherBridgeAvailable(): boolean {
  return Boolean(launcherWebview()?.postMessage);
}

function postLauncherBridge(payload: Record<string, unknown>): boolean {
  const webview = launcherWebview();
  if (!webview?.postMessage) {
    return false;
  }
  try {
    webview.postMessage({ channel: LAUNCHER_BRIDGE_CHANNEL, ...payload });
    return true;
  } catch {
    return false;
  }
}

/** request_id（webui_launcher_<ts36> 形态，与 launcher 会话 ID 同族）。 */
export function createLauncherBridgeRequestID(now: number = Date.now()): string {
  return `webui_launcher_br_${now.toString(36)}`;
}

/** 启动台就绪握手（宿主日志留痕：React launcher surface 已加载且桥在卷）。 */
export function postLauncherSurfaceReady(): boolean {
  return postLauncherBridge({ type: "launcher_surface_ready", surface: "launcher" });
}

/** directive 执行请求（open/new 一律经宿主治理与既有公共链，React 无执行权）。 */
export function postLauncherDirective(
  directive: Record<string, unknown>,
  requestID: string
): boolean {
  return postLauncherBridge({ type: "launcher_directive", request_id: requestID, directive });
}

/** 原生文件选择请求（宿主原生文件对话框；348 语义宿主版，SHELL-2 起归属宿主）。 */
export function postLauncherChooseFile(requestID: string): boolean {
  return postLauncherBridge({ type: "launcher_choose_file", request_id: requestID });
}

/** 页内治理三键选择面（SHELL-2：跟随发起方——agent/对话发起 → React 页内卡）。 */
export type LauncherGovernanceChoice = "save" | "switch" | "cancel";

/** 治理三键选择回投（宿主据 choice 执行保存/直行/取消）。 */
export function postLauncherGovernanceChoice(
  requestID: string,
  choice: LauncherGovernanceChoice
): boolean {
  return postLauncherBridge({
    type: "launcher_governance_choice",
    request_id: requestID,
    choice
  });
}

/** 宿主页内治理请求载荷（vit_governance_required）。 */
export interface LauncherGovernanceRequest {
  request_id: string;
  action: string;
  current_project: string;
}

/** 订阅宿主页内治理请求；返回解订函数。桥不可用=空订阅（listener 不挂）。 */
export function subscribeGovernanceRequests(
  handler: (request: LauncherGovernanceRequest) => void
): () => void {
  const webview = launcherWebview();
  if (!webview?.addEventListener) {
    return () => {};
  }
  const listener = (event: MessageEvent) => {
    const data: unknown = event.data;
    if (!data || typeof data !== "object") {
      return;
    }
    const record = data as Record<string, unknown>;
    if (str(record.type) !== LAUNCHER_GOVERNANCE_REQUIRED_TYPE) {
      return;
    }
    const requestID = str(record.request_id);
    if (!requestID) {
      return;
    }
    handler({
      request_id: requestID,
      action: str(record.action),
      current_project: str(record.current_project)
    });
  };
  webview.addEventListener("message", listener);
  return () => {
    webview.removeEventListener?.("message", listener);
  };
}

/**
 * 烟测取证回执（仅 launcher_smoke URL 参数下由 LauncherSurface 调用）：
 * React 把真实收到的 vit_launcher_result 转报宿主日志——脚本断言 React 侧真收。
 */
export function postLauncherResultAck(requestID: string, status: string): boolean {
  return postLauncherBridge({ type: "launcher_result_ack", request_id: requestID, status });
}

/** 订阅宿主结果回执；返回解订函数。桥不可用=空订阅（listener 不挂）。 */
export function subscribeLauncherResults(
  handler: (result: LauncherBridgeResult) => void
): () => void {
  const webview = launcherWebview();
  if (!webview?.addEventListener) {
    return () => {};
  }
  const listener = (event: MessageEvent) => {
    const data: unknown = event.data;
    if (!data || typeof data !== "object") {
      return;
    }
    const record = data as Record<string, unknown>;
    if (str(record.type) !== LAUNCHER_BRIDGE_RESULT_TYPE) {
      return;
    }
    handler({
      type: str(record.type),
      request_id: str(record.request_id),
      status: str(record.status),
      action: str(record.action),
      project_name: str(record.project_name),
      file_path: str(record.file_path),
      scene: str(record.scene),
      governance: str(record.governance),
      message: str(record.message)
    });
  };
  webview.addEventListener("message", listener);
  return () => {
    webview.removeEventListener?.("message", listener);
  };
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}
