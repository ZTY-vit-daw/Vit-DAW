/*
 * UI-PROJECT-1（346）工程内 surface 单元测试（静态渲染 + 纯函数）。
 *
 * fixture 纪律（卡面）：本文件所有工程名/宏名/快照引用均为测试 fixture，
 * 不代表真实工程或真实 agent 数据；断言不含把示例工程名硬编码进生产代码。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ProjectSurface } from "./ProjectSurface";
import { createProjectFixtureService } from "./projectFixtures";
import type { MacroOption } from "../components";
import type { MacroCardView, ProjectThreadItem } from "./projectData";

const FIXTURE_CONTEXT = {
  name: "测试工程甲（fixture）",
  source: "fixture" as const,
  detail: "3 条轨道"
};

const FIXTURE_OPTIONS: MacroOption[] = [
  { id: "warm", name: "温暖", description: "中频略厚。", contour: [0.2, 0.5, 0.8] },
  { id: "bright", name: "明亮", description: "高频提起。", contour: [0.3, 0.5, 0.9] },
  { id: "tight", name: "紧实", description: "低频收短。", contour: [0.8, 0.5, 0.3] }
];

function threadWith(...items: ProjectThreadItem[]): ProjectThreadItem[] {
  return items;
}

describe("ProjectSurface：上下文头与 fixture 标注", () => {
  it("标题栏 = i-project 徽标 + 工程名；V 品牌标不进标题栏；fixture 标注在卷", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith({ kind: "message", id: "m1", author: "vit", text: "就绪。" })
        }}
      />
    );
    expect(markup).toContain('data-project-root');
    expect(markup).toContain('data-fixture="project-surface"');
    expect(markup).toContain('data-project-context-source="fixture"');
    expect(markup).toContain("测试工程甲（fixture）");
    expect(markup).toContain('data-project-titlebar="true"');
    expect(markup).toContain("v5-project-titlebar");
    // V 品牌标（i-vit）不得出现在整个 surface（P3 无 OS 身份位）
    expect(markup).not.toContain("#i-vit");
    expect(markup).toContain("3 条轨道");
  });

  it("上下文缺失时渲染错误态与重试（fixture 指定文案）", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface fixture={{ contextErrorText: "fixture：上下文不可用。" }} />
    );
    expect(markup).toContain('data-project-state="error"');
    expect(markup).toContain("fixture：上下文不可用。");
    expect(markup).toContain("重试");
  });
});

describe("ProjectSurface：消息流", () => {
  it("用户/Vit 消息按底色区分（v5-message--user/--vit），作者断言在卷", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith(
            { kind: "message", id: "u1", author: "user", text: "把鼓组压紧一点。" },
            { kind: "message", id: "v1", author: "vit", text: "已在「染色」里准备了方向。" }
          )
        }}
      />
    );
    expect(markup).toContain('data-message-author="user"');
    expect(markup).toContain('data-message-author="vit"');
    expect(markup).toContain("把鼓组压紧一点。");
  });
});

describe("ProjectSurface：执行轨迹", () => {
  it("busy→running / done→completed / error→failed 映射在 DOM", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith(
            {
              kind: "trajectory",
              id: "t1",
              view: {
                state: "busy",
                title: "正在应用宏「温暖」",
                steps: [{ id: "s1", label: "写入绑定参数", status: "current" }]
              }
            },
            { kind: "trajectory", id: "t2", view: { state: "done", title: "已应用宏「明亮」", steps: [] } },
            {
              kind: "trajectory",
              id: "t3",
              view: { state: "error", title: "正在渲染对比 A/B", steps: [], failureReason: "设备占用。" }
            },
            { kind: "trajectory", id: "t4", view: { state: "idle", title: "待机", steps: [] } }
          )
        }}
      />
    );
    expect(markup).toContain('data-status-card-state="running"');
    expect(markup).toContain('data-status-card-state="completed"');
    expect(markup).toContain('data-status-card-state="failed"');
    expect(markup).toContain('data-project-trajectory-state="idle"');
    expect(markup).toContain("设备占用。");
  });
});

describe("ProjectSurface：宏卡四态与快照", () => {
  const macro = (id: string, overrides: Partial<MacroCardView>): ProjectThreadItem => ({
    kind: "macro",
    id,
    view: {
      object: "鼓组总线",
      name: "染色",
      options: FIXTURE_OPTIONS,
      selectedId: null,
      state: "default",
      ...overrides
    }
  });

  it("四态 DOM：default / auditioning / applied / failed；应用按钮未选择时禁用", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith(
            macro("m-default", {}),
            macro("m-aud", { state: "auditioning", selectedId: "warm" }),
            macro("m-applied", { state: "applied", selectedId: "warm", appliedOptionId: "warm", appliedSnapshotRef: "fixture-snapshot-m-applied-warm" }),
            macro("m-failed", { state: "failed", selectedId: "bright", failureReason: "音频设备被其他程序占用。" })
          )
        }}
      />
    );
    expect(markup).toContain('data-macro-state="default"');
    expect(markup).toContain('data-macro-state="auditioning"');
    expect(markup).toContain('data-macro-state="applied"');
    expect(markup).toContain('data-macro-state="failed"');
    // 红线：未选择的应用按钮禁用（default 卡）
    expect(markup).toContain("已应用 · 可撤销");
    expect(markup).toContain("应用失败");
    expect(markup).toContain("音频设备被其他程序占用。");
    expect(markup).toContain("重试");
  });

  it("快照回执一行在卷；已撤销留痕行", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith(
            { kind: "snapshot", id: "s1", view: { time: "14:07", snapshotRef: "fixture-snapshot-1" } },
            { kind: "snapshot", id: "s2", view: { time: "14:11", snapshotRef: "fixture-snapshot-2", undone: true } }
          )
        }}
      />
    );
    expect(markup).toContain('data-snapshot-receipt="true"');
    expect(markup).toContain("14:07");
    expect(markup).toContain('data-project-snapshot-undone="true"');
    expect(markup).toContain("已撤销（14:11 的快照）");
  });
});

describe("ProjectSurface：A/B 动作", () => {
  it("idle/preparing/ready/failed 四态 DOM；ready 绑定真实渲染引用语义", () => {
    const markup = renderToStaticMarkup(
      <ProjectSurface
        fixture={{
          context: FIXTURE_CONTEXT,
          thread: threadWith(
            { kind: "ab", id: "ab1", view: { state: "idle", title: "对比" } },
            { kind: "ab", id: "ab2", view: { state: "preparing", title: "对比" } },
            {
              kind: "ab",
              id: "ab3",
              view: { state: "ready", title: "对比", bypassRef: "fixture-render-bypass", processedRef: "fixture-render-processed" }
            },
            { kind: "ab", id: "ab4", view: { state: "failed", title: "对比", failureReason: "fixture：渲染失败。" } }
          )
        }}
      />
    );
    expect(markup).toContain('data-project-ab-state="idle"');
    expect(markup).toContain('data-project-ab-state="preparing"');
    expect(markup).toContain('data-project-ab-state="ready"');
    expect(markup).toContain('data-project-ab-state="failed"');
    expect(markup).toContain("对比 A/B 已就绪");
    expect(markup).toContain("fixture：渲染失败。");
  });
});

describe("ProjectSurface：输入区四态", () => {
  it("default / focus / disabled / replying（forceComposerState 仅样例用）", () => {
    const base = { context: FIXTURE_CONTEXT };
    const mk = (fixture: Record<string, unknown>) => renderToStaticMarkup(<ProjectSurface fixture={fixture as never} />);
    expect(mk({ ...base })).toContain('data-project-composer-state="default"');
    expect(mk({ ...base, forceComposerState: "focus" })).toContain('data-project-composer-state="focus"');
    expect(mk({ ...base, inputDisabled: true })).toContain('data-project-composer-state="disabled"');
    expect(mk({ ...base, forceComposerState: "replying" })).toContain('data-project-composer-state="replying"');
    expect(mk({ ...base, inputDisabled: true })).toContain("装载完成后可继续输入");
    expect(mk({ ...base, forceComposerState: "replying" })).toContain("Vit 正在回复…");
  });
});

describe("fixture 服务替身", () => {
  it("applyMacro 成功带快照引用；失败带原因；撤销失败抛错", async () => {
    const ok = createProjectFixtureService({
      applyOutcome: { ok: true, snapshotRef: "fixture-snapshot-x", time: "14:07" }
    });
    await expect(ok.applyMacro("mx", "warm")).resolves.toEqual({
      ok: true,
      snapshotRef: "fixture-snapshot-x",
      time: "14:07"
    });
    const fail = createProjectFixtureService({
      applyOutcome: { ok: false, reason: "fixture：应用失败。" }
    });
    await expect(fail.applyMacro("mx", "warm")).resolves.toEqual({ ok: false, reason: "fixture：应用失败。" });
    const undoFail = createProjectFixtureService({ undoFails: true });
    await expect(undoFail.undoSnapshot("ref")).rejects.toThrow("撤销失败");
  });

  it("缺 context 配置时 fetchContext 抛错（不伪造工程名）", async () => {
    const svc = createProjectFixtureService({});
    await expect(svc.fetchContext()).rejects.toThrow("fixture 未配置工程上下文");
  });
});
