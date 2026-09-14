// TRAJ-IMPL-2（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.1-3 + §7 裁定 A，
// 2026-09-14 用户定稿）：item 工具步标题的**人话映射表**。
//
// 裁定 A=②「映射成人话」：常见族的工具标识符转中文人话（如「已完成 频率关系观察」）；
// **未命中的标识符原样显示**——原样才有取证价值（吞掉或改写成概括词都会让人看不出
// 这一步到底调了什么），并在本文件「待补登记」里记一笔。映射表是活表：先覆盖常见族
// （ccb 观察 / mix_tick / trajectory / audition / approval），其余按前缀族兜底。
//
// 标识符口径（真栈事件实测）：item.started 带 payload.tool（或
// payload.command_raw.tool），item.completed / item.failed 带
// payload.command_name——同一个动作在起止两侧写法不同
// （ccb.observation_catalog 对 ccb_observation_catalog），所以查表前先归一化
// （小写 + 下划线折成点），两种写法命中同一条。

/** 步状态族（人话动词的四个分支；未命中的状态归到最接近的一族） */
export type StepStatusFamily = "running" | "completed" | "failed" | "pending";

export interface StepLabelEntry {
  /** 中文人话短语（动作或对象），与状态动词拼成整句：「已完成 插件搜索」 */
  gloss: string;
  /** 状态专属整句覆盖（可选）：通用拼装读不通的族在这里写整句 */
  titles?: Partial<Record<StepStatusFamily, string>>;
}

export interface StepLabel {
  /** 显示标题：命中映射=人话整句；未命中=标识符原样 */
  title: string;
  /** 是否命中映射表（未命中即「待补登记」的候选） */
  mapped: boolean;
  /** 用作查表的原始标识符（未命中时它就是标题） */
  identifier: string;
}

const STATUS_VERBS: Record<StepStatusFamily, string> = {
  running: "正在执行",
  completed: "已完成",
  failed: "执行失败",
  pending: "等待确认"
};

/**
 * 常见族条目表（键 = 归一化标识符：小写 + 下划线折点）。
 *
 * 待补登记（真栈出现过但本表未覆盖，原样显示；补表时按「动作+对象」写短语）：
 *   - track.* 细分动作（除了 add/delete/mute/solo/volume/pan/rename 外的轨道族）
 *   - plugin.* 细分动作（scan / explain_controls / show_editor / semantic_*）
 *   - project.audio_analysis_*（分析启停）
 *   - 其它未登记族：原样显示即诚实，宁可少映射也不猜语义
 */
export const STEP_LABEL_TABLE: Record<string, StepLabelEntry> = {
  // ── ① ccb 观察族（用户裁定 A 列名族）
  "ccb.observation.request": { gloss: "频率关系观察" },
  "ccb.observation.recorded": { gloss: "频率关系观察" },
  "ccb.observation.observed": { gloss: "频率关系观察" },
  "ccb.observation.catalog": { gloss: "可用观察视图清单" },
  // ── ② mix_tick 混音族
  "mix.tick": { gloss: "混音调整", titles: { pending: "混音调整等待你确认" } },
  "mix.tick.pending": { gloss: "混音调整", titles: { pending: "混音调整等待你确认" } },
  "mix.tick.audition.judgment": { gloss: "试听判定记录" },
  "mix.tick.confirmation.no.candidate": { gloss: "混音确认（未发现候选）" },
  "mix.tick.confirmation.routed": { gloss: "混音确认（已路由）" },
  // ── ③ trajectory 轨迹族（item 侧的轨迹事件）
  "trajectory.turn.started": { gloss: "实验回合开始" },
  "trajectory.turn.completed": { gloss: "实验回合收尾" },
  "trajectory.turn.failed": { gloss: "实验回合失败" },
  "trajectory.turn.stopped": { gloss: "实验回合停止" },
  "trajectory.observation.recorded": { gloss: "轨迹观察记录" },
  "trajectory.intent.framed": { gloss: "实验意图确立" },
  "trajectory.hypothesis.proposed": { gloss: "实验假设提出" },
  "trajectory.round.started": { gloss: "实验轮次开始" },
  "trajectory.intervention.applied": { gloss: "类型化干预应用" },
  "trajectory.user.judgment.requested": { gloss: "试听判定请求" },
  "trajectory.user.judgment.recorded": { gloss: "试听判定记录" },
  // ── ④ audition 试听族
  "audition.prepare": { gloss: "A/B 试听准备" },
  "audition.ready": { gloss: "A/B 试听就绪" },
  "audition.select": { gloss: "A/B 试听选段" },
  "audition.apply.candidate": { gloss: "应用试听候选" },
  "audition.inspect.candidate": { gloss: "检查试听候选" },
  "audition.retain.candidate": { gloss: "保留试听候选" },
  "audition.rollback.candidate": { gloss: "回退试听候选" },
  "audition.status": { gloss: "试听状态查询" },
  "audition.stop": { gloss: "停止试听" },
  "audition.failed": { gloss: "试听失败" },
  // ── ⑤ approval 审批族
  "approval.requested": {
    gloss: "权限确认",
    titles: { pending: "等待你的权限确认", running: "等待你的权限确认", completed: "权限已确认" }
  },
  "approval.granted": { gloss: "权限已确认" },
  "approval.denied": { gloss: "权限被拒绝" },
  // ── 插件族（真栈高频）
  "plugin.search": { gloss: "插件搜索" },
  "plugin.semantic.search": { gloss: "插件语义搜索" },
  "plugin.semantic.build.index": { gloss: "插件语义索引构建" },
  "plugin.semantic.get": { gloss: "插件语义信息读取" },
  "plugin.find": { gloss: "插件查找" },
  "plugin.list": { gloss: "插件清单" },
  "plugin.list.available": { gloss: "可用插件清单" },
  "plugin.load": { gloss: "插件装载" },
  "plugin.load.to.rack": { gloss: "插件装载到机架" },
  "plugin.instantiate": { gloss: "插件实例化" },
  "plugin.delete": { gloss: "插件移除" },
  "plugin.get.parameters": { gloss: "插件参数读取" },
  "plugin.set.parameter": { gloss: "插件参数设置" },
  "plugin.set.params.batch": { gloss: "插件参数批量设置" },
  "plugin.open": { gloss: "插件界面打开" },
  // ── 轨道族（真栈高频）
  "track.add": { gloss: "新建轨道" },
  "track.create": { gloss: "新建轨道" },
  "track.add.audio": { gloss: "导入音频到新轨" },
  "track.delete": { gloss: "删除轨道" },
  "track.list": { gloss: "轨道清单" },
  "track.rename": { gloss: "轨道重命名" },
  "track.mute": { gloss: "轨道静音" },
  "track.solo": { gloss: "轨道独奏" },
  "track.volume": { gloss: "轨道音量" },
  "track.pan": { gloss: "轨道声像" },
  "track.level": { gloss: "轨道电平" },
  // ── 工程族
  "project.state": { gloss: "工程状态读取" },
  "project.structure": { gloss: "工程结构读取" },
  "project.headroom": { gloss: "工程余量读取" }
};

/** 前缀族兜底（长前缀优先）：精确条目未命中时给一个族级人话，仍比裸标识符可读 */
export const STEP_LABEL_PREFIXES: Array<[string, StepLabelEntry]> = [
  ["ccb.observation.", { gloss: "工程观察" }],
  ["ccb.", { gloss: "工程观察" }],
  ["mix.tick", { gloss: "混音调整" }],
  ["trajectory.", { gloss: "实验轨迹记录" }],
  ["audition.", { gloss: "A/B 试听" }],
  ["approval.", { gloss: "权限确认", titles: { pending: "等待你的权限确认" } }],
  ["plugin.", { gloss: "插件操作" }],
  ["track.", { gloss: "轨道操作" }],
  ["project.", { gloss: "工程操作" }],
  ["transport.", { gloss: "走带控制" }]
];

/** 查表键归一化：小写 + 下划线折成点（ccb_observation_catalog → ccb.observation.catalog） */
export function normalizeStepIdentifier(identifier: string): string {
  return String(identifier ?? "").trim().toLowerCase().replace(/_/g, ".");
}

/** 事件状态/步状态 → 状态族（未知状态按事件类型兜底，最后回落 completed） */
export function stepStatusFamily(status: string, eventType = ""): StepStatusFamily {
  const value = String(status ?? "").trim().toLowerCase();
  if (value === "failed" || value === "error" || value === "cancelled" || value === "canceled") {
    return "failed";
  }
  if (value === "running" || value === "in_progress" || value === "started" || value === "active") {
    return "running";
  }
  if (value.startsWith("waiting") || value === "pending" || value === "requested" || value === "waiting_confirmation") {
    return "pending";
  }
  if (value === "completed" || value === "done" || value === "sent" || value === "ok" || value === "succeeded") {
    return "completed";
  }
  const type = String(eventType ?? "").trim().toLowerCase();
  if (type === "item.started" || type.endsWith(".started")) return "running";
  if (type === "item.failed" || type.endsWith(".failed")) return "failed";
  if (type === "approval.requested") return "pending";
  return "completed";
}

/** 条目查表：精确 → 最长前缀 → null（未命中） */
export function stepLabelEntry(identifier: string): StepLabelEntry | null {
  const key = normalizeStepIdentifier(identifier);
  if (!key) return null;
  const exact = STEP_LABEL_TABLE[key];
  if (exact) return exact;
  let best: StepLabelEntry | null = null;
  let bestLength = 0;
  for (const [prefix, entry] of STEP_LABEL_PREFIXES) {
    if (prefix.length > bestLength && key.startsWith(prefix)) {
      best = entry;
      bestLength = prefix.length;
    }
  }
  return best;
}

/**
 * 标识符 → 人话标题。命中：「动词 + 短语」（或条目的状态专属整句）；
 * 未命中：**标识符原样返回**（mapped=false，不吞不改写）。
 */
export function stepLabel(options: { identifier?: string; status?: string; eventType?: string }): StepLabel {
  const identifier = String(options.identifier ?? "").trim();
  if (!identifier) {
    return { title: "", mapped: false, identifier: "" };
  }
  const entry = stepLabelEntry(identifier);
  if (!entry) {
    return { title: identifier, mapped: false, identifier };
  }
  const family = stepStatusFamily(options.status ?? "", options.eventType ?? "");
  const override = entry.titles?.[family];
  if (override) {
    return { title: override, mapped: true, identifier };
  }
  const verb = STATUS_VERBS[family];
  return { title: verb + " " + entry.gloss, mapped: true, identifier };
}
