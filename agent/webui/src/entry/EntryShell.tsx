/*
 * SHELL-ENTRY-RAIL-1（417）：入口壳层 EntryShell——窗口级常驻左栏 + 右主区。
 *
 * 结构（卡片 §1）：
 *   左栏 aside[data-entry-rail]（280px，可收起 180ms）
 *   ├─ 左栏头：Vit 品牌 + 当前面标题（启动页=工程列表 / 工程面=对话流）
 *   │          + 收起/展开按钮（复用 WindowPresentation sidebarOpen/setSidebarOpen）
 *   │          + NativeFocusHint
 *   └─ 左栏体 div[data-entry-rail-slot]：门户容器，内容由当前面 createPortal 注入
 *   右主区 div[data-entry-main]：App 原有两个 hidden surface div（launcher/project）原样放入
 *
 * 不变量：
 * - 收展状态跨 surface 保持（sidebarOpen 挂在 WindowPresentationProvider，不随 branch 卸载）；
 * - surface 切换只换左栏/主区内容，**同一条 data-entry-rail 节点不重建**（卡片 §5）；
 * - fixture/gallery 不渲染 EntryShell → 无 rail context → 消费方保持原内嵌行为。
 *
 * 收起后的展开入口：左栏宽 0（窗口级不占位）时左栏头被裁切，主区左缘保留浮动
 * 展开把手（data-entry-rail-expand），鼠标/键盘仍可展开——不把左栏做成不可逆收起。
 */

import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import { NativeFocusHint, useWindowPresentation } from "../project/loading/windowPresentation";
import { EntryRailProvider, type EntryRailSurface } from "./entryRail";
import "./entryShell.css";

/** 左栏头当前面标题（卡片 §1：启动页=工程列表；工程面=对话流）。 */
const ENTRY_SURFACE_TITLE: Record<EntryRailSurface, string> = {
  launcher: "工程列表",
  project: "对话流"
};

/** 左栏体槽位语义名（SHELL-2 收展契约沿用：启动页=recents；工程面内容 B2 接入）。 */
const ENTRY_SURFACE_SLOT: Record<EntryRailSurface, string> = {
  launcher: "recents",
  project: "chats"
};

export interface EntryShellProps {
  /** 当前入口面（launcher/project）——只切内容，不重建左栏容器。 */
  surface: EntryRailSurface;
  /** 右主区：App 原有两个 hidden surface div（+入口设置 dialog）原样放入。 */
  children: ReactNode;
}

export function EntryShell({ surface, children }: EntryShellProps) {
  // 收展唯一状态源：WindowPresentationProvider.sidebarOpen（旧 launcher 抽屉同源）。
  const { sidebarOpen, setSidebarOpen } = useWindowPresentation();
  const slotId = useId();
  const slotRef = useRef<HTMLDivElement | null>(null);
  // 门户容器要等 DOM 就绪：首帧无 host（消费方不 portal），挂载后 setState 再投影。
  // StrictMode 下 effect 双跑只重复 set 同一节点，无副作用。
  const [host, setHost] = useState<HTMLElement | null>(null);
  useEffect(() => {
    setHost(slotRef.current);
  }, []);
  const collapsed = !sidebarOpen;
  const rail = useMemo(() => ({ host, surface, collapsed }), [host, surface, collapsed]);
  return (
    <EntryRailProvider value={rail}>
      <div className="v5-design v5-entry-shell" data-entry-shell data-entry-rail-surface={surface}>
        <aside
          className="v5-entry-rail"
          data-entry-rail
          data-entry-rail-collapsed={collapsed ? "true" : "false"}
          aria-label="入口左栏"
        >
          <header className="v5-entry-rail-head" data-entry-rail-head>
            <span className="v5-entry-rail-brand" data-entry-rail-brand>
              Vit
            </span>
            <span className="v5-entry-rail-title" data-entry-rail-title>
              {ENTRY_SURFACE_TITLE[surface]}
            </span>
            {/* NativeFocusHint 不重复：launcher 面旧标题栏 hint 在 rail 模式隐藏，由左栏头承担；
                工程面 titlebar 的 hint 是既有渲染断言（render-window 段）锚点，保持原位不复制。 */}
            {surface === "launcher" ? <NativeFocusHint /> : null}
            <button
              type="button"
              className="v5-btn v5-btn--secondary v5-window-sidebar-toggle v5-entry-rail-toggle"
              data-window-sidebar-toggle
              aria-controls={slotId}
              aria-expanded={sidebarOpen}
              aria-label={sidebarOpen ? "收起左栏" : "展开左栏"}
              title={sidebarOpen ? "收起左栏" : "展开左栏"}
              onClick={() => setSidebarOpen(!sidebarOpen)}
            >
              {sidebarOpen ? "收起" : "展开"}
            </button>
          </header>
          {/* 左栏体门户容器：SHELL-2 收展契约沿用（收起时 width 0 + inert + aria-hidden）。 */}
          <div
            ref={slotRef}
            id={slotId}
            className="v5-entry-rail-slot"
            data-entry-rail-slot
            data-window-sidebar-slot={ENTRY_SURFACE_SLOT[surface]}
            data-sidebar-open={sidebarOpen ? "true" : "false"}
            aria-hidden={collapsed}
            inert={collapsed}
          />
        </aside>
        <div className="v5-entry-main" data-entry-main>
          {children}
          {/* 收起态展开把手：左栏 0 宽时左栏头被裁切，浮动把手保证展开路径不丢失。 */}
          {collapsed ? (
            <button
              type="button"
              className="v5-entry-rail-expand"
              data-entry-rail-expand
              aria-label="展开左栏"
              title="展开左栏"
              onClick={() => setSidebarOpen(true)}
            >
              ›
            </button>
          ) : null}
        </div>
      </div>
    </EntryRailProvider>
  );
}

export default EntryShell;
