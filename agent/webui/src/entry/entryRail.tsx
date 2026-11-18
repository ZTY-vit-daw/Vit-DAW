/*
 * SHELL-ENTRY-RAIL-1（417）：入口壳左栏 rail 上下文。
 *
 * 背景：live 入口 App 的 launcher/project 两面原本各自成面——启动页抽屉只在
 * launcher 内嵌、工程面没有左列。本卡把「窗口级常驻左栏」提到 EntryShell（App 内、
 * WindowPresentationProvider 之下）：左栏容器跨 surface 常驻，内容由当前面经
 * portal 注入（B1 先「内容切换」；B2 CHAT-STREAM-LIST-1 再接对话流列表/动效）。
 *
 * 分工：
 * - EntryShell 提供 rail 上下文：host=左栏体门户容器 DOM；surface=当前入口面；
 *   collapsed=WindowPresentation.sidebarOpen 取反（收展唯一状态源仍在该 provider）；
 * - 消费方（ConvergedLauncher / ProjectSurface）用 useEntryRail() 判定：
 *   **返回 null 即 fixture/gallery/独立静态渲染（无 EntryShell）**——必须保持原有
 *   内嵌抽屉/标题栏行为逐字不变。
 *
 * 纪律：本 context 只走 React 树内共享，不进 windowPresentation 协议（卡片红线）；
 * 不改 launcherBridge/entrySurfaceBridge 桥协议。
 */

import { createContext, useContext, type ReactNode } from "react";

/** 有常驻左栏的 live 入口面（启动页 launcher / 工程面 project）。 */
export type EntryRailSurface = "launcher" | "project";

export interface EntryRailContextValue {
  /** 左栏体门户容器（EntryShell 挂载后非空；fixture/gallery 无 context）。 */
  host: HTMLElement | null;
  /** 当前入口面——决定左栏内容归属（启动页=工程列表 / 工程面=对话流占位）。 */
  surface: EntryRailSurface;
  /** 左栏是否收起（= !WindowPresentation.sidebarOpen；跨 surface 共享）。 */
  collapsed: boolean;
}

const EntryRailContext = createContext<EntryRailContextValue | null>(null);

export function EntryRailProvider({ value, children }: { value: EntryRailContextValue; children: ReactNode }) {
  return <EntryRailContext.Provider value={value}>{children}</EntryRailContext.Provider>;
}

/**
 * 读取 rail 上下文：无 EntryShell（gallery fixture / 静态渲染 / MainApp surface）
 * 返回 null——消费方据此走原有内嵌路径，行为逐字不变。
 */
export function useEntryRail(): EntryRailContextValue | null {
  return useContext(EntryRailContext);
}
