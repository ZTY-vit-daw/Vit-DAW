// GUI-F5：事件轮询器空闲门。忙态（发送中/动作响应中/试听等待/链活/轨迹回合
// 开放）任一为真时，空轮询不累积空闲拍——链活期（GUI-F3 的 agentTurnRunning）
// 终局结果不漏取；全部空闲才按 4 拍（连续失败 3 拍）休眠，回合终态后不会无限轮询。
// B9：free-state chat 链不进 goal/continuations 投影（2026-09-11 mtwwegtp 取证：
// 链执行期轮询休眠、终局一次性补渲染），补 trajectoryLive——客户端轨迹回合只要
// 还有 running/pending 轮次（壳节点开至 trajectory.turn 终态），轮询不休眠。
export interface AgentEventPollBusyFlags {
  isSending?: boolean;
  respondingActionID?: string | null;
  auditionWaiting?: boolean;
  agentTurnRunning?: boolean;
  trajectoryLive?: boolean;
}

export function agentEventPollBusy(flags: AgentEventPollBusyFlags): boolean {
  return Boolean(flags.isSending || flags.respondingActionID || flags.auditionWaiting || flags.agentTurnRunning || flags.trajectoryLive);
}

export function createAgentEventPollIdleGate(config?: { idleThreshold?: number; errorThreshold?: number }) {
  const idleThreshold = config?.idleThreshold ?? 4;
  const errorThreshold = config?.errorThreshold ?? 3;
  let idleTicks = 0;
  return {
    get idleTicks(): number {
      return idleTicks;
    },
    markActive(): void {
      idleTicks = 0;
    },
    // 返回 true 表示达到休眠阈值、应停止轮询
    tickIdle(busy: boolean): boolean {
      if (busy) {
        return false;
      }
      idleTicks += 1;
      return idleTicks >= idleThreshold;
    },
    tickError(busy: boolean): boolean {
      if (busy) {
        return false;
      }
      idleTicks += 1;
      return idleTicks >= errorThreshold;
    },
  };
}
