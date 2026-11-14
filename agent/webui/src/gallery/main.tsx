import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import {
  ActionCard,
  Button,
  ContextTarget,
  Icon,
  MacroCard,
  Message,
  ProjectBadge,
  SnapshotReceipt,
  StatusCard,
  StatusRing,
  TextField,
  VitMark,
  type MacroOption,
} from "../components";
import { DESIGN_SYMBOLS, DesignSprite, SPRITE_SYMBOL_IDS } from "../design/sprite";
import "../styles/tokens.css";
import "../design/sprite.css";
import "../components/components.css";
import "./gallery.css";

/**
 * UI-FOUNDATION-1 组件 gallery。
 * 本页全部工程名 / 消息 / 数据均为设计 fixture（页面顶部有固定标注），
 * 不接 App.tsx 业务，不代表真实 agent 数据。
 */

const ICON_LABELS: Record<string, string> = {
  "i-project": "工程",
  "i-new": "新建工程",
  "i-open": "打开文件夹",
  "i-minimize": "最小化窗口",
  "i-send": "发送",
  "i-stop": "停止生成或执行",
  "i-track": "轨道",
  "i-bus": "总线 / 编组",
  "i-fx": "效果器",
  "i-macro": "宏",
  "i-ab": "对比 A/B",
  "i-play": "播放",
  "i-record": "录音",
  "i-render": "渲染 / 导出",
  "i-undo": "撤销 / 回到快照",
  "i-snapshot": "快照",
  "i-attach": "引用轨道 / 区间",
  "i-idle": "状态：待机",
  "i-busy": "状态：变化中",
  "i-done": "状态：完成",
  "i-error": "状态：出错",
  "i-alert": "错误 / 警示",
  "i-vit": "Vit 品牌标（仅 OS 身份位置）",
};

const MACRO_OPTIONS: MacroOption[] = [
  {
    id: "warm",
    name: "温暖",
    description: "中频略厚，鼓皮更靠近听众。",
    contour: [0.2, 0.3, 0.45, 0.62, 0.78, 0.85, 0.8, 0.72, 0.6, 0.5, 0.42, 0.35],
  },
  {
    id: "bright",
    name: "明亮",
    description: "高频提起，镲片更清楚。",
    contour: [0.25, 0.22, 0.2, 0.24, 0.3, 0.38, 0.5, 0.62, 0.72, 0.8, 0.86, 0.9],
  },
  {
    id: "tight",
    name: "紧实",
    description: "低频收短，整体更靠前。",
    contour: [0.5, 0.45, 0.35, 0.28, 0.3, 0.4, 0.55, 0.6, 0.55, 0.5, 0.55, 0.6],
  },
];

function Section(props: {
  id: string;
  title: string;
  desc?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="v5-gal-section" data-gallery={props.id}>
      <h2 className="v5-gal-section-title">{props.title}</h2>
      {props.desc ? <p className="v5-gal-section-desc">{props.desc}</p> : null}
      <div className="v5-gal-section-body">{props.children}</div>
    </section>
  );
}

function IconMatrix() {
  return (
    <table className="v5-gal-matrix">
      <thead>
        <tr>
          <th>图标</th>
          <th>浅底 16</th>
          <th>浅底 20</th>
          <th>浅底 24</th>
          <th>深底 16</th>
          <th>深底 20</th>
          <th>深底 24</th>
          <th>选中 20</th>
        </tr>
      </thead>
      <tbody>
        {SPRITE_SYMBOL_IDS.map((id) => (
          <tr key={id}>
            <th className="v5-gal-matrix-label">
              {ICON_LABELS[id] ?? id}
              <code>{id}</code>
            </th>
            {([16, 20, 24] as const).map((s) => (
              <td key={`l${s}`} data-bg="light">
                <Icon name={id} size={s} />
              </td>
            ))}
            {([16, 20, 24] as const).map((s) => (
              <td key={`d${s}`} data-bg="dark" className="v5-dark v5-dark-cell">
                <Icon name={id} size={s} />
              </td>
            ))}
            <td data-bg="selected">
              <span className="is-selected v5-icon-sample--selected">
                <Icon name={id} size={20} />
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Gallery() {
  return (
    <div className="v5-design v5-gal-root">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 组件 gallery · v5.1 基座</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页所有工程名、消息与数据均为设计 fixture，仅用于展示组件全部状态，
          不是真实工程或真实 agent 数据。
        </p>
      </header>

      <Section
        id="tokens"
        title="设计令牌（S4）"
        desc="来源 design-tokens-v5.json（含 v5.1 depth 令牌），CSS 变量加 --v5- 前缀。"
      >
        <div className="v5-gal-tokens">
          {[
            ["read-bg", "#FFFFFF"],
            ["read-workspace", "#F0F1F2"],
            ["ui-text", "#141414"],
            ["ui-muted", "#555555"],
            ["ui-canvas", "#F4F4F4"],
            ["control-header", "#31567D"],
            ["control-shell-raised", "#D4E1EF"],
            ["control-well-deep", "#BDCDDF"],
            ["control-choice-selected", "#F6E7AA"],
            ["control-pane", "#1D2228"],
            ["control-blue", "#73B8F5"],
            ["control-yellow", "#FACC1A"],
            ["window-edge", "#5B626A"],
            ["warning-shell", "#F7E9E8"],
            ["warning-edge", "#9D3430"],
            ["attention-shell", "#F6E7AA"],
          ].map(([name, hex]) => (
            <div key={name} className="v5-gal-token">
              <span className="v5-gal-swatch" style={{ background: hex }} />
              <span className="v5-gal-token-name">{name}</span>
              <code className="v5-gal-token-hex">{hex}</code>
            </div>
          ))}
        </div>
      </Section>

      <Section
        id="icon-matrix"
        title="图标矩阵（S3）"
        desc="23 枚原创 symbol（22 功能 + i-vit），16 / 20 / 24px × 浅底 / 深底 / 选中。"
      >
        <IconMatrix />
      </Section>

      <Section
        id="titlebar"
        title="对话窗标题栏样例"
        desc="标题栏 = i-project + 工程名；V 品牌标不进入标题栏（v5.1 修订）。"
      >
        <div className="v5-gal-titlebar">
          <ProjectBadge size={20} />
          <span className="v5-gal-titlebar-name">鼓组混音 v2</span>
          <Icon name="i-minimize" size={16} className="v5-gal-titlebar-min" />
        </div>
      </Section>

      <Section id="buttons" title="按钮（S3）" desc="主按钮墨底；次按钮纸底 1.5px 墨边；危险按钮红底纸字；文字按钮。">
        <div className="v5-gal-row">
          <Button variant="primary">应用</Button>
          <Button variant="secondary">对比 A/B</Button>
          <Button variant="danger">覆盖导出</Button>
          <Button variant="text">存为我的快照</Button>
        </div>
        <div className="v5-gal-row">
          <Button variant="primary" icon="i-send">发送</Button>
          <Button variant="secondary" icon="i-stop">停止</Button>
          <Button variant="primary" disabled>
            应用
          </Button>
          <Button variant="secondary" disabled>
            对比 A/B
          </Button>
        </div>
      </Section>

      <Section
        id="inputs"
        title="输入框（§8.6）"
        desc="默认 / 禁用（装载中：斜纹底）/ Vit 回复中（发送键变 i-stop）。"
      >
        <div className="v5-gal-col">
          <TextField placeholder="告诉 Vit 下一步做什么" onSend={() => {}} />
          <TextField
            placeholder="正在打开《鼓组混音 v2》，打开完成后可继续输入"
            disabled
            disabledHint="打开完成后可继续输入"
          />
          <TextField placeholder="Vit 正在回复…" busy onStop={() => {}} />
        </div>
      </Section>

      <Section id="context-target" title="上下文目标" desc="默认浅底 1px 边；选中浅黄 + 2px 墨框 + 1px 外环。">
        <div className="v5-gal-row">
          <ContextTarget label="鼓组总线" />
          <ContextTarget label="鼓组总线" icon="i-bus" selected />
          <ContextTarget label="主唱轨" icon="i-track" />
        </div>
      </Section>

      <Section
        id="macro-cards"
        title="宏卡（S5 · 四态）"
        desc="默认 / 试听中（黄）/ 已应用（蓝，可撤销）/ 失败（红，原因 + 重试）。选择 ≠ 应用。"
      >
        <div className="v5-gal-col v5-gal-col-wide">
          <MacroCard
            object="鼓组总线"
            name="染色"
            options={MACRO_OPTIONS}
            selectedId={null}
            onSelect={() => {}}
            state="default"
          />
          <MacroCard
            object="鼓组总线"
            name="染色"
            options={MACRO_OPTIONS}
            selectedId="warm"
            onSelect={() => {}}
            state="auditioning"
            onAudition={() => {}}
            onCancelAudition={() => {}}
          />
          <MacroCard
            object="鼓组总线"
            name="染色"
            options={MACRO_OPTIONS}
            selectedId="warm"
            onSelect={() => {}}
            state="applied"
            onUndo={() => {}}
            onSaveSnapshot={() => {}}
          />
          <MacroCard
            object="鼓组总线"
            name="染色"
            options={MACRO_OPTIONS}
            selectedId="bright"
            onSelect={() => {}}
            state="failed"
            failureReason="音频设备被其他程序占用。"
            onRetry={() => {}}
          />
        </div>
      </Section>

      <Section
        id="status-rings"
        title="状态环（四态 × 三尺寸）"
        desc="待机 3/4 虚线（9s 慢转）、进行中 3/4 实线（1.6s 快转）、完成、出错；形状不依赖颜色。"
      >
        <div className="v5-gal-ringrow">
          {(["idle", "busy", "done", "error"] as const).map((st) => (
            <span key={st} className="v5-gal-ringcell">
              <StatusRing state={st} size={16} />
              <StatusRing state={st} size={20} />
              <StatusRing state={st} size={24} />
              <span className="v5-current-chip">
                <StatusRing state={st === "busy" ? "busy" : "done"} size={16} />
              </span>
              <span className="v5-dark v5-dark-cell v5-gal-ringcell-dark">
                <StatusRing state={st} size={20} />
              </span>
            </span>
          ))}
        </div>
        <p className="v5-gal-caption">
          待机 / 进行中 / 完成 / 出错；末列为深底样张（黄 chip 内为当前步骤动效位）。
        </p>
      </Section>

      <Section
        id="status-cards"
        title="状态卡（执行轨迹）"
        desc="进行中（最多 3 步 + 小面积黄）/ 完成（收成一行回执）/ 失败（原因 + 重试）。"
      >
        <div className="v5-gal-col v5-gal-col-wide">
          <StatusCard
            title="正在打开《鼓组混音 v2》"
            steps={[
              { id: "s1", label: "读取工程文件", status: "done", duration: "0.8s" },
              { id: "s2", label: "连接音频设备", status: "done", duration: "0.4s" },
              { id: "s3", label: "载入轨道与效果器", status: "current" },
            ]}
            state="running"
            progress={0.64}
          />
          <StatusCard
            title="正在应用宏「温暖」"
            steps={[
              { id: "a1", label: "写入 6 个绑定参数", status: "done", duration: "0.2s" },
              { id: "a2", label: "渲染对比 A/B", status: "done", duration: "3.1s" },
            ]}
            state="completed"
          />
          <StatusCard
            title="正在渲染对比 A/B"
            steps={[
              { id: "f1", label: "导出旁通渲染", status: "done", duration: "2.8s" },
              { id: "f2", label: "导出处理渲染", status: "done", duration: "3.0s" },
            ]}
            state="failed"
            failureReason="音频设备被其他程序占用，无法写入对比轨道。"
            onRetry={() => {}}
          />
        </div>
      </Section>

      <Section
        id="messages"
        title="消息与快照回执"
        desc="用户 / Vit 只靠底色区分；快照回执为一行。"
      >
        <div className="v5-gal-col">
          <Message author="user">帮我把鼓组总线压得再紧一点，但别失去瞬态。</Message>
          <Message author="vit">
            已在「染色」里准备了三个方向，试听满意后再应用，随时可以撤销。
          </Message>
          <SnapshotReceipt time="14:07" onUndo={() => {}} />
        </div>
      </Section>

      <Section
        id="action-cards"
        title="动作卡（后果等级）"
        desc="可撤销（蓝）/ 需要你确认（黄）/ 不可逆（红确认按钮）；等级用图标 + 词表达。"
      >
        <div className="v5-gal-col v5-gal-col-wide">
          <ActionCard
            title="应用宏「温暖」"
            description="写入 6 个绑定参数，应用后可整体回退。"
            consequence="reversible"
            confirmLabel="应用"
            onConfirm={() => {}}
          />
          <ActionCard
            title="装载 3 个 VST3 插件到鼓组总线"
            description="插件会改变工程渲染结果，装载前需要你确认。"
            consequence="confirm"
            confirmLabel="确认"
            onConfirm={() => {}}
            onCancel={() => {}}
          />
          <ActionCard
            title="覆盖导出《鼓组混音 v2》"
            description="将覆盖当前已渲染的文件，无法恢复。"
            consequence="irreversible"
            confirmLabel="覆盖导出"
            onConfirm={() => {}}
            onCancel={() => {}}
          />
        </div>
      </Section>

      <Section
        id="os-identity"
        title="磁带小标志与 V 品牌标"
        desc="i-project 用于工程对象；i-vit 只出现在 OS 身份位置（任务栏 Vit 条目、系统通知），不进标题栏。"
      >
        <div className="v5-gal-row">
          <span className="v5-gal-markcell">
            <ProjectBadge size={16} />
            <ProjectBadge size={20} />
            <ProjectBadge size={24} />
            <ProjectBadge size={20} state="busy" />
          </span>
          <span className="v5-gal-caption">i-project：16 / 20 / 24px；第四枚带变化中状态点</span>
        </div>
        <div className="v5-gal-osrow">
          <div className="v5-taskbar v5-dark" data-os-sample="taskbar">
            <span className="v5-taskbar-entry">
              <VitMark size={20} state="busy" />
              <span>Vit</span>
            </span>
            <span className="v5-taskbar-entry">
              <ProjectBadge size={20} />
              <span>鼓组混音 v2</span>
            </span>
          </div>
          <div className="v5-notification" data-os-sample="notification">
            <VitMark size={24} state="done" />
            <div className="v5-notification-body">
              <strong>鼓组混音 v2</strong>
              <p>已应用「温暖」，A/B 已渲染好。</p>
              <div className="v5-notification-actions">
                <Button variant="text">打开 Vit</Button>
                <Button variant="text">撤销</Button>
              </div>
            </div>
          </div>
        </div>
      </Section>

      <footer className="v5-gal-footer">
        <p>
          设计来源哈希：design-tokens-v5.json = 1d985d5e…；
          round2-sprite.svg = 746a49f5…（23 symbol 逐字内联）。
        </p>
        <p>UI-FOUNDATION-1 · 本页为设计基座 fixture，不接 App.tsx 业务。</p>
      </footer>
    </div>
  );
}

// DESIGN_SYMBOLS 供烟测脚本核对 symbol markup（在页面 window 上只读暴露）。
if (typeof window !== "undefined") {
  (window as unknown as { __V5_SPRITE_IDS: string[] }).__V5_SPRITE_IDS = [
    ...SPRITE_SYMBOL_IDS,
  ];
  (window as unknown as { __V5_SPRITE_MARKUP: Record<string, string> }).__V5_SPRITE_MARKUP =
    { ...DESIGN_SYMBOLS };
}

const root = document.getElementById("gallery-root");
if (root) {
  createRoot(root).render(
    <StrictMode>
      <Gallery />
    </StrictMode>
  );
}
