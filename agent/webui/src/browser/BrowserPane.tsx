// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2g）：App.tsx BrowserPane+浏览器桥（M 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Archive, ChevronLeft, ChevronRight, Globe, Loader2, RefreshCw, Search, Square } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { AgentInvokeResponse, AgentUIState, ArtifactSummary, ChatResponse, JsonRecord } from "../types";
import { asRecord, finiteNumber, firstArray, numericValue, textValue, truthy, uniqueID } from "../lib/util";
import { captureBrowserPage } from "../lib/api";
import { macroControlID, macroControlValue, macroControlValueMutation, normalizeMacroControl } from "../macro/normalize";
import { mediaScopeKeyFromUIState, uploadMetadata } from "../settings/config";
import { historyScopeKeyFromUIState } from "../historyScope";
import { agentMutationBridgeChannel, browserBridgeChannel, externalBrowserChannel } from "../App";

export type BrowserBridgeState = {
  ready?: boolean;
  loading?: boolean;
  url?: string;
  title?: string;
  last_error?: string;
  can_go_back?: boolean;
  can_go_forward?: boolean;
  visible?: boolean;
  profile?: string;
};

export function BrowserPane({
  conversationID,
  uiState,
  onCaptured,
  onRefresh
}: {
  conversationID: string;
  uiState: AgentUIState | null;
  onCaptured: (artifact: ArtifactSummary) => void;
  onRefresh: () => Promise<void>;
}) {
  const [address, setAddress] = useState("https://www.bing.com");
  const [bridgeReady, setBridgeReady] = useState(() => hasBrowserBridge());
  const [browserState, setBrowserState] = useState<BrowserBridgeState>({});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const captureRequestRef = useRef("");
  const addressRef = useRef(address);
  const browserStateRef = useRef<BrowserBridgeState>({});
  const conversationIDRef = useRef(conversationID);
  const uiStateRef = useRef<AgentUIState | null>(uiState);
  const onCapturedRef = useRef(onCaptured);
  const onRefreshRef = useRef(onRefresh);
  const scopeKey = mediaScopeKeyFromUIState(uiState) || historyScopeKeyFromUIState(uiState) || "unsaved";

  useEffect(() => {
    addressRef.current = address;
  }, [address]);

  useEffect(() => {
    browserStateRef.current = browserState;
  }, [browserState]);

  useEffect(() => {
    conversationIDRef.current = conversationID;
    uiStateRef.current = uiState;
    onCapturedRef.current = onCaptured;
    onRefreshRef.current = onRefresh;
  }, [conversationID, onCaptured, onRefresh, uiState]);

  const postBrowserMessage = useCallback((type: string, payload: JsonRecord = {}) => {
    const didPost = postToBrowserBridge(type, {
      ...payload,
      scope_key: scopeKey
    });
    setBridgeReady(didPost);
    return didPost;
  }, [scopeKey]);

  const syncBounds = useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const rect = viewport.getBoundingClientRect();
    postBrowserMessage("bounds", {
      visible: rect.width > 8 && rect.height > 8,
      rect: {
        x: rect.left,
        y: rect.top,
        screen_x: window.screenX + rect.left,
        screen_y: window.screenY + rect.top,
        width: rect.width,
        height: rect.height,
        dpr: window.devicePixelRatio || 1
      }
    });
  }, [postBrowserMessage]);

  const saveCapturedPayload = useCallback(async (payload: JsonRecord) => {
    setBusy(true);
    setMessage("");
    try {
      const currentBrowserState = browserStateRef.current;
      const result = await captureBrowserPage({
        url: textValue(payload.url ?? currentBrowserState.url, ""),
        title: textValue(payload.title ?? currentBrowserState.title, ""),
        text: textValue(payload.text, ""),
        selection_text: textValue(payload.selection_text ?? payload.selected_text, ""),
        ...uploadMetadata(conversationIDRef.current, uiStateRef.current)
      });
      if (result.artifact) {
        onCapturedRef.current(result.artifact);
        setMessage("已捕获到资料库，并附加到当前对话");
        await onRefreshRef.current();
      } else {
        setMessage("捕获完成，但没有返回媒体记录");
      }
    } catch (captureError) {
      setMessage(captureError instanceof Error ? captureError.message : "捕获失败");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    const bridge = browserBridge();
    setBridgeReady(Boolean(bridge));
    if (!bridge) {
      return;
    }
    const handleMessage = (event: MessageEvent) => {
      const data = browserBridgePayload(event.data);
      if (textValue(data.channel, "") !== browserBridgeChannel) {
        return;
      }
      const type = textValue(data.type, "");
      if (type === "state") {
        const state = asRecord(data.state);
        setBrowserState(state as BrowserBridgeState);
        const nextURL = textValue(state.url, "");
        if (nextURL) {
          setAddress(nextURL);
        }
        const errorText = textValue(state.last_error, "");
        if (errorText) {
          setMessage(errorText);
        }
      }
      if (type === "capture_result") {
        const requestID = textValue(data.request_id, "");
        if (requestID && requestID !== captureRequestRef.current) {
          return;
        }
        const errorText = textValue(data.error, "");
        if (errorText) {
          setBusy(false);
          setMessage(errorText);
          return;
        }
        void saveCapturedPayload(asRecord(data.payload));
      }
    };
    bridge.addEventListener?.("message", handleMessage);
    postToBrowserBridge("mount", { url: addressRef.current, scope_key: scopeKey, visible: true });
    window.requestAnimationFrame(syncBounds);
    return () => {
      bridge.removeEventListener?.("message", handleMessage);
      postToBrowserBridge("hide", { scope_key: scopeKey });
    };
  }, [saveCapturedPayload, scopeKey, syncBounds]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !bridgeReady) {
      return;
    }
    const resizeObserver = new ResizeObserver(() => syncBounds());
    resizeObserver.observe(viewport);
    window.addEventListener("resize", syncBounds);
    window.addEventListener("scroll", syncBounds, true);
    const timer = window.setInterval(syncBounds, 1200);
    syncBounds();
    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", syncBounds);
      window.removeEventListener("scroll", syncBounds, true);
      window.clearInterval(timer);
    };
  }, [bridgeReady, syncBounds]);

  const navigate = (rawURL: string) => {
    const target = normalizeBrowserURL(rawURL);
    if (!target) {
      return;
    }
    setAddress(target);
    setMessage("");
    postBrowserMessage("navigate", { url: target });
    window.requestAnimationFrame(syncBounds);
  };

  const capture = () => {
    if (!bridgeReady || !browserState.ready) {
      setMessage("WebView2 companion 尚未就绪");
      return;
    }
    const requestID = uniqueID("browser_capture");
    captureRequestRef.current = requestID;
    setBusy(true);
    setMessage("");
    postBrowserMessage("capture", { request_id: requestID });
  };

  const loading = Boolean(browserState.loading);
  const lastError = textValue(browserState.last_error, "");
  const statusLabel = bridgeReady
    ? browserState.ready
      ? loading
        ? "加载中"
        : "就绪"
      : "等待 WebView2"
    : "需要 Godot WebView2";

  return (
    <div className="pane-body browser-pane">
      <section className="browser-shell">
        <form
          className="browser-toolbar"
          onSubmit={(event) => {
            event.preventDefault();
            navigate(address);
          }}
        >
          <button className="icon-button" type="button" title="后退" onClick={() => postBrowserMessage("back")} disabled={!browserState.can_go_back}>
            <ChevronLeft size={16} />
          </button>
          <button className="icon-button" type="button" title="鍓嶈繘" onClick={() => postBrowserMessage("forward")} disabled={!browserState.can_go_forward}>
            <ChevronRight size={16} />
          </button>
          <button className="icon-button" type="button" title={loading ? "停止加载" : "刷新"} onClick={() => postBrowserMessage(loading ? "stop" : "reload")} disabled={!browserState.ready}>
            {loading ? <Square size={14} /> : <RefreshCw size={15} />}
          </button>
          <label className="browser-address">
            <Globe size={15} />
            <input
              value={address}
              spellCheck={false}
              onFocus={() => postBrowserMessage("focus")}
              onChange={(event) => setAddress(event.currentTarget.value)}
              placeholder="搜索或输入网址"
              disabled={!bridgeReady}
            />
          </label>
          <button className="tool-button primary" type="submit" disabled={!bridgeReady}>
            <Search size={16} />
            <span>打开</span>
          </button>
        </form>
        <div className="browser-status-line">
          <span className={`browser-status-dot ${browserState.ready ? "ready" : ""} ${lastError ? "error" : ""}`} />
          <strong>{statusLabel}</strong>
          <span>{textValue(browserState.title, "") || textValue(browserState.url, "") || "WebView2 companion browser"}</span>
          <button className="tool-button" type="button" onClick={capture} disabled={busy || !bridgeReady || !browserState.ready}>
            {busy ? <Loader2 className="spin" size={16} /> : <Archive size={16} />}
            <span>捕获到资料库</span>
          </button>
        </div>
        <div ref={viewportRef} className="browser-viewport" onClick={() => postBrowserMessage("focus")}>
          <div className="browser-viewport-placeholder">
            <Globe size={36} />
            <strong>{bridgeReady ? "WebView2 浏览器区域" : "等待 Godot WebView2 companion"}</strong>
            <span>{bridgeReady ? "真实浏览器会覆盖此区域；页面内容不会自动进入 Agent 上下文。" : "请从 Godot 内打开 Ask Vit。开发浏览器里不会伪装登录浏览器。"}</span>
          </div>
        </div>
        {(message || lastError) && <div className={`notice ${lastError ? "warning" : ""}`}>{message || lastError}</div>}
      </section>
    </div>
  );
}

type BrowserWebViewBridge = {
  postMessage: (message: unknown) => void;
  addEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
  removeEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
};

export function browserBridge(): BrowserWebViewBridge | null {
  const hostWindow = window as unknown as { chrome?: { webview?: BrowserWebViewBridge } };
  return hostWindow.chrome?.webview ?? null;
}

export function hasBrowserBridge(): boolean {
  return Boolean(browserBridge()?.postMessage);
}

export function postToBrowserBridge(type: string, payload: JsonRecord = {}): boolean {
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      ...payload,
      channel: browserBridgeChannel,
      type
    });
    return true;
  } catch {
    return false;
  }
}

export function postExternalBrowserMessage(type: string, payload: JsonRecord = {}): boolean {
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      ...payload,
      channel: externalBrowserChannel,
      type
    });
    return true;
  } catch {
    return false;
  }
}

export function postAgentMutationsFromChatResponse(response: ChatResponse, source: string): boolean {
  return postAgentMutations(agentMutationsFromChatResponse(response), source);
}

export function postAgentMutationsFromInvokeResponse(response: AgentInvokeResponse, source: string): boolean {
  return postAgentMutations(agentMutationsFromInvokeResponse(response), source);
}

export function postAgentMutations(mutations: JsonRecord[], source: string): boolean {
  if (mutations.length === 0) {
    return false;
  }
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      channel: agentMutationBridgeChannel,
      type: "agent_kernel_result",
      source,
      mutation_count: mutations.length,
      mutations
    });
    return true;
  } catch {
    return false;
  }
}

function agentMutationsFromChatResponse(response: ChatResponse): JsonRecord[] {
  const mutations: JsonRecord[] = [];
  firstArray(response.executed_kernel_reply).forEach((entry) => {
    appendAgentMutationsFromExecution(mutations, asRecord(entry));
  });
  return compactAgentMutations(mutations);
}

function agentMutationsFromInvokeResponse(response: AgentInvokeResponse): JsonRecord[] {
  const result = asRecord(response.result);
  if (Object.keys(result).length === 0) {
    return [];
  }
  const entry: JsonRecord = {
    status: response.status,
    command_name: response.command_name,
    tool: response.tool,
    risk_level: response.risk_level,
    result
  };
  const mutations: JsonRecord[] = [];
  appendAgentMutationsFromExecution(mutations, entry);
  return compactAgentMutations(mutations);
}

export function appendAgentMutationsFromExecution(mutations: JsonRecord[], entry: JsonRecord): void {
  const result = asRecord(entry.result);
  const payload = Object.keys(result).length > 0 ? result : entry;
  if (!agentExecutionSucceeded(entry, payload)) {
    return;
  }
  const commandName = textValue(entry.command_name ?? entry.tool ?? entry.cmd ?? entry.command ?? payload.command_name ?? payload.tool ?? payload.cmd ?? payload.command, "").toLowerCase();
  const uiAction = textValue(payload.ui_action ?? entry.ui_action, "").toLowerCase();
  const trackID = textValue(payload.track_id ?? payload.target_track_id ?? entry.track_id, "");
  const clipID = textValue(payload.clip_id ?? payload.new_clip_id ?? payload.created_clip_id ?? entry.clip_id, "");
  const notes = firstArray(payload.notes).map(asRecord);
  const selectedExistingMacro = uiAction === "rack_macro_selected" || truthy(payload.selected_existing_macro);

  if (uiAction === "select_clip" || commandName === "select_clip" || hasCreatedClipPayload(payload)) {
    if (clipID) {
      mutations.push({
        kind: "select_clip",
        track_id: trackID,
        clip_id: clipID,
        created_clip_ids: firstArray(payload.created_clip_ids),
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "focus_track" || uiAction === "select_track" || commandName === "select_track") {
    if (trackID) {
      mutations.push({
        kind: "focus_track",
        track_id: trackID,
        source_command: commandName
      });
    }
  }

  if (uiAction === "focus_plugin" || uiAction === "select_plugin" || commandName === "select_plugin") {
    const pluginID = textValue(payload.plugin_id ?? payload.plugin_item_id ?? payload.node_id ?? entry.plugin_id, "");
    if (pluginID) {
      mutations.push({
        kind: "focus_plugin",
        track_id: trackID,
        plugin_id: pluginID,
        source_command: commandName
      });
    }
  }

  if (isTrackStateRefreshCommand(commandName, uiAction)) {
    mutations.push({
      kind: trackID ? "track_state_changed" : "project_state_changed",
      track_id: trackID,
      mute: payload.mute ?? payload.muted,
      solo: payload.solo,
      armed: payload.is_armed ?? payload.armed,
      volume_db: payload.volume_db ?? payload.db,
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (isClipStateRefreshCommand(commandName, uiAction)) {
    mutations.push({
      kind: "project_state_changed",
      track_id: trackID,
      clip_id: clipID,
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (uiAction === "remove_clips" || commandName === "remove_clips") {
    mutations.push({
      kind: "remove_clips",
      clip_ids: firstArray(payload.removed_clip_ids, payload.requested_clip_ids, payload.clip_ids),
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (uiAction === "midi_note_patch" || isMidiNoteWriteCommand(commandName)) {
    if (clipID || notes.length > 0) {
      const insertedCount = numericValue(payload.inserted_count ?? payload.added_count);
      const mutatedCount = numericValue(payload.mutated_count);
      const quantizedCount = numericValue(payload.quantized_count);
      mutations.push({
        kind: "midi_notes_changed",
        track_id: trackID,
        clip_id: clipID,
        notes,
        inserted_count: insertedCount,
        mutated_count: mutatedCount,
        deleted_count: payload.deleted_count ?? payload.removed_count,
        quantized_count: quantizedCount,
        source_command: commandName,
        fetch_if_missing: notes.length === 0 && (insertedCount > 0 || mutatedCount > 0 || quantizedCount > 0)
      });
    }
  }

  if (uiAction === "rack_macro_value_changed" || commandName === "control.set_macro_values" || commandName === "control_set_macro_values") {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload.control ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    if (macroID) {
      mutations.push(macroControlValueMutation({ ...macro, macro_id: macroID }, finiteNumber(payload.value ?? macro.value, macroControlValue(macro)), true, "agent_invoke"));
    }
    firstArray(payload.applied_parameters).map(asRecord).forEach((applied) => {
      if (textValue(applied.control ?? applied.param_id, "") !== "track.volume") {
        return;
      }
      const appliedTrackID = textValue(applied.track_id, "");
      const targetValue = finiteNumber(applied.target_value, NaN);
      if (!appliedTrackID || Number.isNaN(targetValue)) {
        return;
      }
      mutations.push({
        kind: "track_state_changed",
        track_id: appliedTrackID,
        volume_db: targetValue,
        source_command: commandName,
        requires_refresh: true
      });
    });
  }

  if (!selectedExistingMacro && (uiAction === "rack_macro_upserted" || commandName === "plugin_map_macro_to_params" || commandName === "control_add_macro")) {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    if (macroID) {
      mutations.push({
        kind: "rack_macro_upserted",
        macro_id: macroID,
        macro: { ...macro, macro_id: macroID },
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "rack_macro_renamed" || commandName === "control.rename_macro" || commandName === "control_rename_macro") {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    const name = textValue(payload.name ?? payload.new_name ?? macro.name, "");
    if (macroID && name) {
      mutations.push({
        kind: "rack_macro_renamed",
        macro_id: macroID,
        name,
        macro: { ...macro, macro_id: macroID, name },
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "rack_macro_binding_added" || commandName === "control.add_binding" || commandName === "control_add_binding") {
    const macroID = textValue(payload.macro_id ?? entry.macro_id, "");
    const binding = asRecord(payload.binding ?? payload);
    if (macroID && Object.keys(binding).length > 0) {
      mutations.push({
        kind: "rack_macro_binding_added",
        macro_id: macroID,
        binding,
        source_command: commandName,
        requires_refresh: true
      });
    }
  }
}

export function agentExecutionSucceeded(entry: JsonRecord, payload: JsonRecord): boolean {
  const entryStatus = textValue(entry.status, "").toLowerCase();
  const payloadStatus = textValue(payload.status, "").toLowerCase();
  if (
    entryStatus === "error" ||
    entryStatus === "failed" ||
    entryStatus === "failure" ||
    payloadStatus === "error" ||
    payloadStatus === "failed" ||
    payloadStatus === "failure"
  ) {
    return false;
  }
  const status = entryStatus || payloadStatus;
  return status === "" || status === "ok" || status === "success" || status === "completed";
}

function hasCreatedClipPayload(payload: JsonRecord): boolean {
  return textValue(payload.new_clip_id ?? payload.created_clip_id, "") !== "" || firstArray(payload.created_clip_ids).length > 0;
}

function isMidiNoteWriteCommand(commandName: string): boolean {
  return [
    "apply_midi_note_patch",
    "add_midi_notes",
    "add_midi_notes_bulk",
    "mutate_midi_notes",
    "delete_midi_notes",
    "midi.apply_note_patch",
    "midi.add_notes",
    "midi.add_notes_bulk"
  ].includes(commandName);
}

function isTrackStateRefreshCommand(commandName: string, uiAction: string): boolean {
  if (uiAction === "track_state_changed" || uiAction === "project_state_changed") {
    return true;
  }
  return [
    "add_track",
    "track.add",
    "add_audio_track",
    "track.add_audio",
    "append_ghost_track",
    "track.append_ghost",
    "delete_track",
    "track.delete",
    "rename_track",
    "track.rename",
    "set_mute",
    "track.mute",
    "set_solo",
    "track.solo",
    "arm_track",
    "track.arm",
    "set_volume",
    "track.volume",
    "rack_add_node",
    "rack.add_node",
    "instantiate_plugin",
    "plugin.load_to_rack",
    "delete_plugin",
    "plugin.delete",
    "freeze_track",
    "track.freeze",
    "unfreeze_track",
    "track.unfreeze"
  ].includes(commandName);
}

function isClipStateRefreshCommand(commandName: string, uiAction: string): boolean {
  if (uiAction === "clip_state_changed" || uiAction === "project_state_changed") {
    return true;
  }
  return [
    "move_clip",
    "clip.move",
    "resize_clip",
    "clip.resize",
    "split_clip",
    "clip.split",
    "clone_clip",
    "clip.clone",
    "add_audio_clip",
    "clip.add_audio",
    "import_audio",
    "clip.import_audio",
    "import_media_to_track",
    "clip.import_media_to_track",
    "insert_midi_clip",
    "midi.insert_clip",
    "create_midi_clip",
    "midi.create_clip"
  ].includes(commandName);
}

function compactAgentMutations(mutations: JsonRecord[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  mutations.forEach((mutation, index) => {
    const key = [
      textValue(mutation.kind, ""),
      textValue(mutation.track_id, ""),
      textValue(mutation.clip_id, ""),
      textValue(mutation.macro_id, ""),
      textValue(mutation.source_command, ""),
      index
    ].join(":");
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(mutation);
  });
  return out;
}

function browserBridgePayload(raw: unknown): JsonRecord {
  if (typeof raw === "string") {
    try {
      return asRecord(JSON.parse(raw));
    } catch {
      return {};
    }
  }
  return asRecord(raw);
}

export function normalizeBrowserURL(rawURL: string): string {
  const value = rawURL.trim();
  if (!value) {
    return "";
  }
  const lower = value.toLowerCase();
  if (
    lower.startsWith("http://") ||
    lower.startsWith("https://") ||
    lower.startsWith("file://") ||
    lower.startsWith("data:") ||
    lower.startsWith("about:")
  ) {
    return value;
  }
  if (/^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?(\/.*)?$/i.test(value)) {
    return `http://${value}`;
  }
  if (value.includes(".") && !/\s/.test(value)) {
    return `https://${value}`;
  }
  return `https://www.bing.com/search?q=${encodeURIComponent(value)}`;
}

