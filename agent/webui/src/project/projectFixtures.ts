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
  ProjectChatOutcome,
  ProjectContextView,
  ProjectService,
  ProjectThreadItem,
  TutorMaterialLoadOutcome,
  TutorStartOutcome
} from "./projectData";

export interface ProjectFixtureConfig {
  /** 工程上下文（必须带 source:"fixture"，名称带（fixture）标注）。 */
  context?: ProjectContextView;
  /** 发送对话的 fixture 回执文案。 */
  reply?: string;
  /** SHELL-6：发送对话的完整回执载荷（省略时用 reply/默认占位构造）。 */
  chatOutcome?: ProjectChatOutcome;
  /** 发送过的消息台账（SHELL-6 确认卡点击发送断言用；由调用方读）。 */
  sentMessages?: string[];
  /** TUTOR-SIM-2：发送时使用的 conversation_id 台账（断言导师会话寻址）。 */
  sentConversationIDs?: (string | undefined)[];
  /** TUTOR-SIM-2：自动开场回执；省略 = unavailable（fixture 不触网）。 */
  tutorStart?: TutorStartOutcome;
  /** TUTOR-SIM-2：自动开场调用台账（工程路径；断言每工程一次）。 */
  tutorStartCalls?: string[];
  /** TUTOR-SIM-4：材料装载回执；省略 = 成功（fixture track/clip id）。 */
  loadMaterialOutcome?: TutorMaterialLoadOutcome;
  /** TUTOR-SIM-4：材料装载调用台账（会话 id + 材料名；由调用方读）。 */
  loadMaterialCalls?: Array<{ conversationID: string; title: string }>;
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
    async sendMessage(text: string, conversationID?: string): Promise<ProjectChatOutcome> {
      await delay(latency);
      sendCount += 1;
      config.sentMessages?.push(text);
      config.sentConversationIDs?.push(conversationID);
      if (config.chatOutcome) return config.chatOutcome;
      return { text: config.reply ?? `（fixture 回执）收到：「${text}」。` };
    },
    async startTutorSim(projectPath: string): Promise<TutorStartOutcome> {
      // fixture 服务替身不触网：默认返回 unavailable（普通工程语义）。
      await delay(latency);
      config.tutorStartCalls?.push(projectPath);
      if (config.tutorStart) return config.tutorStart;
      return { status: "unavailable", reason: "fixture_tutor_sim_unavailable" };
    },
    async loadTutorMaterial(conversationID: string, title: string): Promise<TutorMaterialLoadOutcome> {
      // fixture 服务替身不触网：回执由配置给定（默认=成功，fixture 证据 id）。
      await delay(latency);
      config.loadMaterialCalls?.push({ conversationID, title });
      if (config.loadMaterialOutcome) return config.loadMaterialOutcome;
      return {
        ok: true,
        alreadyLoaded: false,
        trackID: `fixture-track-${config.loadMaterialCalls?.length ?? 0}`,
        clipID: `fixture-clip-${config.loadMaterialCalls?.length ?? 0}`,
        pluginID: "fixture-plugin-0",
        instrumentAttached: true
      };
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
