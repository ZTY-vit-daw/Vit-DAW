import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { DesignSprite, DESIGN_SYMBOLS, SPRITE_SYMBOL_IDS } from "./sprite";

const SPRITE_SOURCE_PATH = fileURLToPath(
  new URL("../../design/main-window-v5/round2-sprite.svg", import.meta.url)
);
const TOKENS_JSON_PATH = fileURLToPath(
  new URL("../../design/main-window-v5/design-tokens-v5.json", import.meta.url)
);
const TOKENS_CSS_PATH = fileURLToPath(
  new URL("../styles/tokens.css", import.meta.url)
);

const spriteSource = readFileSync(SPRITE_SOURCE_PATH, "utf-8");
const tokensCssSource = readFileSync(TOKENS_CSS_PATH, "utf-8");
const tokensJson = JSON.parse(readFileSync(TOKENS_JSON_PATH, "utf-8")) as {
  tokens: Record<string, string>;
};

describe("v5.1 雪碧图（UI-FOUNDATION-1）", () => {
  it("23 个 symbol：22 枚功能图标 + i-vit", () => {
    expect(SPRITE_SYMBOL_IDS).toHaveLength(23);
    expect(Object.keys(DESIGN_SYMBOLS)).toHaveLength(23);
    expect(new Set(SPRITE_SYMBOL_IDS).size).toBe(23);
    expect(SPRITE_SYMBOL_IDS).toContain("i-vit");
    expect(SPRITE_SYMBOL_IDS).toContain("i-project");
    expect(SPRITE_SYMBOL_IDS).toContain("i-error");
  });

  it("每个 symbol markup 与 round2-sprite.svg 逐字一致", () => {
    for (const id of SPRITE_SYMBOL_IDS) {
      expect(spriteSource, `${id} markup 必须逐字出现在设计源文件中`).toContain(
        DESIGN_SYMBOLS[id]
      );
    }
  });

  it("设计源文件中没有多余 symbol", () => {
    const idsInFile = [...spriteSource.matchAll(/<symbol id="([^"]+)"/g)].map(
      (m) => m[1]
    );
    expect(idsInFile.sort()).toEqual([...SPRITE_SYMBOL_IDS].sort());
  });

  it("DesignSprite 渲染 23 个 <symbol>，id 与 viewBox 正确", () => {
    const markup = renderToStaticMarkup(<DesignSprite />);
    expect(markup).toContain('class="v5-sprite"');
    const ids = [...markup.matchAll(/<symbol id="([^"]+)" viewBox="0 0 24 24"/g)].map(
      (m) => m[1]
    );
    expect(ids).toEqual([...SPRITE_SYMBOL_IDS]);
  });
});

describe("v5.1 令牌回指（design-tokens-v5.json -> tokens.css）", () => {
  const squash = (s: string) => s.toLowerCase().replace(/\s+/g, "");

  it("JSON 令牌值全部出现在 tokens.css 中", () => {
    const css = squash(tokensCssSource);
    const missing: string[] = [];
    for (const [key, value] of Object.entries(tokensJson.tokens)) {
      if (!css.includes(squash(value))) missing.push(`${key}=${value}`);
    }
    expect(missing, `tokens.css 缺少回指: ${missing.join(", ")}`).toEqual([]);
  });

  it("v5.1 深度令牌在卷（read-workspace / window-edge / selected ring / 三层柔影）", () => {
    const css = squash(readFileSync(TOKENS_CSS_PATH, "utf-8"));
    for (const needle of [
      "#f0f1f2",
      "#5b626a",
      "0 0 0 1px rgba(20,20,20,.20)",
      "0 18px 48px rgba(0,0,0,.28)",
      "#f6e7aa",
    ].map(squash)) {
      expect(css).toContain(needle);
    }
  });
});

describe("v5.1 组件语义红线", () => {
  const options = [
    { id: "a", name: "温暖", description: "中频略厚。", contour: [0.1, 0.5, 0.9] },
    { id: "b", name: "明亮", description: "高频提起。", contour: [0.9, 0.5, 0.1] },
    { id: "c", name: "紧实", description: "低频收短。", contour: [0.5, 0.9, 0.5] },
  ];

  it("渲染宏卡四态 data-macro-state", async () => {
    const { MacroCard } = await import("../components/MacroCard");
    for (const state of ["default", "auditioning", "applied", "failed"] as const) {
      const markup = renderToStaticMarkup(
        <MacroCard
          object="鼓组总线"
          name="染色"
          options={options}
          selectedId="a"
          onSelect={() => {}}
          state={state}
          failureReason="设备占用。"
        />
      );
      expect(markup).toContain(`data-macro-state="${state}"`);
    }
  });

  it("未选择时应用按钮禁用（选择 ≠ 应用）", async () => {
    const { MacroCard } = await import("../components/MacroCard");
    const markup = renderToStaticMarkup(
      <MacroCard
        object="鼓组总线"
        name="染色"
        options={options}
        selectedId={null}
        onSelect={() => {}}
        state="default"
      />
    );
    expect(markup).toContain("disabled");
    expect(markup).not.toContain("已应用");
  });

  it("V 标记仅 OS 身份：VitMark 渲染 i-vit；ProjectBadge 渲染 i-project", async () => {
    const { VitMark } = await import("../components/VitMark");
    const { ProjectBadge } = await import("../components/ProjectBadge");
    expect(renderToStaticMarkup(<VitMark size={20} state="busy" />)).toContain(
      'href="#i-vit"'
    );
    expect(renderToStaticMarkup(<ProjectBadge size={20} />)).toContain(
      'href="#i-project"'
    );
  });

  it("状态环四态各渲染对应 symbol", async () => {
    const { StatusRing } = await import("../components/StatusRing");
    const map = {
      idle: "i-idle",
      busy: "i-busy",
      done: "i-done",
      error: "i-error",
    } as const;
    for (const [state, symbol] of Object.entries(map)) {
      expect(
        renderToStaticMarkup(<StatusRing state={state as keyof typeof map} size={20} />)
      ).toContain(`href="#${symbol}"`);
    }
  });
});
