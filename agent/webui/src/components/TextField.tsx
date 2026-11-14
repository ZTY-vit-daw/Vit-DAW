import { Icon } from "./icons";

/**
 * 输入框 / 对话输入区（SPEC-V5 §8.6 + depthComputed.composer）。
 * 状态：default / focus（浏览器原生 :focus-visible 样式）/ disabled（装载中，
 * 斜纹底 + 提示）/ busy（Vit 回复中，发送键变为 i-stop 停止键）。
 */
export interface TextFieldProps {
  value?: string;
  placeholder?: string;
  disabledHint?: string;
  disabled?: boolean;
  /** Vit 回复中：发送键切换为停止键。 */
  busy?: boolean;
  onSend?: () => void;
  onStop?: () => void;
  onChange?: (value: string) => void;
  className?: string;
}

export function TextField({
  value,
  placeholder,
  disabledHint,
  disabled,
  busy,
  onSend,
  onStop,
  onChange,
  className,
}: TextFieldProps) {
  const cls = ["v5-composer", className].filter(Boolean).join(" ");
  const state = disabled ? "disabled" : busy ? "busy" : "default";
  const sendLabel = busy ? "停止" : "发送";
  return (
    <div className={cls} data-input-state={state}>
      <input
        type="text"
        className="v5-composer-input"
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        aria-label={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !disabled) {
            if (busy) onStop?.();
            else onSend?.();
          }
        }}
      />
      <button
        type="button"
        className="v5-composer-send v5-btn v5-btn--primary"
        disabled={disabled}
        onClick={() => (busy ? onStop?.() : onSend?.())}
        aria-label={sendLabel}
        title={sendLabel}
      >
        <Icon name={busy ? "i-stop" : "i-send"} size={16} />
      </button>
    </div>
  );
}
