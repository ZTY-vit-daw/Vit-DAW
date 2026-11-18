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
 *
 * CONV-RAIL-1（418，B2a）增补：
 * - 左栏 footer（data-entry-rail-foot）固定常驻、跨 surface 不重建：设置按钮
 *   （App 回调打开既有 settings dialog）+ 预留头像槽；不随左栏体列表内容变化。
 * - 收展状态写 localStorage（vit_entry_rail_collapsed），挂载时恢复——DSH 习惯的
 *   重载记忆；收展唯一状态源仍是 WindowPresentation.sidebarOpen（本文件只在挂载
 *   时用它恢复初始值，不改 windowPresentation 协议）。
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

/** 左栏收展持久化键（DSH 习惯：重载后恢复上次收展状态）。 */
export const ENTRY_RAIL_COLLAPSED_STORAGE_KEY = "vit_entry_rail_collapsed";

/** 读取持久化收展状态：true=收起。localStorage 不可用/缺省=展开（fail-open）。 */
export function loadEntryRailCollapsed(): boolean {
  try {
    return window.localStorage.getItem(ENTRY_RAIL_COLLAPSED_STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

/** 写入持久化收展状态（best-effort；存储失败不得阻塞收展交互）。 */
export function saveEntryRailCollapsed(collapsed: boolean): void {
  try {
    window.localStorage.setItem(ENTRY_RAIL_COLLAPSED_STORAGE_KEY, collapsed ? "true" : "false");
  } catch {
    // localStorage 不可用时收展仍按内存状态工作。
  }
}

export interface EntryShellProps {
  /** 当前入口面（launcher/project）——只切内容，不重建左栏容器。 */
  surface: EntryRailSurface;
  /** 左栏 footer 设置按钮回调（App 打开既有 settings dialog）。 */
  onOpenSettings?: () => void;
  /** 右主区：App 原有两个 hidden surface div（+入口设置 dialog）原样放入。 */
  children: ReactNode;
}

export function EntryShell({ surface, onOpenSettings, children }: EntryShellProps) {
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
  // B2a：挂载时恢复上次收展状态（只读一次，避免覆盖用户当次操作）；展开态由默认值承担。
  useEffect(() => {
    if (loadEntryRailCollapsed()) {
      setSidebarOpen(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  // B2a：收展变化写 localStorage（DSH 习惯的重载记忆；失败不阻塞交互）。
  useEffect(() => {
    saveEntryRailCollapsed(collapsed);
  }, [collapsed]);
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
          {/* B2a：footer 固定在左栏底（不随列表内容/surface 变化）——设置常驻入口 +
              预留头像槽。设置打开既有 App settings dialog（onOpenSettings 回调）。 */}
          <footer className="v5-entry-rail-foot" data-entry-rail-foot>
            <button
              type="button"
              className="v5-btn v5-btn--secondary v5-entry-rail-settings"
              data-entry-rail-settings
              disabled={!onOpenSettings}
              onClick={onOpenSettings}
            >
              设置
            </button>
            <span
              className="v5-entry-rail-avatar"
              data-entry-rail-avatar
              aria-hidden="true"
              title="账号头像（预留槽位）"
            />
          </footer>
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
