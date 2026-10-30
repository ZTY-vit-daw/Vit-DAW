import { useEffect, useState } from "react";
import { isConsumedInteractionStatus } from "./interactionGuard";
import type { JsonRecord } from "./types";

// 通用声明式交互卡控件集（FE-INTERACTION-CARD-KIT-1）。契约：interaction
// 的 payload 携带 card = { schema: "interaction_card.v1", title, fields, actions }，
// 其中 fields: {id, type: text|number|range|select, label, value, min, max,
// step, unit, description, required, options: [{value, label}]}，
// actions: {action_id, label, style: primary|secondary|danger}。渲染器只按
// 声明画卡——新增卡面不再需要 webui 场景分支（编舞确认卡为首个迁移卡）。
// 未声明 card 的既有卡继续走原分支，本组件是增不是替换。

export type InteractionCardField = {
  id: string;
  type: "text" | "number" | "range" | "select";
  label: string;
  value: string;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  description?: string;
  required?: boolean;
  options: Array<{ value: string; label: string }>;
};

export type InteractionCardAction = {
  action_id: string;
  label: string;
  style: "primary" | "secondary" | "danger";
};

export type InteractionCardData = {
  schema: string;
  title: string;
  fields: InteractionCardField[];
  actions: InteractionCardAction[];
};

function textValue(value: unknown, fallback = ""): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return String(value);
  }
  return fallback;
}

function asRecord(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as JsonRecord) : {};
}

function firstArray(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function finiteNumber(value: unknown): number | undefined {
  const parsed = typeof value === "number" ? value : Number(textValue(value).trim());
  return Number.isFinite(parsed) ? parsed : undefined;
}

function truthy(value: unknown): boolean {
  return value === true || value === "true";
}

const FIELD_TYPES = new Set(["text", "number", "range", "select"]);
const ACTION_STYLES = new Set(["primary", "secondary", "danger"]);

// declarativeCardFromAction 提取并校验声明式契约：缺 card、无 action、字段
// 缺 id/类型未知均判非声明卡（返回 null 落回既有渲染分支）；未知 action
// style 收敛为 secondary（渲染降级不失败）。
export function declarativeCardFromAction(action: JsonRecord): InteractionCardData | null {
  const payload = asRecord(action.payload ?? action.data);
  const rawCard = asRecord(payload.card);
  if (Object.keys(rawCard).length === 0) {
    return null;
  }
  const rawActions = firstArray(rawCard.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  if (rawActions.length === 0) {
    return null;
  }
  const actions: InteractionCardAction[] = rawActions
    .map((raw) => {
      const actionID = textValue(raw.action_id ?? raw.id).trim();
      if (!actionID) {
        return null;
      }
      const style = textValue(raw.style).trim().toLowerCase();
      return {
        action_id: actionID,
        label: textValue(raw.label).trim() || actionID,
        style: (ACTION_STYLES.has(style) ? style : "secondary") as InteractionCardAction["style"]
      };
    })
    .filter((item): item is InteractionCardAction => item !== null);
  if (actions.length === 0) {
    return null;
  }
  const fields = firstArray(rawCard.fields).map(asRecord).filter((item) => Object.keys(item).length > 0).map((raw) => {
    const type = textValue(raw.type).trim().toLowerCase();
    const fieldID = textValue(raw.id).trim();
    return {
      id: fieldID || `field-${Math.random().toString(36).slice(2)}`,
      type: (FIELD_TYPES.has(type) ? type : "text") as InteractionCardField["type"],
      label: textValue(raw.label).trim() || fieldID,
      value: textValue(raw.value),
      min: finiteNumber(raw.min),
      max: finiteNumber(raw.max),
      step: finiteNumber(raw.step),
      unit: textValue(raw.unit).trim(),
      description: textValue(raw.description).trim(),
      required: truthy(raw.required),
      options: firstArray(raw.options).map(asRecord).filter((item) => Object.keys(item).length > 0).map((option) => ({
        value: textValue(option.value ?? option.id ?? option.key ?? option.label).trim(),
        label: textValue(option.label ?? option.title ?? option.name ?? option.value ?? option.id).trim()
      })).filter((option) => option.value !== "")
    };
  });
  return {
    schema: textValue(rawCard.schema).trim() || "interaction_card",
    title: textValue(rawCard.title).trim(),
    fields,
    actions
  };
}

// declarativeFieldValuesForPayload 把声明字段的编辑值组成回传载荷——与既有
// 话术确认链等价并存：server 侧 executeAutomationChoreo 读 payload.fields。
export function declarativeFieldValuesForPayload(card: InteractionCardData, values: JsonRecord): JsonRecord {
  const fields: JsonRecord = {};
  card.fields.forEach((field) => {
    const value = textValue(values[field.id]);
    if (value !== "") {
      fields[field.id] = value;
    }
  });
  return { fields };
}

function declarativeRequiredMissing(card: InteractionCardData, values: JsonRecord): boolean {
  return card.fields.some((field) => field.required && textValue(values[field.id]).trim() === "");
}

function declarativeSettledStatus(action: JsonRecord): "" | "settled" | "confirmed" | "cancelled" | "failed" {
  if (textValue(action.resolved_action_id, "") !== "") {
    const status = textValue(action.status ?? action.stage).toLowerCase();
    if (status.includes("cancel")) {
      return "cancelled";
    }
    if (status.includes("fail") || status.includes("error")) {
      return "failed";
    }
    return "confirmed";
  }
  const status = textValue(action.status ?? action.stage).toLowerCase();
  if (status.includes("cancel")) {
    return "cancelled";
  }
  if (status.includes("fail") || status.includes("error")) {
    return "failed";
  }
  if (isConsumedInteractionStatus(status) || status.includes("complete") || status.includes("done")) {
    return "settled";
  }
  return "";
}

function declarativeSettledLabel(state: string): string {
  if (state === "cancelled") {
    return "已取消";
  }
  if (state === "failed") {
    return "失败";
  }
  if (state === "confirmed") {
    return "已确认";
  }
  if (state === "settled") {
    return "已处理";
  }
  return "待确认";
}

// 与 App.tsx interactionActionID 同式（${interactionID}:${actionID}）——busy
// 态的 pendingID 必须与 handleInteractionAction 里的记账键一致。
function declarativePendingID(interactionID: string, actionID: string): string {
  return `${interactionID}:${actionID}`;
}

function declarativeIsCancel(actionID: string): boolean {
  return ["cancel", "reject", "deny", "dismiss", "close", "done"].includes(actionID.trim().toLowerCase());
}

function initialDeclarativeValues(card: InteractionCardData): JsonRecord {
  const out: JsonRecord = {};
  card.fields.forEach((field) => {
    out[field.id] = field.value;
  });
  return out;
}

function DeclarativeFieldControl({
  field,
  value,
  onChange
}: {
  field: InteractionCardField;
  value: string;
  onChange: (value: string) => void;
}) {
  if (field.type === "select") {
    if (field.options.length === 0) {
      return <span className="choice-empty">-</span>;
    }
    return (
      <div className="choice-list">
        {field.options.map((option, index) => (
          <button
            key={`${option.value}-${index}`}
            className={`choice-option ${value === option.value ? "selected" : ""}`}
            type="button"
            data-field-option={option.value}
            onClick={() => onChange(option.value)}
          >
            <span className="choice-index">{index + 1}.</span>
            <span className="choice-main">
              <strong>{option.label}</strong>
            </span>
          </button>
        ))}
      </div>
    );
  }
  if (field.type === "range") {
    const numeric = finiteNumber(value) ?? field.min ?? 0;
    return (
      <span className="declarative-range">
        <input
          type="range"
          value={numeric}
          min={field.min}
          max={field.max}
          step={field.step}
          data-field-control="range"
          onChange={(event) => onChange(event.currentTarget.value)}
        />
        <output>
          {numeric}
          {field.unit ? ` ${field.unit}` : ""}
        </output>
      </span>
    );
  }
  return (
    <span className="declarative-input">
      <input
        type={field.type === "number" ? "number" : "text"}
        value={value}
        min={field.min}
        max={field.max}
        step={field.step}
        data-field-control={field.type}
        onChange={(event) => onChange(event.currentTarget.value)}
      />
      {field.unit && <small className="declarative-unit">{field.unit}</small>}
    </span>
  );
}

export function DeclarativeInteractionCard({
  action,
  card,
  respondingActionID,
  onInteractionAction
}: {
  action: JsonRecord;
  card: InteractionCardData;
  respondingActionID: string;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
}) {
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const settled = declarativeSettledStatus(action);
  const isInteractive = interactionID !== "" && settled === "";
  const [values, setValues] = useState<JsonRecord>(() => initialDeclarativeValues(card));
  const valueSignature = card.fields.map((field) => `${field.id}:${field.value}`).join("|");
  useEffect(() => {
    setValues(initialDeclarativeValues(card));
  }, [interactionID, valueSignature]);
  const requiredMissing = declarativeRequiredMissing(card, values);
  const body = textValue(action.body).trim();
  const reviewItems = firstArray(action.review_items).map(asRecord).filter((item) => Object.keys(item).length > 0);

  return (
    <div
      className={`action-card declarative ${isInteractive ? "interactive" : ""} ${settled ? "settled" : "attention"}`}
      data-card-schema={card.schema}
      data-interaction-id={interactionID}
    >
      <div className="action-content">
        <div className="action-title-line">
          <strong>{card.title || textValue(action.title).trim() || "交互卡"}</strong>
          <span>{declarativeSettledLabel(settled)}</span>
        </div>
        {body && <p className="action-body">{body}</p>}
        {reviewItems.length > 0 && (
          <div className="action-detail-grid">
            {reviewItems.map((item, index) => (
              <div className="action-detail" key={textValue(item.id, `review-${index}`)}>
                <span>{textValue(item.title ?? item.status, `项目 ${index + 1}`)}</span>
                <strong>{textValue(item.body ?? item.value, "-") || "-"}</strong>
              </div>
            ))}
          </div>
        )}
        {isInteractive && card.fields.length > 0 && (
          <div className="interaction-fields">
            {card.fields.map((field) => (
              <label className="interaction-field" key={field.id} data-field-id={field.id} data-field-type={field.type}>
                <span>
                  {field.label}
                  {field.required ? " *" : ""}
                </span>
                <DeclarativeFieldControl
                  field={field}
                  value={textValue(values[field.id], field.value)}
                  onChange={(value) => setValues((current) => ({ ...current, [field.id]: value }))}
                />
                {field.description && <small>{field.description}</small>}
              </label>
            ))}
          </div>
        )}
        {isInteractive && (
          <div className="action-buttons">
            {card.actions.map((cardAction) => {
              const pendingID = declarativePendingID(interactionID, cardAction.action_id);
              const busy = respondingActionID === pendingID;
              const disabled = respondingActionID !== "" || (requiredMissing && !declarativeIsCancel(cardAction.action_id));
              return (
                <button
                  key={cardAction.action_id}
                  className={`action-button ${cardAction.style}`}
                  type="button"
                  data-action-id={cardAction.action_id}
                  disabled={disabled}
                  onClick={() =>
                    onInteractionAction(
                      action,
                      { action_id: cardAction.action_id, label: cardAction.label, style: cardAction.style },
                      declarativeFieldValuesForPayload(card, values)
                    )
                  }
                >
                  <span>{cardAction.label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
