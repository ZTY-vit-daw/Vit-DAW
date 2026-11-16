import { useEffect, useState } from "react";
import { isConsumedInteractionStatus } from "./interactionGuard";
import type { JsonRecord } from "./types";
import { AudioPlayer } from "./widgets/AudioPlayer";
import {
  ComparePair,
  normalizeCompareContext,
  type ComparePreference,
  type CompareSideMedia
} from "./widgets/ComparePair";

// 通用声明式交互卡控件集（FE-INTERACTION-CARD-KIT-1）。契约：interaction
// 的 payload 携带 card = { schema: "interaction_card.v1", title, fields, actions }，
// 其中 fields: {id, type: text|number|range|select, label, value, min, max,
// step, unit, description, required, options: [{value, label}]}，
// actions: {action_id, label, style: primary|secondary|danger}。渲染器只按
// 声明画卡——新增卡面不再需要 webui 场景分支（编舞确认卡为首个迁移卡）。
// 未声明 card 的既有卡继续走原分支，本组件是增不是替换。

export type InteractionCardField = {
  id: string;
  type: "text" | "number" | "range" | "select" | "audio_player" | "compare_pair";
  label: string;
  value: string;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  description?: string;
  required?: boolean;
  options: Array<{ value: string; label: string }>;
  // WIDGET-KIT-1（379）音乐控件挂载：audio_player / compare_pair 字段载荷。
  // 事件回流走既有交互回流通道（declarativeFieldValuesForPayload 的 payload.fields）：
  // compare_pair 的偏好落在字段 value，理由落在 reasonField 指向的字段。
  media?: { artifactID: string; src: string };
  sides?: { a: CompareSideMedia; b: CompareSideMedia };
  /** compare_pair 自身比较上下文（红线①：组件侧白名单提取，不收实验身份） */
  contextID?: string;
  /** compare_pair 理由回流目标字段 id（缺省=卡内不渲染理由输入） */
  reasonField?: string;
};

// WIDGET-KIT-1（379）布局族缺口补齐（目录 §2：行/列/折叠；网格/标签页不在本卡）。
// 无 layout 声明的卡零变化（平铺渲染）；布局只重排字段容器，不改字段契约与
// 回流通道。layout 与 368 阶段 1 草案若冲突，以不重做现有为原则（卡面边界）。
export type InteractionCardLayoutGroup = {
  id: string;
  title: string;
  fieldIDs: string[];
  collapsible: boolean;
  defaultOpen: boolean;
};

export type InteractionCardLayout = {
  type: "row" | "column" | "collapse";
  groups: InteractionCardLayoutGroup[];
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
  layout?: InteractionCardLayout;
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

const FIELD_TYPES = new Set(["text", "number", "range", "select", "audio_player", "compare_pair"]);
const LAYOUT_TYPES = new Set(["row", "column", "collapse"]);
const COMPARE_PREFERENCE_VALUES = new Set(["a", "b", "neither", "unsure"]);
const ACTION_STYLES = new Set(["primary", "secondary", "danger"]);

// widgetSideMedia 提取 compare_pair 单侧媒体声明；label 缺省用侧标。
function widgetSideMedia(raw: JsonRecord, fallbackLabel: string): CompareSideMedia {
  return {
    label: textValue(raw.label ?? raw.title ?? raw.name).trim() || fallbackLabel,
    artifactID: textValue(raw.artifact_id ?? raw.artifactID).trim(),
    src: textValue(raw.src ?? raw.url).trim(),
    durationHint: finiteNumber(raw.duration_hint ?? raw.durationHint)
  };
}

// widgetFieldPayload 提取音乐控件字段载荷（379）：audio_player 取 media 声明
//（artifact_id 走既有 preview URL 通道），compare_pair 取 a/b 两侧 + 比较上下文
// + 理由回流目标字段。缺声明的控件字段渲染诚实降级（不可播占位/单侧占位）。
function widgetFieldPayload(raw: JsonRecord, type: string): Partial<InteractionCardField> {
  if (type === "audio_player") {
    const source = asRecord(raw.media ?? raw.audio);
    return {
      media: {
        artifactID: textValue(source.artifact_id ?? source.artifactID).trim(),
        src: textValue(source.src ?? source.url).trim()
      }
    };
  }
  if (type === "compare_pair") {
    const sideA = asRecord(raw.a ?? raw.left);
    const sideB = asRecord(raw.b ?? raw.right);
    const reasonField = textValue(raw.reason_field ?? raw.reasonField).trim();
    return {
      sides: {
        a: widgetSideMedia(sideA, "A"),
        b: widgetSideMedia(sideB, "B")
      },
      contextID: textValue(raw.context_id ?? raw.contextID).trim(),
      reasonField: reasonField !== "" ? reasonField : undefined
    };
  }
  return {};
}

// declarativeLayoutFromCard 提取布局声明（row/column/collapse）：type 未知或
// groups 为空/无有效字段引用 → undefined（平铺回退，渲染降级不失败）。未编入
// 任何 group 的字段追加为隐式尾组，声明缺漏不丢字段。fieldIDs 引用已注册字段
// id 白名单——悬空引用被过滤，不产生死容器。
function declarativeLayoutFromCard(rawCard: JsonRecord, fieldIDs: string[]): InteractionCardLayout | undefined {
  const rawLayout = asRecord(rawCard.layout);
  const type = textValue(rawLayout.type).trim().toLowerCase();
  if (!LAYOUT_TYPES.has(type)) {
    return undefined;
  }
  const known = new Set(fieldIDs);
  const groups = firstArray(rawLayout.groups).map(asRecord).filter((item) => Object.keys(item).length > 0).map((group, index) => {
    const id = textValue(group.id).trim() || `group-${index + 1}`;
    return {
      id,
      title: textValue(group.title ?? group.label).trim(),
      fieldIDs: firstArray(group.fields ?? group.field_ids).map((value) => textValue(value).trim()).filter((value) => known.has(value)),
      collapsible: type === "collapse",
      defaultOpen: group.open === undefined ? index === 0 : truthy(group.open)
    };
  }).filter((group) => group.fieldIDs.length > 0);
  // 全部组引用悬空 = 布局声明整体无效 → undefined（平铺回退渲染全部字段）
  if (groups.length === 0) {
    return undefined;
  }
  const grouped = new Set(groups.flatMap((group) => group.fieldIDs));
  const leftover = fieldIDs.filter((id) => !grouped.has(id));
  if (leftover.length > 0) {
    groups.push({ id: "group-rest", title: "", fieldIDs: leftover, collapsible: type === "collapse", defaultOpen: true });
  }
  return { type: type as InteractionCardLayout["type"], groups };
}

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
    const normalizedType = (FIELD_TYPES.has(type) ? type : "text") as InteractionCardField["type"];
    const fieldID = textValue(raw.id).trim();
    return {
      id: fieldID || `field-${Math.random().toString(36).slice(2)}`,
      type: normalizedType,
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
      })).filter((option) => option.value !== ""),
      ...widgetFieldPayload(raw, normalizedType)
    };
  }).map((field) => (
    // compare_pair 字段值收敛为合法偏好四态（非法声明值清空，不代选）
    field.type === "compare_pair" && !COMPARE_PREFERENCE_VALUES.has(field.value) ? { ...field, value: "" } : field
  ));
  return {
    schema: textValue(rawCard.schema).trim() || "interaction_card",
    title: textValue(rawCard.title).trim(),
    fields,
    actions,
    layout: declarativeLayoutFromCard(rawCard, fields.map((field) => field.id))
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

// DeclarativeWidgetControl 渲染音乐控件字段（379）：audio_player 走轻量播放器
//（媒体声明缺失时诚实降级占位），compare_pair 走 A/B 对比组合；偏好写回字段
// value、理由写回 reasonField 字段——全部经既有 declarativeFieldValuesForPayload
// 回流通道随 action 上行，不新增回流面。
function DeclarativeWidgetControl({
  field,
  value,
  reasonValue,
  onPreference,
  onReason
}: {
  field: InteractionCardField;
  value: string;
  reasonValue?: string;
  onPreference: (preference: ComparePreference) => void;
  onReason: (reason: string) => void;
}) {
  if (field.type === "audio_player") {
    const media = field.media ?? { artifactID: "", src: "" };
    if (media.artifactID === "" && media.src === "") {
      return (
        <span className="declarative-widget-unavailable" data-widget-unavailable="audio_player">
          {field.label}（音频源未声明）
        </span>
      );
    }
    return (
      <AudioPlayer
        title={field.label}
        artifactID={media.artifactID !== "" ? media.artifactID : undefined}
        src={media.src !== "" ? media.src : undefined}
        expandable={false}
        defaultExpanded
      />
    );
  }
  if (field.type === "compare_pair") {
    const sides = field.sides;
    if (!sides) {
      return (
        <span className="declarative-widget-unavailable" data-widget-unavailable="compare_pair">
          {field.label}（A/B 两侧媒体未声明）
        </span>
      );
    }
    return (
      <ComparePair
        title={field.label}
        a={sides.a}
        b={sides.b}
        context={normalizeCompareContext({ context_id: field.contextID ?? "" }, field.id)}
        layout="row"
        preference={(COMPARE_PREFERENCE_VALUES.has(value) ? value : "") as ComparePreference | ""}
        reason={reasonValue}
        reasonEnabled={Boolean(field.reasonField)}
        onPreference={onPreference}
        onReason={onReason}
      />
    );
  }
  return null;
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

  // 音乐控件字段回写（379）：compare_pair 偏好→字段 value、理由→reasonField
  // 字段。全部经 values → declarativeFieldValuesForPayload 随 action 上行
  // （既有回流通道），组件自身不直连任何业务落账接口（红线②）。
  const writeFieldValue = (fieldID: string, value: string) => {
    setValues((current) => ({ ...current, [fieldID]: value }));
  };
  const renderFieldControl = (field: InteractionCardField) => {
    if (field.type === "audio_player" || field.type === "compare_pair") {
      const reasonFieldID = field.reasonField && card.fields.some((item) => item.id === field.reasonField) ? field.reasonField : undefined;
      return (
        <DeclarativeWidgetControl
          field={field}
          value={textValue(values[field.id], field.value)}
          reasonValue={reasonFieldID !== undefined ? textValue(values[reasonFieldID], "") : undefined}
          onPreference={(preference) => writeFieldValue(field.id, preference)}
          onReason={(reason) => {
            if (reasonFieldID !== undefined) {
              writeFieldValue(reasonFieldID, reason);
            }
          }}
        />
      );
    }
    return (
      <DeclarativeFieldControl
        field={field}
        value={textValue(values[field.id], field.value)}
        onChange={(value) => writeFieldValue(field.id, value)}
      />
    );
  };
  const renderFieldRow = (field: InteractionCardField) => (
    <label className="interaction-field" key={field.id} data-field-id={field.id} data-field-type={field.type}>
      <span>
        {field.label}
        {field.required ? " *" : ""}
      </span>
      {renderFieldControl(field)}
      {field.description && <small>{field.description}</small>}
    </label>
  );
  // 布局族（379）：layout 声明存在且类型合法时按组渲染（row/column=容器分列，
  // collapse=details 折叠），否则维持原平铺（无声明卡片零变化）。
  const fieldsByID = new Map(card.fields.map((field) => [field.id, field]));
  const renderFieldsArea = () => {
    if (!isInteractive || card.fields.length === 0) {
      return null;
    }
    if (!card.layout) {
      return <div className="interaction-fields">{card.fields.map(renderFieldRow)}</div>;
    }
    return (
      <div className="interaction-layout" data-card-layout={card.layout.type} data-layout-type={card.layout.type}>
        {card.layout.groups.map((group) => {
          const groupFields = group.fieldIDs.map((id) => fieldsByID.get(id)).filter((field): field is InteractionCardField => Boolean(field));
          if (group.collapsible) {
            return (
              <details
                key={group.id}
                className="interaction-collapse"
                data-layout-group={group.id}
                data-collapse-open={group.defaultOpen ? "true" : "false"}
                open={group.defaultOpen}
              >
                <summary>{group.title || group.id}</summary>
                <div className="interaction-collapse-body">{groupFields.map(renderFieldRow)}</div>
              </details>
            );
          }
          return (
            <div
              key={group.id}
              className="interaction-layout-group"
              data-layout-group={group.id}
              data-layout-group-title={group.title}
            >
              {group.title !== "" && <div className="interaction-layout-group-title">{group.title}</div>}
              {groupFields.map(renderFieldRow)}
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <div
      className={`action-card declarative ${isInteractive ? "interactive" : ""} ${settled ? "settled" : "attention"}`}
      data-card-schema={card.schema}
      data-interaction-id={interactionID}
      data-card-layout={card.layout?.type}
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
        {renderFieldsArea()}
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
