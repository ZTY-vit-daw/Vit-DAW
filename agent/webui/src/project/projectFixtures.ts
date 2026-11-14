/*
 * UI-PROJECT-1（346）工程内 surface · fixture 服务替身。
 *
 * 作用：gallery / 渲染烟测 / 静态单测的离线服务端 stub。ProjectService 接缝的
 * fixture 实现——它扮演「服务端回执」角色（成功/失败由配置给定），UI 状态机
 * 本身不做任何伪造；红线由 projectData.test.ts 断言。
 *
 * fixture 纪律：所有示例工程名/快照引用由调用方显式传入并带（fixture）标注；
 * 本文件不内置任何示例工程名。
 */

import type {
  ABActionView,
  MacroApplyOutcome,
  ProjectContextView,
  ProjectService,
  ProjectThreadItem
} from "./projectData";

export interface ProjectFixtureConfig {
  /** 工程上下文（必须带 source:"fixture"，名称带（fixture）标注）。 */
  context?: ProjectContextView;
  /** 发送对话的 fixture 回执文案。 */
  reply?: string;
  /** 应用宏的 fixture 回执（成功带 snapshotRef / 失败带 reason）。 */
  applyOutcome?: MacroApplyOutcome;
  /** 撤销是否成功（false 时抛错，用于样例失败态）。 */
  undoFails?: boolean;
  /** A/B 准备回执；省略 = 失败（reason 用 abFailureReason）。 */
  abResult?: { bypassRef: string; processedRef: string };
  abFailureReason?: string;
  /** 模拟延迟（毫秒，默认 0 保证渲染烟测确定性）。 */
  latencyMs?: number;
}

function delay(ms: number): Promise<void> {
  return ms > 0 ? new Promise((resolve) => setTimeout(resolve, ms)) : Promise.resolve();
}

/** 快照引用按宏卡 id + 选项生成（fixture 可追溯，不冒充真实工程快照）。 */
export function fixtureSnapshotRef(macroId: string, optionId: string): string {
  return `fixture-snapshot-${macroId}-${optionId}`;
}

export function createProjectFixtureService(config: ProjectFixtureConfig = {}): ProjectService {
  const latency = config.latencyMs ?? 0;
  let sendCount = 0;
  return {
    async fetchContext(): Promise<ProjectContextView> {
      await delay(latency);
      if (!config.context) throw new Error("fixture 未配置工程上下文（context）");
      return config.context;
    },
    async sendMessage(text: string): Promise<string> {
      await delay(latency);
      sendCount += 1;
      return config.reply ?? `（fixture 回执）收到：「${text}」。`;
    },
    async stopReply(): Promise<void> {
      await delay(latency);
    },
    async applyMacro(macroId: string, optionId: string): Promise<MacroApplyOutcome> {
      await delay(latency);
      if (config.applyOutcome) return config.applyOutcome;
      return {
        ok: true,
        snapshotRef: fixtureSnapshotRef(macroId, optionId),
        time: new Date().toLocaleTimeString("zh-CN", { hour12: false, hour: "2-digit", minute: "2-digit" })
      };
    },
    async undoSnapshot(): Promise<void> {
      await delay(latency);
      if (config.undoFails) throw new Error("fixture：撤销失败（服务端拒绝）");
    },
    async prepareAB(): Promise<{ bypassRef: string; processedRef: string }> {
      await delay(latency);
      if (config.abResult) return config.abResult;
      throw new Error(config.abFailureReason ?? "fixture：A/B 准备失败（未配置 abResult）");
    }
  };
}

/**
 * 从 fixture 线程抽取 AB 条目 id（gallery 样例接线用；找不到返回 null）。
 */
export function firstABItemID(thread: ProjectThreadItem[]): string | null {
  for (const item of thread) {
    if (item.kind === "ab") return item.id;
  }
  return null;
}

/** AB 视图的预览标签（ready 态显示真实引用存在性，不显示引用内容冒充音频）。 */
export function abReadyLabel(view: ABActionView): string {
  if (view.state === "ready" && view.bypassRef && view.processedRef) return "对比 A/B 已就绪";
  if (view.state === "preparing") return "正在准备对比 A/B…";
  return "对比 A/B";
}
