import { useRef, useState } from "react";
import { AudioPlayer } from "./AudioPlayer";
import "./widgets.css";

// WIDGET-KIT-1（池序 379）：A/B 对比组合组件（控件收敛报告 §3.3 最新设计）。
// 结构=两个通用 AudioPlayer 实例（A/B）+ 共享比较控制；呈现层复用
// TrajectoryAuditionPanel 的磁带侧标/播放态语义与 audition.ts 的中立 A/B 措辞
// 片段，但**新组件独立上下文**（不挂 AuditionSession，不依赖 TrajectoryState）。
//
// 三红线（卡面）：
//  ① 不伪造旧实验 turn/round 身份：比较上下文只含 compare 自身身份域
//    （contextID/surfaceID/materialRef），normalizeCompareContext 白名单提取，
//    turn_id/round_id/实验身份字段一律不收不传；
//  ② 偏好只记录偏好事件：比较≠裁决——本组件不 import、不调用
//    submitAuditionJudgment / handleAuditionJudgment（agent 侧
//    retain/rollback/settle 混音业务落账路由），结果只经 onCompareEvent 回流，
//    事件携带 preference/reason，无任何裁决语义字段；
//  ③ 异长版本不共轴：两版各自绑定坐标（positionRule="per-side"），各自独立
//    AudioPlayer 实例/进度/时长，无共享时间轴、无跨侧位置映射；互斥播放只做
//    「同刻只响一版」，不做位置同步。
//
// 采用版本（adoption）不在比较组件内：混音调用方按原任务契约处理保留/回滚，
// 教学调用方明确采用时另行走自己的动作（收敛报告 §3.3 迁移边界）。

export type ComparePreference = "a" | "b" | "neither" | "unsure";

/** 比较自身上下文（红线①：无 turn/round/实验身份域） */
export type CompareContext = {
  contextID: string;
  surfaceID?: string;
  materialRef?: string;
};

export type CompareSide = "a" | "b";

export type CompareEvent =
  | {
      widget: "compare_pair";
      kind: "audition";
      side: CompareSide;
      contextID: string;
      positionRule: "per-side";
      at: string;
    }
  | {
      widget: "compare_pair";
      kind: "preference";
      preference: ComparePreference;
      reason: string;
      contextID: string;
      positionRule: "per-side";
      at: string;
    }
  | {
      widget: "compare_pair";
      kind: "reason";
      preference: ComparePreference | "";
      reason: string;
      contextID: string;
      positionRule: "per-side";
      at: string;
    };

/** 位置规则常量：异长版本各自绑定坐标，不强行共轴（红线③） */
export const COMPARE_POSITION_RULE = "per-side" as const;

export const COMPARE_PREFERENCE_LABELS: Record<ComparePreference, string> = {
  a: "更喜欢 A",
  b: "更喜欢 B",
  neither: "都不喜欢",
  unsure: "不确定"
};

export const COMPARE_PREFERENCE_VALUES: ComparePreference[] = ["a", "b", "neither", "unsure"];

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim();
}

function nowISO(): string {
  try {
    return new Date().toISOString();
  } catch {
    return "";
  }
}

/**
 * 红线①：比较上下文白名单提取——只收 compare 自身身份域（contextID/
 * surface_id/material_ref），调用方原始声明里的 turn_id/round_id/experiment
 * 等实验身份字段一律丢弃；缺 contextID 时生成组件自有身份（compare_pair-*），
 * 绝不借用任何旧实验会话键。
 */
export function normalizeCompareContext(raw: unknown, fallbackID = ""): CompareContext {
  const row = raw && typeof raw === "object" && !Array.isArray(raw) ? (raw as Record<string, unknown>) : {};
  const contextID = text(row.context_id ?? row.contextID) || fallbackID || `compare_pair-${Math.random().toString(36).slice(2, 10)}`;
  const surfaceID = text(row.surface_id ?? row.surfaceID);
  const materialRef = text(row.material_ref ?? row.materialRef);
  return surfaceID || materialRef ? { contextID, surfaceID, materialRef } : { contextID };
}

/** 试听事件：材料身份 + 真实播放状态（catalog §3 audio_player 行回流面） */
export function compareAuditionEvent(context: CompareContext, side: CompareSide): CompareEvent {
  return {
    widget: "compare_pair",
    kind: "audition",
    side,
    contextID: context.contextID,
    positionRule: COMPARE_POSITION_RULE,
    at: nowISO()
  };
}

/** 偏好事件（红线②）：只记录偏好与理由，无裁决/落账语义字段 */
export function comparePreferenceEvent(context: CompareContext, preference: ComparePreference, reason = ""): CompareEvent {
  return {
    widget: "compare_pair",
    kind: "preference",
    preference,
    reason: reason.trim(),
    contextID: context.contextID,
    positionRule: COMPARE_POSITION_RULE,
    at: nowISO()
  };
}

/** 理由事件：随偏好一起回流（偏好未选时 preference 为空串，不代选） */
export function compareReasonEvent(context: CompareContext, preference: ComparePreference | "", reason: string): CompareEvent {
  return {
    widget: "compare_pair",
    kind: "reason",
    preference,
    reason: reason.trim(),
    contextID: context.contextID,
    positionRule: COMPARE_POSITION_RULE,
    at: nowISO()
  };
}

export type CompareSideMedia = {
  label: string;
  artifactID?: string;
  src?: string;
  /** 调用方声明的时长参考（秒）：异长版本各自绑定坐标的披露面（红线③） */
  durationHint?: number;
};

/**
 * A/B 对比组合：两个通用播放器实例 + 共享比较控制。
 * 播放互斥（同刻只响一版，切侧 bump 对侧 pauseNonce）；偏好四态 + 理由输入；
 * 事件只经 onCompareEvent 回流（试听/偏好/理由），不路由任何裁决接口。
 */
export function ComparePair({
  title = "A/B 对比",
  a,
  b,
  context,
  layout = "column",
  preference: controlledPreference,
  reason: controlledReason,
  reasonEnabled = true,
  disabled = false,
  onCompareEvent,
  onPreference,
  onReason
}: {
  title?: string;
  a: CompareSideMedia;
  b: CompareSideMedia;
  /** 比较自身上下文（红线①：组件不收也不造实验身份） */
  context?: CompareContext;
  /** row=左右两列（窄窗自动单列，CSS 断点） */
  layout?: "row" | "column";
  /** 受控偏好（卡内挂载时由字段值驱动）；缺省用组件内部态 */
  preference?: ComparePreference | "";
  /** 受控理由（卡内挂载时由关联理由字段驱动）；undefined=组件内部态 */
  reason?: string;
  /** false=不渲染理由输入（卡内未声明理由字段时） */
  reasonEnabled?: boolean;
  disabled?: boolean;
  onCompareEvent?: (event: CompareEvent) => void;
  /** 偏好写回缝（卡内挂载：写回字段 value；独立使用可缺省） */
  onPreference?: (preference: ComparePreference) => void;
  /** 理由写回缝（卡内挂载：写回关联理由字段；独立使用可缺省） */
  onReason?: (reason: string) => void;
}) {
  const contextRef = useRef<CompareContext | null>(null);
  if (context && contextRef.current?.contextID !== context.contextID) {
    contextRef.current = context;
  }
  const resolvedContext = context ?? contextRef.current ?? normalizeCompareContext(null);
  const [innerPreference, setInnerPreference] = useState<ComparePreference | "">("");
  const [innerReason, setInnerReason] = useState("");
  const preference = controlledPreference !== undefined ? controlledPreference : innerPreference;
  const reason = controlledReason !== undefined ? controlledReason : innerReason;
  // 播放互斥：记录当前在响的侧；切到另一侧时 bump 对侧 pauseNonce
  const [activeSide, setActiveSide] = useState<CompareSide | "">("");
  const [pauseNonces, setPauseNonces] = useState<{ a: number; b: number }>({ a: 0, b: 0 });
  const sides: Array<{ side: CompareSide; media: CompareSideMedia }> = [
    { side: "a", media: a },
    { side: "b", media: b }
  ];

  const emit = (event: CompareEvent) => {
    onCompareEvent?.(event);
  };

  const handlePlaybackChange = (side: CompareSide, playing: boolean) => {
    if (!playing) return;
    if (activeSide !== side) {
      setActiveSide(side);
      const other: CompareSide = side === "a" ? "b" : "a";
      setPauseNonces((current) => ({ ...current, [other]: current[other] + 1 }));
      emit(compareAuditionEvent(resolvedContext, side));
    } else {
      emit(compareAuditionEvent(resolvedContext, side));
    }
  };

  const choosePreference = (value: ComparePreference) => {
    if (disabled) return;
    if (controlledPreference === undefined) {
      setInnerPreference(value);
    }
    onPreference?.(value);
    emit(comparePreferenceEvent(resolvedContext, value, reason));
  };

  const updateReason = (value: string) => {
    onReason?.(value);
    if (controlledReason === undefined) {
      setInnerReason(value);
    }
  };

  const submitReason = () => {
    if (disabled) return;
    const value = reason.trim();
    if (value === "") return;
    emit(compareReasonEvent(resolvedContext, preference, value));
  };

  return (
    <div
      className="widget-compare"
      data-widget-compare-pair={title.trim() || "A/B 对比"}
      data-compare-context={resolvedContext.contextID}
      data-compare-layout={layout}
      data-compare-position-rule={COMPARE_POSITION_RULE}
      data-compare-disabled={disabled ? "true" : "false"}
    >
      <div className="widget-compare-head">
        <span className="widget-compare-title" data-compare-title={title.trim() || "A/B 对比"}>{title.trim() || "A/B 对比"}</span>
        <span className="widget-compare-rule">切换只影响播放，不改你的作品；两版进度各自独立</span>
      </div>
      <div className="widget-compare-sides">
        {sides.map(({ side, media }) => (
          <div className="widget-compare-side" key={side} data-compare-side={side} data-compare-active-side={activeSide === side ? "true" : "false"}>
            <AudioPlayer
              title={`${side.toUpperCase()} · ${media.label || side.toUpperCase()}`}
              artifactID={media.artifactID}
              src={media.src}
              expandable={false}
              durationHint={media.durationHint}
              pauseNonce={pauseNonces[side]}
              onPlaybackChange={(playing) => handlePlaybackChange(side, playing)}
            />
          </div>
        ))}
      </div>
      <div className="widget-compare-controls" role="group" aria-label="比较偏好">
        {COMPARE_PREFERENCE_VALUES.map((value) => (
          <button
            key={value}
            type="button"
            className={`widget-compare-pref ${preference === value ? "selected" : ""}`}
            data-compare-preference={value}
            aria-pressed={preference === value}
            disabled={disabled}
            onClick={() => choosePreference(value)}
          >
            {COMPARE_PREFERENCE_LABELS[value]}
          </button>
        ))}
        {reasonEnabled && (
          <input
            type="text"
            className="widget-compare-reason"
            data-compare-reason="true"
            placeholder="说说理由（回车提交）"
            aria-label="比较理由（回车提交）"
            value={reason}
            disabled={disabled}
            onChange={(event) => updateReason(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                submitReason();
              }
            }}
            onBlur={() => submitReason()}
          />
        )}
      </div>
    </div>
  );
}
