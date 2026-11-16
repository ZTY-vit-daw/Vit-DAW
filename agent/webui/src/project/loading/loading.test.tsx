/*
 * UI-LOADING-1（347）装载状态卡 / 装载门静态渲染测试（renderToStaticMarkup）。
 *
 * fixture 纪律（卡面）：工程名均带（fixture）标注或为通用句式，不冒充真实工程。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { LoadingCard } from "./LoadingCard";
import { ProjectLoadingGate, loadingProjectNameFromUIState } from "./ProjectLoadingGate";
import { emptyLoadingView, type LoadingCardView } from "./loadingData";

function view(overrides: Partial<LoadingCardView>): LoadingCardView {
  return { ...emptyLoadingView(), title: "测试工程甲（fixture）", ...overrides };
}

const STEPS = [
  { id: "s1", label: "已读取工程文件", status: "done" as const, duration: "0.4s" },
  { id: "s2", label: "已恢复轨道", status: "done" as const },
  { id: "s3", label: "正在连接音频设备", status: "current" as const }
];

describe("LoadingCard DOM", () => {
  it("loading 三步样张（SPEC P2）：标题/磁带/≤3 步/当前步 chip/输入禁用声明", () => {
    const markup = renderToStaticMarkup(
      <LoadingCard view={view({ phase: "loading", steps: STEPS, progress: null })} />
    );
    expect(markup).toContain('data-loading-card');
    expect(markup).toContain('data-loading-phase="loading"');
    expect(markup).toContain('data-loading-input-disabled="true"');
    expect(markup).toContain("正在打开《测试工程甲（fixture）》");
    expect(markup).toContain("完成后可继续输入");
    expect(markup).toContain("已读取工程文件");
    expect(markup).toContain("已恢复轨道");
    expect(markup).toContain("正在连接音频设备");
    expect(markup).toContain('data-loading-step-status="current"');
    expect(markup).toContain('data-loading-step-status="done"');
    // 磁带在场（tapeSVG 结构）+ 待机帧带量（未知进度不伪造）
    expect(markup).toContain('data-loading-tape');
    expect(markup).toContain("<svg");
    expect(markup).toContain('data-loading-tape-l="1.00"');
    expect(markup).toContain('data-loading-tape-r="0.20"');
    // 步骤轨迹 ≤3（SPEC §8.2）
    expect((markup.match(/data-loading-step-status=/g) ?? []).length).toBe(3);
  });

  it("有真实进度时磁带带量推进（p=0.64 → L=0.49 R=0.71）", () => {
    const markup = renderToStaticMarkup(
      <LoadingCard view={view({ phase: "loading", steps: STEPS, progress: 0.64 })} />
    );
    expect(markup).toContain('data-loading-tape-l="0.49"');
    expect(markup).toContain('data-loading-tape-r="0.71"');
  });

  it("完成收一行回执（蓝）+轨迹可展开（details）；无进度条元素", () => {
    const markup = renderToStaticMarkup(
      <LoadingCard view={view({ phase: "completed", steps: STEPS, resultNote: "3 条轨道已恢复" })} />
    );
    expect(markup).toContain('data-loading-phase="completed"');
    expect(markup).toContain("已打开《测试工程甲（fixture）》");
    expect(markup).toContain("3 条轨道已恢复");
    expect(markup).toContain("<details");
    expect(markup).toContain("装载轨迹");
    expect(markup).toContain('data-loading-input-disabled="false"');
    expect(markup).not.toContain("v5-progress-track"); // 进度只用磁带带量一种表达
  });

  it("错误路径（必测）：failed 写明原因+重试按钮；interrupted 黄态+重新打开入口", () => {
    const failed = renderToStaticMarkup(
      <LoadingCard
        view={view({ phase: "failed", failureReason: "音频设备被其他程序占用。" })}
        onRetry={() => undefined}
      />
    );
    expect(failed).toContain('data-loading-phase="failed"');
    expect(failed).toContain("音频设备被其他程序占用。");
    expect(failed).toContain('data-loading-retry');
    expect(failed).toContain("重试");
    expect(failed).toContain('data-loading-input-disabled="false"');

    const interrupted = renderToStaticMarkup(
      <LoadingCard
        view={view({ phase: "interrupted", failureReason: "宿主窗口被关闭，装载中断。" })}
        onRetry={() => undefined}
      />
    );
    expect(interrupted).toContain('data-loading-phase="interrupted"');
    expect(interrupted).toContain("宿主窗口被关闭，装载中断。");
    expect(interrupted).toContain("重新打开");
  });

  it("无 onRetry 时错误卡不放恢复按钮（不放假按钮）；无倒计时/剩余文案", () => {
    const markup = renderToStaticMarkup(
      <LoadingCard view={view({ phase: "failed", failureReason: "x。" })} />
    );
    expect(markup).not.toContain('data-loading-retry');
    expect(markup).not.toContain("预计");
    expect(markup).not.toContain("剩余");
  });

  it("磁带氛围色默认中性 [200,200,196]（无身份色外溢——fill 是 62% 透明度壳色）", () => {
    const markup = renderToStaticMarkup(<LoadingCard view={view({ phase: "loading" })} />);
    expect(markup).toContain("rgba(200,200,196,0.62)");
  });
});

describe("ProjectLoadingGate DOM（fixture 模式，零网络）", () => {
  it("probe=error：装载卡 failed 态+重试；children 不渲染（输入禁用语义）", () => {
    const markup = renderToStaticMarkup(
      <ProjectLoadingGate fixture={{ probe: "error", errorText: "fixture：连不上 Vit agent。" }}>
        <div data-probe-child>工程内主窗</div>
      </ProjectLoadingGate>
    );
    expect(markup).toContain('data-loading-gate-state="error"');
    expect(markup).toContain('data-loading-phase="failed"');
    expect(markup).toContain("fixture：连不上 Vit agent。");
    expect(markup).toContain("重试");
    expect(markup).not.toContain('data-probe-child');
    expect(markup).toContain('data-fixture="loading-gate"');
  });

  it("probe=ok：children 渲染（输入恢复）；装载卡退场", () => {
    const markup = renderToStaticMarkup(
      <ProjectLoadingGate fixture={{ probe: "ok" }}>
        <div data-probe-child>工程内主窗</div>
      </ProjectLoadingGate>
    );
    expect(markup).toContain('data-loading-gate-state="ok"');
    expect(markup).toContain('data-probe-child');
    expect(markup).not.toContain('data-loading-card');
  });

});

describe("loadingProjectNameFromUIState", () => {
  it("project_path 尾段为工程名（与 346 projectService 同语义：尾段不去扩展名）；无 path/非字符串返回空串", () => {
    expect(loadingProjectNameFromUIState({ project: { project_path: "D:/fixtures/测试工程甲.vit" } })).toBe("测试工程甲.vit");
    expect(loadingProjectNameFromUIState({ project: { project_path: "" } })).toBe("");
    expect(loadingProjectNameFromUIState({ project: {} })).toBe("");
    expect(loadingProjectNameFromUIState(null)).toBe("");
  });
});
