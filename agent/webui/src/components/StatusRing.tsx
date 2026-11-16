import { Icon, type IconSize } from "./icons";

/** SPEC-V5 §4.2 状态层四态：待机 / 变化中 / 安全完成 / 出错。 */
export type RingState = "idle" | "busy" | "done" | "error";

export interface StatusRingProps {
  state: RingState;
  size?: IconSize;
  className?: string;
}

const RING_SYMBOL: Record<RingState, "i-idle" | "i-busy" | "i-done" | "i-error"> = {
  idle: "i-idle",
  busy: "i-busy",
  done: "i-done",
  error: "i-error",
};

/**
 * 状态环。形状编码不依赖颜色（SPEC-V5 §7.2）：
 * 待机 = 3/4 虚线弧（9s 慢转）、进行中 = 3/4 实线弧（1.6s 快转）、
 * 完成 = 满环 + 勾、出错 = 满环 + 感叹。
 * 颜色按 SPEC §4.2/§4.4：待机中性、进行中深黄线（浅底上黄色只做底色）、
 * 完成 blue header、出错 warning edge；深底容器可用 --v5-control-blue。
 */
export function StatusRing({ state, size = 20, className }: StatusRingProps) {
  const cls = ["v5-ring", `v5-ring--${state}`, className]
    .filter(Boolean)
    .join(" ");
  return <Icon name={RING_SYMBOL[state]} size={size} className={cls} />;
}
