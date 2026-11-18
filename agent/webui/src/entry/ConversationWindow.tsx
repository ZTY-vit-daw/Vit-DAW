/*
 * CONV-WINDOW-SHELL-1（428，A 方案）：右侧对话窗口壳（ConversationWindow）。
 *
 * 问题（用户手测）：工程启动/切换时右侧对话流“刷新”——launcher 在 surface≠launcher
 * 时被卸载、project 首次打开新挂载并重拉 context，观感=窗口重建。
 *
 * 本组件把右侧主区抽成常驻窗口壳 + 两个叠放的持久流层：
 * - 窗口壳（data-conversation-window）与已挂载的层在 surface 切换/工程打开/返回时
 *   **同一 DOM 节点不重建**（层只 hidden，不卸载；是否挂载由调用方 mounted 标记决定）；
 * - active 层可见/可交互并播放 180–240ms 入场过渡（opacity + 轻微 translateX；
 *   prefers-reduced-motion 下关闭）；inactive 层 hidden + inert + aria-hidden，
 *   不参与布局、不抢焦点、不可交互；
 * - 各层内部沿用既有 surface frame（launcher/project 自带 titlebar/composer），
 *   本卡不重做统一标题栏（A 方案边界）；滚动位置/输入草稿/加载卡因 DOM 常驻而保留。
 *
 * 边界：无 EntryShell context 的 fixture/gallery 不走本组件（App 的 MainApp 旧路径
 * 与 gallery 渲染树保持原样）；415 windowPresentation 收展时长语言=180ms，本卡过渡
 * 取 200ms 同族。
 */

import { useId, type ReactNode } from "react";
import "./conversationWindow.css";

/** 对话窗口可承载的流层（与 entry surface 一一对应）。 */
export type ConversationLayerKey = "launcher" | "project";

export interface ConversationWindowLayer {
  key: ConversationLayerKey;
  /** 已首次进入（mounted 标记）：true 才渲染层；之后 surface 切换只切 active，不卸载。 */
  mounted: boolean;
  children: ReactNode;
}

export interface ConversationWindowProps {
  /** 当前活动流层。 */
  active: ConversationLayerKey;
  /** 叠放层（顺序=DOM 顺序=launcher 先 project 后）。 */
  layers: ConversationWindowLayer[];
  /** 窗口壳 aria-label（默认「对话窗口」）。 */
  label?: string;
}

export function ConversationWindow({ active, layers, label = "对话窗口" }: ConversationWindowProps) {
  const shellId = useId();
  return (
    <div
      className="v5-conversation-window"
      data-conversation-window
      data-conversation-window-active={active}
      aria-label={label}
      id={shellId}
    >
      {layers.map((layer) => {
        if (!layer.mounted) {
          return null;
        }
        const layerActive = layer.key === active;
        return (
          <section
            key={layer.key}
            className={[
              "v5-conversation-layer",
              layerActive ? "v5-conversation-layer--active" : "v5-conversation-layer--inactive"
            ].join(" ")}
            data-conversation-layer={layer.key}
            data-conversation-layer-active={layerActive ? "true" : "false"}
            hidden={!layerActive}
            inert={!layerActive}
            aria-hidden={!layerActive}
          >
            {layer.children}
          </section>
        );
      })}
    </div>
  );
}

export default ConversationWindow;
