// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx SettingsPage 族（D 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Activity, Archive, Bot, Brain, CheckCircle2, Globe, Loader2, RefreshCw, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import type { AgentConfigResponse, AgentUIState, EngineConfig, MultimodalRouteConfig, RuntimeStatusResponse } from "../types";
import { asRecord, lastPathPart, textValue } from "../lib/util";
import { fetchAgentConfig, saveAgentConfig } from "../lib/api";
import { defaultSettingsConfig, mergeSettingsConfig, multimodalRouteMeta, retainSecretInputs, settingsSavedStatus } from "./config";
import { buildDiagnosticsPayload, buildDiagnosticsRows, InfoGrid, KeyValue } from "../topbar/diagnostics";
import { statusLabel } from "../actions/labels";

export type SettingsTab = "agent" | "llm" | "multimodal" | "browser" | "diagnostics";

export function SettingsPage({
  runtimeStatus,
  uiState,
  onClose
}: {
  runtimeStatus: RuntimeStatusResponse | null;
  uiState: AgentUIState | null;
  onClose: () => void;
}) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("llm");
  const [config, setConfig] = useState<EngineConfig>(() => defaultSettingsConfig());
  const [configResponse, setConfigResponse] = useState<AgentConfigResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState("");

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setStatus("读取配置中...");
    try {
      const response = await fetchAgentConfig();
      setConfigResponse(response);
      setConfig(mergeSettingsConfig(response.config));
      setStatus(response.path ? `配置文件：${response.path}` : "配置已读取");
    } catch (configError) {
      setStatus(configError instanceof Error ? configError.message : "配置读取失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const saveConfig = async () => {
    setSaving(true);
    setStatus("保存中...");
    try {
      const response = await saveAgentConfig(config);
      setConfigResponse(response);
      // 保存成功后服务端不回传明文 key（RedactSecrets）；沿用当前输入，避免刚输入的 key 从输入框消失。
      // 清空 key 框的语义只属于重新读取配置（loadConfig/重开设置页）。
      setConfig((current) => retainSecretInputs(mergeSettingsConfig(response.config), current));
      setStatus(settingsSavedStatus(config.apiKey ?? "", Boolean(response.hasApiKey)));
    } catch (saveError) {
      setStatus(saveError instanceof Error ? saveError.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const setConfigField = <K extends keyof EngineConfig>(key: K, value: EngineConfig[K]) => {
    setConfig((current) => ({ ...current, [key]: value }));
  };

  const updateRoute = (routeKey: string, patch: Partial<MultimodalRouteConfig>) => {
    setConfig((current) => ({
      ...current,
      multimodalRoutes: {
        ...(current.multimodalRoutes ?? {}),
        [routeKey]: {
          ...(current.multimodalRoutes?.[routeKey] ?? {}),
          ...patch
        }
      }
    }));
  };

  const browser = config.browser ?? {};
  const kernel = asRecord(runtimeStatus?.kernel);
  const project = asRecord(uiState?.project);
  const diagnosticsRows = buildDiagnosticsRows(runtimeStatus, uiState);
  const diagnosticsPayload = buildDiagnosticsPayload(runtimeStatus, uiState, configResponse);

  return (
    <section className="settings-page">
      <div className="settings-header">
        <div>
          <p className="eyebrow">Ask Vit</p>
          <h1>设置</h1>
        </div>
        <div className="settings-actions">
          <button className="tool-button" type="button" onClick={() => void loadConfig()} disabled={loading || saving}>
            <RefreshCw size={16} />
            <span>重载</span>
          </button>
          <button className="tool-button primary" type="button" onClick={() => void saveConfig()} disabled={loading || saving}>
            {saving ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            <span>保存</span>
          </button>
          <button className="icon-button" type="button" title="关闭设置" onClick={onClose}>
            <X size={16} />
          </button>
        </div>
      </div>

      <div className="settings-grid">
        <nav className="settings-nav" aria-label="Settings sections">
          {settingsTabs.map(({ key, label, icon: Icon }) => (
            <button key={key} type="button" className={activeTab === key ? "active" : ""} onClick={() => setActiveTab(key)}>
              <Icon size={16} />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <div className="settings-body">
          {activeTab === "agent" && (
            <section className="settings-section">
              <h2>Agent</h2>
              <InfoGrid
                rows={[
                  ["服务", runtimeStatus?.service ?? "VitAgent"],
                  ["PID", textValue(runtimeStatus?.pid, "-")],
                  ["内核", Boolean(kernel.connected) ? "已连接" : "未连接"],
                  ["工程", lastPathPart(textValue(project.project_path, "Vit Project"))]
                ]}
              />
              <KeyValue label="配置" value={configResponse?.path ?? "~/.vit/config.json"} />
              <KeyValue label="状态" value={statusLabel(status, status || "空闲")} />
            </section>
          )}

          {activeTab === "llm" && (
            <section className="settings-section">
              <h2>LLM API</h2>
              <SettingsTextField
                label="Base URL"
                value={config.baseUrl ?? ""}
                placeholder="https://api.openai.com/v1"
                onChange={(value) => setConfigField("baseUrl", value)}
              />
              <SettingsTextField
                label="API Key"
                value={config.apiKey ?? ""}
                type="password"
                autoComplete="new-password"
                placeholder={configResponse?.hasApiKey ? "API Key 已保存；留空保持不变" : "API Key"}
                onChange={(value) => setConfigField("apiKey", value)}
              />
              <SettingsTextField
                label="默认模型"
                value={config.defaultModel ?? ""}
                placeholder="供应商模型 ID"
                onChange={(value) => setConfigField("defaultModel", value)}
              />
            </section>
          )}

          {activeTab === "multimodal" && (
            <section className="settings-section">
              <h2>多模态路由</h2>
              <div className="route-list">
                {multimodalRouteMeta.map((meta) => (
                  <RouteEditor
                    key={meta.key}
                    meta={meta}
                    route={config.multimodalRoutes?.[meta.key] ?? {}}
                    hasAPIKey={Boolean(configResponse?.routeApiKeys?.[meta.key])}
                    onChange={(patch) => updateRoute(meta.key, patch)}
                  />
                ))}
              </div>
            </section>
          )}

          {activeTab === "browser" && (
            <section className="settings-section">
              <h2>浏览器</h2>
              <SettingsCheckboxField
                label="启用 WebView2 companion"
                checked={Boolean(browser.enabled)}
                onChange={(checked) => setConfigField("browser", { ...browser, enabled: checked })}
              />
              <SettingsTextField
                label="模式"
                value={browser.mode ?? "webview2_companion"}
                onChange={(value) => setConfigField("browser", { ...browser, mode: value })}
              />
              <SettingsTextField
                label="Profile Path"
                value={browser.profilePath ?? ""}
                placeholder="留空使用默认浏览器 profile"
                onChange={(value) => setConfigField("browser", { ...browser, profilePath: value })}
              />
              <SettingsCheckboxField
                label="保留登录状态"
                checked={Boolean(browser.rememberSession)}
                onChange={(checked) => setConfigField("browser", { ...browser, rememberSession: checked })}
              />
            </section>
          )}

          {activeTab === "diagnostics" && (
            <section className="settings-section">
              <h2>诊断</h2>
              <InfoGrid rows={diagnosticsRows} />
              <pre className="settings-json">
                {JSON.stringify(diagnosticsPayload, null, 2)}
              </pre>
            </section>
          )}
        </div>
      </div>
    </section>
  );
}

export function SettingsTextField({
  label,
  value,
  placeholder,
  type = "text",
  autoComplete = "off",
  onChange
}: {
  label: string;
  value: string;
  placeholder?: string;
  type?: string;
  autoComplete?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="settings-field">
      <span>{label}</span>
      <input
        type={type}
        value={value}
        placeholder={placeholder}
        autoComplete={autoComplete}
        onChange={(event) => onChange(event.currentTarget.value)}
      />
    </label>
  );
}

export function SettingsCheckboxField({
  label,
  checked,
  onChange
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="settings-check">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.currentTarget.checked)} />
      <span>{label}</span>
    </label>
  );
}

export function RouteEditor({
  meta,
  route,
  hasAPIKey,
  onChange
}: {
  meta: { key: string; label: string; hint: string };
  route: MultimodalRouteConfig;
  hasAPIKey: boolean;
  onChange: (patch: Partial<MultimodalRouteConfig>) => void;
}) {
  return (
    <section className="route-card">
      <div className="route-card-header">
        <div>
          <h3>{meta.label}</h3>
          <span>{meta.hint}</span>
        </div>
        <SettingsCheckboxField label="启用" checked={Boolean(route.enabled)} onChange={(enabled) => onChange({ enabled })} />
      </div>
      <div className="route-fields">
        <SettingsTextField label="Provider" value={route.provider ?? ""} placeholder="default / openai / local" onChange={(provider) => onChange({ provider })} />
        <SettingsTextField label="Model" value={route.model ?? ""} placeholder="model id" onChange={(model) => onChange({ model })} />
        <SettingsTextField label="Base URL" value={route.baseUrl ?? ""} placeholder="留空继承 LLM API" onChange={(baseUrl) => onChange({ baseUrl })} />
        <SettingsTextField
          label="API Key"
          value={route.apiKey ?? ""}
          type="password"
          autoComplete="new-password"
          placeholder={hasAPIKey ? "API Key 已保存；留空保持不变" : "留空继承 LLM API key"}
          onChange={(apiKey) => onChange({ apiKey })}
        />
        <SettingsTextField label="Fallback" value={route.fallback ?? ""} placeholder="fallback route key" onChange={(fallback) => onChange({ fallback })} />
        <label className="settings-field">
          <span>Priority</span>
          <input
            type="number"
            autoComplete="off"
            value={route.priority ?? 0}
            onChange={(event) => onChange({ priority: Number(event.currentTarget.value) || 0 })}
          />
        </label>
      </div>
    </section>
  );
}

const settingsTabs: Array<{ key: SettingsTab; label: string; icon: LucideIcon }> = [
  { key: "agent", label: "Agent", icon: Bot },
  { key: "llm", label: "LLM API", icon: Brain },
  { key: "multimodal", label: "多模态路由", icon: Archive },
  { key: "browser", label: "浏览器", icon: Globe },
  { key: "diagnostics", label: "诊断", icon: Activity }
];
