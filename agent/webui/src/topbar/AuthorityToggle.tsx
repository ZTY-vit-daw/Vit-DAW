// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx 权限下拉族出域（Composer 挂载点）。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { ChevronDown, ShieldCheck } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AuthorityMode } from "../types";

const authorityModeOptions: Array<{ id: AuthorityMode; label: string; hint: string }> = [
  { id: "manual_confirmation", label: "普通确认", hint: "每步可逆动作先经你确认再执行" },
  { id: "full_project_access", label: "完全访问", hint: "边界内自主连续执行，无需逐项确认" }
];

// UX-1：权限切换被 409 占用类拒绝时呈现人话原因。api 层抛出的 Error 只带
// message，故以服务端错误码与错误文案双标记识别（文案见 turn_control.go
// handleAuthorityMode）；其它错误原样透传不清吞，正常切换返回 null 不提示。
const authorityBlockedMarkers = [
  "authority_mode_change_blocked",
  "stop_turn_prevents_new_action",
  "authority mode cannot change while an Agent Turn is running"
];

export function authoritySwitchNotice(outcome: { ok: boolean; error?: unknown }): string | null {
  if (outcome.ok) {
    return null;
  }
  const raw = outcome.error instanceof Error ? outcome.error.message : String(outcome.error ?? "");
  if (authorityBlockedMarkers.some((marker) => raw.includes(marker))) {
    return "任务占用工程面中，结束后可切换";
  }
  return raw || "权限模式切换失败";
}

// GUI-F3（2026-09-04 用户第三笔裁定）：权限切换是点开选择的下拉选择器
// （对齐 ZCode 模型选择器形态），不是两档左右分段开关。收起态显示当前档
// 胶囊键，点开浮出选项菜单（说明文案 + 选中态）；锁定态禁用并说明原因。
export function AuthorityToggle({
  authorityMode,
  authorityLocked,
  onAuthorityModeChange
}: {
  authorityMode: AuthorityMode;
  /** authorityBusy || agentTurnRunning：回合进行中锁定档位（服务端权威规则：回合进行中拒切换） */
  authorityLocked: boolean;
  onAuthorityModeChange: (mode: AuthorityMode) => void;
}) {
  const [open, setOpen] = useState(false);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const current = authorityModeOptions.find((option) => option.id === authorityMode) ?? authorityModeOptions[0];

  useEffect(() => {
    if (!open) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!hostRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return (
    <div className="authority-select" ref={hostRef}>
      <button
        type="button"
        className={`authority-select-button${open ? " active" : ""}`}
        disabled={authorityLocked}
        aria-haspopup="menu"
        aria-expanded={open}
        title={authorityLocked ? "任务运行中暂不能切换权限；回合结束后可切换" : "选择工程动作的执行权限"}
        onClick={() => setOpen((current) => !current)}
      >
        <ShieldCheck size={15} />
        <span>{current.label}</span>
        <ChevronDown size={14} className={open ? "flip" : ""} />
      </button>
      {open && (
        <div className="authority-menu" role="menu" aria-label="权限档位">
          {authorityModeOptions.map((option) => (
            <button
              key={option.id}
              type="button"
              role="menuitemradio"
              aria-checked={option.id === authorityMode}
              onClick={() => {
                setOpen(false);
                if (option.id !== authorityMode) {
                  onAuthorityModeChange(option.id);
                }
              }}
            >
              <ShieldCheck size={16} />
              <span className="authority-option-text">
                <strong>{option.label}</strong>
                <small>{option.hint}</small>
              </span>
              <i className={option.id === authorityMode ? "on" : ""} />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
