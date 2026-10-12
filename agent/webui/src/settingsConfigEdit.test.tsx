import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
  RouteEditor,
  SettingsPage,
  SettingsTextField,
  mergeSettingsConfig,
  retainSecretInputs,
  settingsSavedStatus
} from "./App";
import type { EngineConfig } from "./types";

function renderSettingsPage() {
  return renderToStaticMarkup(<SettingsPage runtimeStatus={null} uiState={null} onClose={() => {}} />);
}

// 服务端保存成功后的回传形态：RedactSecrets 抹掉主/路由 key 明文。
function savedServerConfig(): EngineConfig {
  return {
    baseUrl: "https://api.example.com/v1",
    apiKey: "",
    defaultModel: "provider/model-b",
    multimodalRoutes: {
      image: { enabled: true, provider: "openai", model: "vision-b", apiKey: "" }
    }
  };
}

describe("FE-CONFIG-EDIT-1 设置页占位符与输入防扰", () => {
  it("默认模型占位符为'供应商模型 ID'，无双重编码乱码", () => {
    const markup = renderSettingsPage();
    expect(markup).toContain('placeholder="供应商模型 ID"');
    expect(markup).not.toMatch(/[渚涘簲鍟嗘ā鍨]/);
  });

  it("LLM 区块：API Key 框 autocomplete=new-password，其余文本框 autocomplete=off", () => {
    const markup = renderSettingsPage();
    expect(markup).toContain('<input type="password"');
    expect(markup).toMatch(/autocomplete="new-password"/i);
    const inputs = markup.match(/<input[^>]*>/g) ?? [];
    expect(inputs.length).toBeGreaterThanOrEqual(3);
    for (const input of inputs) {
      expect(input).toMatch(/autocomplete=/i);
    }
    const password = inputs.find((input) => input.includes('type="password"'));
    expect(password).toMatch(/autocomplete="new-password"/i);
    const plainTexts = inputs.filter((input) => !input.includes('type="password"'));
    for (const input of plainTexts) {
      expect(input).toMatch(/autocomplete="off"/i);
    }
  });

  it("多模态路由：API Key 框 new-password，其余字段含 Priority 均 off", () => {
    const markup = renderToStaticMarkup(
      <RouteEditor
        meta={{ key: "image", label: "图像", hint: "图像路由" }}
        route={{ provider: "openai", model: "vision", apiKey: "route-key-1", priority: 2 }}
        hasAPIKey={false}
        onChange={() => {}}
      />
    );
    const inputs = markup.match(/<input[^>]*>/g) ?? [];
    const password = inputs.find((input) => input.includes('type="password"'));
    expect(password).toMatch(/autocomplete="new-password"/i);
    for (const input of inputs) {
      // checkbox（启用开关）不适用 autoComplete；文本/数字框须显式 off
      if (input !== password && !input.includes('type="checkbox"')) {
        expect(input).toMatch(/autocomplete="off"/i);
      }
    }
    const priority = inputs.find((input) => input.includes('type="number"'));
    expect(priority).toMatch(/autocomplete="off"/i);
  });

  it("SettingsTextField：受控 value 原样进入 DOM（key 输入态可见）", () => {
    const markup = renderToStaticMarkup(
      <SettingsTextField label="API Key" value="sk-test-123" type="password" autoComplete="new-password" onChange={() => {}} />
    );
    expect(markup).toContain('value="sk-test-123"');
    expect(markup).toMatch(/autocomplete="new-password"/i);
  });
});

describe("FE-CONFIG-EDIT-1 保存后 key 输入态保持", () => {
  it("retainSecretInputs：主 key 与路由 key 沿用当前输入，非敏感字段以服务端回传为准", () => {
    const current: EngineConfig = {
      baseUrl: "https://old.example.com/v1",
      apiKey: "sk-just-typed",
      defaultModel: "provider/model-a",
      multimodalRoutes: {
        image: { enabled: true, provider: "openai", model: "vision-a", apiKey: "route-just-typed" }
      }
    };
    const next = retainSecretInputs(mergeSettingsConfig(savedServerConfig()), current);
    expect(next.apiKey).toBe("sk-just-typed");
    expect(next.multimodalRoutes?.image?.apiKey).toBe("route-just-typed");
    expect(next.defaultModel).toBe("provider/model-b");
    expect(next.baseUrl).toBe("https://api.example.com/v1");
    expect(next.multimodalRoutes?.image?.model).toBe("vision-b");
  });

  it("留空保存时 key 保持留空（沿用已存语义），不为用户填入任何值", () => {
    const current: EngineConfig = { apiKey: "", multimodalRoutes: { image: { apiKey: "" } } };
    const next = retainSecretInputs(mergeSettingsConfig(savedServerConfig()), current);
    expect(next.apiKey).toBe("");
    expect(next.multimodalRoutes?.image?.apiKey).toBe("");
  });

  it("对照：mergeSettingsConfig 仍清空 key（重开设置页/重载的初始加载语义不变）", () => {
    expect(mergeSettingsConfig(savedServerConfig()).apiKey).toBe("");
    expect(mergeSettingsConfig({ ...savedServerConfig(), apiKey: "leak" }).apiKey).toBe("");
  });
});

describe("FE-CONFIG-EDIT-1 保存状态行两情形文案", () => {
  it("已输入 key：明示输入框保留所输内容", () => {
    expect(settingsSavedStatus("sk-just-typed", true)).toContain("已保存");
    expect(settingsSavedStatus("sk-just-typed", true)).toContain("输入框保留所输内容");
  });

  it("留空且服务端已存 key：明示沿用已存 key", () => {
    const status = settingsSavedStatus("", true);
    expect(status).toContain("已保存");
    expect(status).toContain("留空=沿用已存 key");
  });

  it("留空且无已存 key：仅常规已保存文案", () => {
    const status = settingsSavedStatus("", false);
    expect(status).toContain("已保存");
    expect(status).not.toContain("沿用已存");
    expect(status).not.toContain("保留所输内容");
  });
});
