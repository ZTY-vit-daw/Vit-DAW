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
import { directivePlaceholder } from "../launcher/launcherData";
// CONVERGE-SHELL-1（387）：收敛壳 fixture 面（366 磁带卡整卡形态 + chat 域对话流复用）。
import { ConvergedLauncher, type ConvergedThreadItem } from "../launcher/ConvergedLauncher";
import type { LauncherProjectCard } from "../api/launcher";
import { ProjectSurface } from "../project/ProjectSurface";
import { createProjectFixtureService } from "../project/projectFixtures";
import type { MacroCardView, ProjectThreadItem } from "../project/projectData";
// UI-LOADING-1（347）：P2 装载 / 执行生命周期 fixture 样张（全离线，零网络）。
import { LoadingCard } from "../project/loading/LoadingCard";
import { ProjectLoadingGate } from "../project/loading/ProjectLoadingGate";
import type { LoadingCardView } from "../project/loading/loadingData";
// WIDGET-KIT-1（379）：音乐控件 fixture 样张（轻量播放器/A-B 对比组合/卡内挂载）。
import { AudioPlayer } from "../widgets/AudioPlayer";
import { ComparePair } from "../widgets/ComparePair";
import { DeclarativeInteractionCard, declarativeCardFromAction } from "../interactionCardKit";
import "../styles/tokens.css";
import "../design/sprite.css";
import "../components/components.css";
import "../widgets/widgets.css";
import "./gallery.css";

/**
 * UI-FOUNDATION-1 组件 gallery。
 * 本页全部工程名 / 消息 / 数据均为设计 fixture（页面顶部有固定标注），
 * 不接 App.tsx 业务，不代表真实 agent 数据。
 */

/*
 * UI-LAUNCHER-1（345）launcher fixture（卡面标注：设计 fixture，非真实工程/真实
 * agent 数据）。live surface 的真实端点消费走 ?surface=launcher 主面；此处数据
 * 只用于渲染烟测断言（标题/抽屉/recent 数/建议/输入/加载/错误态）。
 * 命名带（fixture）后缀自证非真实工程名，生产代码不含任何示例工程名。
 */
const LAUNCHER_FIXTURE_CARDS: LauncherProjectCard[] = [
  {
    id: "fixture_launcher_project_1",
    kind: "launcher_project",
    type: "project_open_candidate",
    name: "示例工程甲（fixture）",
    filePath: "D:/design-fixtures/示例工程甲.vit",
    directive: { action: "open_project", file_path: "D:/design-fixtures/示例工程甲.vit", requires_governance: true }
  },
  {
    id: "fixture_launcher_project_2",
    kind: "launcher_project",
    type: "project_open_candidate",
    name: "示例工程乙（fixture）",
    filePath: "D:/design-fixtures/示例工程乙.vit",
    directive: { action: "open_project", file_path: "D:/design-fixtures/示例工程乙.vit", requires_governance: true }
  },
  {
    id: "fixture_launcher_project_3",
    kind: "launcher_project",
    type: "project_open_candidate",
    name: "示例工程丙（fixture）",
    filePath: "D:/design-fixtures/示例工程丙.vit",
    directive: { action: "open_project", file_path: "D:/design-fixtures/示例工程丙.vit", requires_governance: true }
  }
];

const LAUNCHER_FIXTURE_THREAD: ConvergedThreadItem[] = [
  { kind: "user", text: "看看最近的工程" },
  { kind: "assistant", text: "最近的工程有 3 个——点卡片打开，或直接跟我说名字：" },
  { kind: "cards", cards: LAUNCHER_FIXTURE_CARDS },
  { kind: "user", text: "打开示例工程甲" },
  { kind: "assistant", text: "好的，打开《示例工程甲（fixture）》。" },
  {
    kind: "directive",
    view: directivePlaceholder({
      action: "open_project",
      file_path: "D:/design-fixtures/示例工程甲.vit",
      requires_governance: true
    })!
  }
];

const LAUNCHER_FIXTURE_GREETING = {
  status: "ok",
  launcher: true,
  reply: "我在。最近的工程是《示例工程甲（fixture）》——想从哪开始？",
  recent_project: { name: "示例工程甲（fixture）", file_path: "D:/design-fixtures/示例工程甲.vit" },
  recent_count: 3
};

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

      <Section
        id="launcher-surface"
        title="收敛启动台 surface（CONVERGE-CLEANUP-1）"
        desc="?surface=launcher 主面样例：左抽屉真实最近工程（此处为 fixture）+ 问候语 + 建议 + 输入；标题栏只写 Vit 文本；结果卡/directive 一律数据展示占位，不执行 open/new。全状态样例见 gallery.html?surface=launcher。"
      >
        <ConvergedLauncher
          fixture={{
            greeting: LAUNCHER_FIXTURE_GREETING,
            drawerState: "ready",
            drawerEntries: LAUNCHER_FIXTURE_CARDS,
            thread: LAUNCHER_FIXTURE_THREAD
          }}
        />
        <p className="v5-gal-caption" data-launcher-gallery-note>
          directive 卡呈报自动执行语态；fixture 不发送打开/新建桥请求。
          live 工程切换由 Vit 桥执行，治理确认跟随发起方。
        </p>
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

/**
 * UI-LAUNCHER-1 启动台 fixture 渲染页（gallery.html?surface=launcher）。
 * 渲染烟测专页：ready 全尺寸样例 + 加载/错误/空态紧凑样例；全离线（fixture
 * 接缝零网络）。所有工程名与回执均为设计 fixture。
 */
function LauncherFixturePage() {
  return (
    <div className="v5-design v5-gal-root" data-gallery-page="launcher">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 启动台 surface · fixture 渲染页</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页所有工程名与回执均为设计 fixture，不接真实 agent 数据；真实
          /agent/launcher/greeting 与 launcher 聊天消费见主面 ?surface=launcher。
        </p>
      </header>

      <ConvergedLauncher
        fixture={{
          greeting: LAUNCHER_FIXTURE_GREETING,
          drawerState: "ready",
          drawerEntries: LAUNCHER_FIXTURE_CARDS,
          thread: LAUNCHER_FIXTURE_THREAD
        }}
      />

      <div className="v5-gal-launcher-states" data-gallery="launcher-states">
        <div data-launcher-sample="loading">
          <h2 className="v5-gal-caption">加载态（greeting / 最近工程读取中）</h2>
          <ConvergedLauncher fixture={{ thread: [], drawerState: "loading" }} />
        </div>
        <div data-launcher-sample="error">
          <h2 className="v5-gal-caption">错误态（agent 不可用 + 最近工程失败，各带重试）</h2>
          <ConvergedLauncher
            fixture={{
              greetingErrorText: "连不上 Vit agent——启动台暂时不可用。确认 VitAgent 已启动后重试。",
              drawerState: "error",
              drawerErrorText: "最近工程列表暂时读不到（内核未就绪或 agent 异常）。"
            }}
          />
        </div>
        <div data-launcher-sample="empty">
          <h2 className="v5-gal-caption">空态（还没有最近工程记录）</h2>
          <ConvergedLauncher fixture={{ greeting: { reply: "我在。还没有最近的工程记录——想从哪开始？" }, drawerState: "empty" }} />
        </div>
      </div>

      <footer className="v5-gal-footer">
        <p>UI-LAUNCHER-1 · 本页为 launcher 渲染烟测 fixture，不接 App.tsx 业务、不出网。</p>
      </footer>
    </div>
  );
}

/*
 * UI-PROJECT-1（346）P3 工程内对话主窗 fixture（卡面标注：设计 fixture，非真实
 * 工程/真实 agent 数据）。live surface 的真实端点消费走 ?surface=project 主面
 * （ui_state/chat/stop_turn）；此处数据只用于渲染烟测断言（上下文头/消息流/
 * 宏卡四态/快照回执与撤销留痕/轨迹四态/输入四态/A/B 四态）。命名带（fixture）
 * 后缀自证非真实工程名，生产代码不含任何示例工程名。
 */
const PROJECT_FIXTURE_OPTIONS: MacroOption[] = [
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

const PROJECT_FIXTURE_CONTEXT = {
  name: "示例工程甲（fixture）",
  source: "fixture" as const,
  detail: "6 条轨道"
};

function projectMacroThreadItem(
  id: string,
  overrides: Partial<MacroCardView>
): ProjectThreadItem {
  return {
    kind: "macro",
    id,
    view: {
      object: "鼓组总线",
      name: "染色",
      options: PROJECT_FIXTURE_OPTIONS,
      selectedId: null,
      state: "default",
      ...overrides,
    },
  };
}

const PROJECT_FIXTURE_THREAD: ProjectThreadItem[] = [
  { kind: "message", id: "p-u1", author: "user", text: "鼓组总线再紧一点，但别失去瞬态。" },
  { kind: "message", id: "p-v1", author: "vit", text: "我在「染色」里准备了三个方向，试听满意后再应用，随时可以撤销。" },
  projectMacroThreadItem("p-m1", { selectedId: "warm" }),
  {
    kind: "trajectory",
    id: "p-t1",
    view: {
      state: "busy",
      title: "正在应用宏「温暖」",
      steps: [
        { id: "p-s1", label: "读取绑定参数", status: "done", duration: "0.2s" },
        { id: "p-s2", label: "写入 6 个绑定参数", status: "current" },
      ],
    },
  },
  { kind: "snapshot", id: "p-snap1", view: { time: "14:07", snapshotRef: "fixture-snapshot-p-m1-warm" } },
  { kind: "ab", id: "p-ab1", view: { state: "idle", title: "鼓组总线 · 染色 对比" } },
];

/**
 * UI-PROJECT-1 工程内主窗 fixture 渲染页（gallery.html?surface=project）。
 * 就绪全尺寸样例 + 宏卡四态 / 轨迹四态 / 输入四态 / A/B 四态 / 快照与撤销 /
 * 上下文错误态紧凑样例；全离线（fixture 接缝零网络）。
 */
function ProjectFixturePage() {
  return (
    <div className="v5-design v5-gal-root" data-gallery-page="project">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 工程内主窗 surface · fixture 渲染页</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页所有工程名、宏名、快照引用与回执均为设计 fixture，不接真实 agent 数据；
          真实 ui_state / chat / stop_turn 消费见主面 ?surface=project。
        </p>
      </header>

      <ProjectSurface
        fixture={{
          context: PROJECT_FIXTURE_CONTEXT,
          thread: PROJECT_FIXTURE_THREAD,
          service: createProjectFixtureService({
            context: PROJECT_FIXTURE_CONTEXT,
            applyOutcome: {
              ok: true,
              snapshotRef: "fixture-snapshot-interactive",
              time: "14:07"
            }
          })
        }}
      />

      <div className="v5-gal-launcher-states" data-gallery="project-states">
        <div data-project-sample="macro-states">
          <h2 className="v5-gal-caption">宏卡四态（选择 ≠ 应用；失败保留未应用事实）</h2>
          <ProjectSurface
            fixture={{
              context: PROJECT_FIXTURE_CONTEXT,
              thread: [
                projectMacroThreadItem("fx-m-default", {}),
                projectMacroThreadItem("fx-m-aud", { state: "auditioning", selectedId: "warm" }),
                projectMacroThreadItem("fx-m-applied", {
                  state: "applied",
                  selectedId: "warm",
                  appliedOptionId: "warm",
                  appliedSnapshotRef: "fixture-snapshot-fx-m-applied-warm"
                }),
                projectMacroThreadItem("fx-m-failed", {
                  state: "failed",
                  selectedId: "bright",
                  failureReason: "音频设备被其他程序占用。"
                })
              ]
            }}
          />
        </div>
        <div data-project-sample="trajectory">
          <h2 className="v5-gal-caption">执行轨迹四态（busy 步骤推进 / done 收成一行 / error 原因 / idle 待机）</h2>
          <ProjectSurface
            fixture={{
              context: PROJECT_FIXTURE_CONTEXT,
              thread: [
                {
                  kind: "trajectory",
                  id: "fx-t-busy",
                  view: {
                    state: "busy",
                    title: "正在渲染对比 A/B",
                    steps: [
                      { id: "fx-t-s1", label: "导出旁通渲染", status: "done", duration: "2.8s" },
                      { id: "fx-t-s2", label: "导出处理渲染", status: "current" }
                    ]
                  }
                },
                { kind: "trajectory", id: "fx-t-done", view: { state: "done", title: "已应用宏「明亮」", steps: [] } },
                {
                  kind: "trajectory",
                  id: "fx-t-error",
                  view: { state: "error", title: "正在应用宏「温暖」", steps: [], failureReason: "音频设备被其他程序占用，写入失败。" }
                },
                { kind: "trajectory", id: "fx-t-idle", view: { state: "idle", title: "待机", steps: [] } }
              ]
            }}
          />
        </div>
        <div data-project-sample="composer-states">
          <h2 className="v5-gal-caption">输入框四态（默认 / 焦点 / 禁用 / Vit 回复中）</h2>
          <ProjectSurface fixture={{ context: PROJECT_FIXTURE_CONTEXT }} />
          <ProjectSurface fixture={{ context: PROJECT_FIXTURE_CONTEXT, forceComposerState: "focus" }} />
          <ProjectSurface fixture={{ context: PROJECT_FIXTURE_CONTEXT, inputDisabled: true }} />
          <ProjectSurface fixture={{ context: PROJECT_FIXTURE_CONTEXT, forceComposerState: "replying" }} />
        </div>
        <div data-project-sample="ab-states">
          <h2 className="v5-gal-caption">对比 A/B 四态（准备→就绪绑定真实渲染引用；开始方式待用户裁定）</h2>
          <ProjectSurface
            fixture={{
              context: PROJECT_FIXTURE_CONTEXT,
              thread: [
                { kind: "ab", id: "fx-ab-idle", view: { state: "idle", title: "鼓组总线 · 染色 对比" } },
                { kind: "ab", id: "fx-ab-prep", view: { state: "preparing", title: "鼓组总线 · 染色 对比" } },
                {
                  kind: "ab",
                  id: "fx-ab-ready",
                  view: {
                    state: "ready",
                    title: "鼓组总线 · 染色 对比",
                    bypassRef: "fixture-render-bypass",
                    processedRef: "fixture-render-processed"
                  }
                },
                {
                  kind: "ab",
                  id: "fx-ab-failed",
                  view: { state: "failed", title: "鼓组总线 · 染色 对比", failureReason: "渲染队列已满，稍后重试。" }
                }
              ]
            }}
          />
        </div>
        <div data-project-sample="snapshot-undo">
          <h2 className="v5-gal-caption">快照回执与撤销留痕（撤销绑定真实快照引用）</h2>
          <ProjectSurface
            fixture={{
              context: PROJECT_FIXTURE_CONTEXT,
              thread: [
                { kind: "snapshot", id: "fx-snap-live", view: { time: "14:07", snapshotRef: "fixture-snapshot-live" } },
                {
                  kind: "snapshot",
                  id: "fx-snap-undone",
                  view: { time: "14:11", snapshotRef: "fixture-snapshot-undone", undone: true }
                }
              ]
            }}
          />
        </div>
        <div data-project-sample="context-error">
          <h2 className="v5-gal-caption">上下文错误态（重试在卷）</h2>
          <ProjectSurface fixture={{ contextErrorText: "连不上内核——工程上下文暂不可用（fixture 样例）。" }} />
        </div>
      </div>

      <footer className="v5-gal-footer">
        <p>UI-PROJECT-1 · 本页为工程内主窗渲染烟测 fixture，不接 App.tsx 业务、不出网。</p>
      </footer>
    </div>
  );
}

/**
 * UI-LOADING-1（347）P2 装载 / 执行生命周期 fixture 渲染页（gallery.html?surface=loading）。
 * SPEC P2/S2 语义样张：三步装载状态卡（已读取工程文件/已恢复轨道/正在连接音频设备）
 * + 磁带带量三帧（待机/进行 0.64/完成）+ 完成回执（轨迹可展开）+ 错误（重试）
 * + 中断（重新打开）+ 装载门输入禁用/恢复。全离线（fixture 零网络）；
 * onRetry 均为样张空操作（真实重试语义走 live 装载门 / reducer 单测）。
 */
const LOADING_FIXTURE_VIEW: LoadingCardView = {
  phase: "loading",
  title: "示例工程甲（fixture）",
  steps: [
    { id: "fx-l-s1", label: "已读取工程文件", status: "done", duration: "0.4s" },
    { id: "fx-l-s2", label: "已恢复轨道", status: "done" },
    { id: "fx-l-s3", label: "正在连接音频设备", status: "current" }
  ],
  progress: null // 未知进度：磁带停待机帧，不伪造推进
};

function LoadingFixturePage() {
  return (
    <div className="v5-design v5-gal-root" data-gallery-page="loading">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 装载生命周期 · fixture 渲染页</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页工程名、步骤耗时与失败原因均为设计 fixture，不接真实 agent 数据；
          live 装载门消费真实 ui_state 探针（?surface=project 主面）。未知进度不伪造
          推进——磁带带量只随真实进度事件变化。
        </p>
      </header>

      <div className="v5-gal-launcher-states" data-gallery="loading-states">
        <div data-loading-sample="card-steps">
          <h2 className="v5-gal-caption">装载状态卡 · 三步轨迹（SPEC P2；当前步小面积黄，未知进度停待机帧）</h2>
          <LoadingCard view={LOADING_FIXTURE_VIEW} />
        </div>

        <div data-loading-sample="tape-progress">
          <h2 className="v5-gal-caption">磁带带量三帧（待机 L=1/R=0.2 · 进行 p=0.64 · 完成 L=0.2/R=1；带从左盘绕到右盘）</h2>
          <LoadingCard
            view={{ phase: "loading", title: "示例工程甲（fixture）", steps: [], progress: null }}
          />
          <LoadingCard
            view={{ phase: "loading", title: "示例工程甲（fixture）", steps: [], progress: 0.64 }}
          />
          <LoadingCard
            view={{ phase: "completed", title: "示例工程甲（fixture）", steps: [], progress: null, resultNote: "3 条轨道已恢复" }}
          />
        </div>

        <div data-loading-sample="completed">
          <h2 className="v5-gal-caption">完成收一行回执（蓝）·轨迹可展开；输入恢复</h2>
          <LoadingCard
            view={{
              phase: "completed",
              title: "示例工程甲（fixture）",
              steps: LOADING_FIXTURE_VIEW.steps,
              progress: null,
              resultNote: "3 条轨道已恢复"
            }}
          />
        </div>

        <div data-loading-sample="failed">
          <h2 className="v5-gal-caption">错误路径（必测）：失败写明原因与重试入口（红警示）</h2>
          <LoadingCard
            view={{
              phase: "failed",
              title: "示例工程甲（fixture）",
              steps: [],
              progress: null,
              failureReason: "音频设备被其他程序占用，装载停止。"
            }}
            onRetry={() => undefined}
          />
        </div>

        <div data-loading-sample="interrupted">
          <h2 className="v5-gal-caption">中断路径（必测）：中断写明原因与恢复入口（需留意黄）</h2>
          <LoadingCard
            view={{
              phase: "interrupted",
              title: "示例工程甲（fixture）",
              steps: [],
              progress: null,
              failureReason: "宿主窗口被关闭，装载中断。"
            }}
            onRetry={() => undefined}
          />
        </div>

        <div data-loading-sample="gate-disabled">
          <h2 className="v5-gal-caption">装载门：探测失败=装载卡（children 不渲染=输入禁用）；就绪=放行工程内主窗（输入恢复）</h2>
          <ProjectLoadingGate fixture={{ probe: "error", errorText: "连不上 Vit agent——fixture 样例。确认 VitAgent 在运行后重试。" }}>
            <div data-gate-child-hidden>工程内主窗（不应渲染）</div>
          </ProjectLoadingGate>
          <ProjectLoadingGate fixture={{ probe: "ok" }}>
            <ProjectSurface
              fixture={{
                context: { name: "示例工程甲（fixture）", source: "fixture", detail: "3 条轨道" }
              }}
            />
          </ProjectLoadingGate>
        </div>
      </div>

      <footer className="v5-gal-footer">
        <p>UI-LOADING-1 · 本页为装载生命周期渲染烟测 fixture，不接 App.tsx 业务、不出网。</p>
      </footer>
    </div>
  );
}

/**
 * UI-VISUAL-FIDELITY-1 live 形态 fixture 渲染页（gallery.html?surface=launcher-live /
 * project-live）：与 live surface 完全相同的 fill 布局（撑满视口、无边框窗框），
 * 数据仍为离线 fixture（surface root 自带 data-fixture 标注）——渲染烟测多分辨率
 * 断言（1440×900 / 1920×1080 / 125% / 150% 缩放等价视口）用它验证宿主窗口尺寸
 * 不确定时不裁切、输入区/建议始终可达。
 */
function LauncherLiveFixturePage() {
  return (
    <div className="v5-live-fixture" data-gallery-page="launcher-live">
      <DesignSprite />
      <ConvergedLauncher
        fill
        fixture={{
          greeting: LAUNCHER_FIXTURE_GREETING,
          drawerState: "ready",
          drawerEntries: LAUNCHER_FIXTURE_CARDS,
          thread: LAUNCHER_FIXTURE_THREAD
        }}
      />
    </div>
  );
}

function ProjectLiveFixturePage() {
  return (
    <div className="v5-live-fixture" data-gallery-page="project-live">
      <DesignSprite />
      <ProjectSurface
        fill
        fixture={{
          context: PROJECT_FIXTURE_CONTEXT,
          thread: PROJECT_FIXTURE_THREAD,
          service: createProjectFixtureService({
            context: PROJECT_FIXTURE_CONTEXT,
            applyOutcome: {
              ok: true,
              snapshotRef: "fixture-snapshot-live-interactive",
              time: "14:07"
            }
          })
        }}
      />
    </div>
  );
}

/**
 * WIDGET-KIT-1（379）音乐控件 fixture 渲染页（gallery.html?surface=widgets）。
 * 三段样张（全离线，音频=data URI 静音 WAV，零网络零代理）：
 *  ① audio_player 轻量播放器（收起/展开/不可播降级三态）；
 *  ② compare_pair A/B 对比组合（异长版本 duration-hint 各自绑定坐标，
 *    偏好四态+理由输入；onCompareEvent 样张空操作）；
 *  ③ interactionCardKit 卡内挂载（collapse 布局 + audio_player/compare_pair
 *    字段 + 理由字段回流声明 + 提交/取消动作）。
 * 比较面不携带任何试听会话/实验身份（红线①）；不路由混音裁决（红线②）；
 * 不渲染共享时间轴（红线③）。
 */
function fixtureWavDataURL(seconds = 0.5): string {
  const sampleRate = 8000;
  const samples = Math.max(8, Math.floor(sampleRate * seconds));
  const dataSize = samples;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);
  const writeText = (offset: number, value: string) => {
    for (let i = 0; i < value.length; i += 1) {
      view.setUint8(offset + i, value.charCodeAt(i));
    }
  };
  writeText(0, "RIFF");
  view.setUint32(4, 36 + dataSize, true);
  writeText(8, "WAVE");
  writeText(12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate, true);
  view.setUint16(32, 1, true);
  view.setUint16(34, 8, true);
  writeText(36, "data");
  view.setUint32(40, dataSize, true);
  for (let i = 0; i < samples; i += 1) {
    view.setUint8(44 + i, 128);
  }
  let binary = "";
  const bytes = new Uint8Array(buffer);
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i]);
  }
  return `data:audio/wav;base64,${btoa(binary)}`;
}

const WIDGETS_FIXTURE_AUDIO = fixtureWavDataURL();
const WIDGETS_FIXTURE_AUDIO_LONG = fixtureWavDataURL(1.5);

const WIDGETS_FIXTURE_CARD_ACTION = {
  _ui_source: "interaction",
  id: "fixture_widget_card_1",
  kind: "lesson_compare",
  type: "lesson_compare",
  status: "waiting",
  title: "节奏比较练习（fixture）",
  body: "先听示范播放器，再对比两版节奏，选择偏好并说明理由。",
  actions: [
    { id: "submit", label: "提交比较", style: "primary" },
    { id: "cancel", label: "取消", style: "secondary" }
  ],
  payload: {
    card: {
      schema: "interaction_card.v1",
      title: "节奏比较练习（fixture）",
      layout: {
        type: "collapse",
        groups: [
          { id: "fold-compare", title: "点开播放与对比", fields: ["player", "compare"] }
        ]
      },
      fields: [
        { id: "intro", type: "text", label: "说明", value: "先听 A，再听 B" },
        { id: "player", type: "audio_player", label: "示范旋律（fixture）.wav", media: { src: WIDGETS_FIXTURE_AUDIO } },
        {
          id: "compare",
          type: "compare_pair",
          label: "两版节奏对比（fixture）",
          value: "",
          context_id: "fixture-compare-1",
          reason_field: "reason",
          a: { label: "原版节奏", src: WIDGETS_FIXTURE_AUDIO, duration_hint: 12 },
          b: { label: "修改节奏", src: WIDGETS_FIXTURE_AUDIO_LONG, duration_hint: 30 }
        },
        { id: "reason", type: "text", label: "理由", value: "" }
      ],
      actions: [
        { action_id: "submit", label: "提交比较", style: "primary" },
        { action_id: "cancel", label: "取消", style: "secondary" }
      ]
    }
  }
};

function WidgetsFixturePage() {
  const fixtureCard = declarativeCardFromAction(WIDGETS_FIXTURE_CARD_ACTION);
  return (
    <div className="v5-design v5-gal-root" data-gallery-page="widgets" data-widgets-fixture="true">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 音乐控件 · fixture 渲染页</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页音频均为 data URI 静音 WAV fixture（零网络零代理），比较上下文与偏好均为样张，
          不接真实 agent 数据、不路由混音裁决；真实内核预览平面接线留后续真栈卡。
        </p>
      </header>

      <div data-gallery="widgets-audio-player" data-widget-section="audio-player">
        <h2 className="v5-gal-caption">audio_player · 收起态（标题/图标/时长/展开钮）</h2>
        <AudioPlayer title="示范旋律（fixture）.wav" src={WIDGETS_FIXTURE_AUDIO} />
        <h2 className="v5-gal-caption">audio_player · 展开态（播放/进度/时值常驻）</h2>
        <AudioPlayer title="跟练旋律（fixture）.wav" src={WIDGETS_FIXTURE_AUDIO} expandable={false} />
        <h2 className="v5-gal-caption">audio_player · 不可播诚实降级（无媒体声明，收起/展开两态均披露）</h2>
        <AudioPlayer title="缺失样张（fixture）.wav" />
        <AudioPlayer title="缺失样张展开（fixture）.wav" expandable={false} />
      </div>

      <div data-gallery="widgets-compare-pair" data-widget-section="compare-pair">
        <h2 className="v5-gal-caption">compare_pair · 异长两版各自绑定坐标（12s vs 30s，per-side，无共享轴）</h2>
        <ComparePair
          title="两版节奏对比（fixture）"
          layout="row"
          a={{ label: "原版节奏", src: WIDGETS_FIXTURE_AUDIO, durationHint: 12 }}
          b={{ label: "修改节奏", src: WIDGETS_FIXTURE_AUDIO_LONG, durationHint: 30 }}
          context={{ contextID: "fixture-compare-standalone" }}
        />
      </div>

      <div data-gallery="widgets-card-mount" data-widget-section="card-mount">
        <h2 className="v5-gal-caption">interactionCardKit 卡内挂载 · collapse 布局 + audio_player/compare_pair + 理由回流字段</h2>
        {fixtureCard && (
          <DeclarativeInteractionCard
            action={WIDGETS_FIXTURE_CARD_ACTION}
            card={fixtureCard}
            respondingActionID=""
            onInteractionAction={() => undefined}
          />
        )}
      </div>

      <footer className="v5-gal-footer">
        <p>WIDGET-KIT-1 · 本页为音乐控件渲染烟测 fixture，不接 App.tsx 业务、不出网。</p>
      </footer>
    </div>
  );
}

/* ═══════════ CONVERGE-SHELL-1（387）：收敛壳 fixture 渲染页 ═══════════
 * gallery.html?surface=converged —— ConvergedLauncher 全离线 fixture（零网络零桥）。
 * 断言面（render-smoke converged 段）：recents 磁带卡整卡（三类结构节点每行在场）/
 * 新建/打开/建议/对话流（MessageStream 文本段 + 治理卡 + directive）/Composer 输入区
 * + v5 令牌生效。所有工程名与回执均为设计 fixture（命名带（fixture）后缀自证）。
 */
const CONVERGED_FIXTURE_GOVERNANCE_VIEW = {
  requestID: "req_gov_fixture_conv_1",
  action: "open_project",
  currentProject: "示例工程甲（fixture）"
};

const CONVERGED_FIXTURE_THREAD: ConvergedThreadItem[] = [
  { kind: "user", text: "看看最近的工程" },
  { kind: "assistant", text: "最近的工程有 3 个——点卡片打开，或直接跟我说名字：" },
  { kind: "cards", cards: LAUNCHER_FIXTURE_CARDS },
  { kind: "user", text: "打开示例工程甲" },
  { kind: "assistant", text: "好的，打开《示例工程甲（fixture）》。" },
  {
    kind: "directive",
    view: directivePlaceholder({
      action: "open_project",
      file_path: "D:/design-fixtures/示例工程甲.vit",
      requires_governance: true
    })!
  },
  { kind: "governance", view: CONVERGED_FIXTURE_GOVERNANCE_VIEW }
];

function ConvergedFixturePage() {
  return (
    <div className="v5-design v5-gal-root" data-gallery-page="converged">
      <DesignSprite />
      <header className="v5-gal-header">
        <h1 className="v5-gal-title">Vit 收敛壳 ConvergedLauncher · fixture 渲染页</h1>
        <p className="v5-gal-note" data-fixture="notice">
          本页所有工程名与回执均为设计 fixture，不接真实 agent 数据、零桥调用；366
          磁带卡整卡形态与对话流复用在此核对，真实消费见生产入口 ?surface=launcher。
        </p>
      </header>

      <ConvergedLauncher
        fixture={{
          greeting: LAUNCHER_FIXTURE_GREETING,
          drawerState: "ready",
          drawerEntries: LAUNCHER_FIXTURE_CARDS,
          thread: CONVERGED_FIXTURE_THREAD
        }}
      />

      <div className="v5-gal-launcher-states" data-gallery="converged-states">
        <div data-converged-sample="empty">
          <h2 className="v5-gal-caption">空态（还没有最近工程记录）</h2>
          <ConvergedLauncher fixture={{ greeting: { reply: "我在。还没有最近的工程记录——想从哪开始？" }, drawerState: "empty" }} />
        </div>
        <div data-converged-sample="bridge-live">
          <h2 className="v5-gal-caption">桥面挂载点（live 挂载 + 宿主 chrome.webview mock 注入时「打开…」在场；agent 端点不可达=诚实错误态）</h2>
          {/* live 挂载（无 fixture prop）：gallery 页无宿主 webview 时桥不可用（345 语义
              「打开…」不出钮）；render-smoke converged 段用 addInitScript 注入 webview
              mock 后此挂载点出「打开…」并可点击取证 launcher_choose_file 桥路径。 */}
          <ConvergedLauncher />
        </div>
        <div data-converged-sample="recents-missing">
          <h2 className="v5-gal-caption">缺失置灰档（LAUNCHER-RECENTS-CLICK-1：agent os.Stat exists=false → 整卡置灰不可点 + 如实注记，列表不失联）</h2>
          {/* 383 健康度标注样例：exists=false 的条目按生产语义置灰（data-launcher-recent-missing
              + 按钮 disabled + 注记行）；render-smoke converged 段在此样例上断言。 */}
          <ConvergedLauncher
            fixture={{
              // 短问候（窄列样例：含拉丁词的问候在 3 列样例里有不可断下限会溢出，
              // 与缺失档断言面无关——主样例保留长问候覆盖；工程名仍带 fixture 自证）。
              greeting: { status: "ok", launcher: true, reply: "我在。缺失置灰样例。" },
              drawerState: "ready",
              drawerEntries: [
                {
                  id: "fixture_launcher_project_missing",
                  kind: "launcher_project",
                  type: "project_open_candidate",
                  name: "示例工程丁（fixture）",
                  filePath: "D:/design-fixtures/示例工程丁.vit",
                  exists: false,
                  directive: { action: "open_project", file_path: "D:/design-fixtures/示例工程丁.vit", requires_governance: true }
                }
              ]
            }}
          />
        </div>
      </div>

      <footer className="v5-gal-footer">
        <p>CONVERGE-SHELL-1 · 本页为收敛壳渲染烟测 fixture，不接 App.tsx 业务、不出网。</p>
      </footer>
    </div>
  );
}

const root = document.getElementById("gallery-root");
if (root) {
  const surfaceParam = new URLSearchParams(window.location.search).get("surface")?.toLowerCase();
  const launcherFixturePage = surfaceParam === "launcher";
  const convergedFixturePage = surfaceParam === "converged";
  const projectFixturePage = surfaceParam === "project";
  const loadingFixturePage = surfaceParam === "loading";
  const launcherLiveFixturePage = surfaceParam === "launcher-live";
  const projectLiveFixturePage = surfaceParam === "project-live";
  const widgetsFixturePage = surfaceParam === "widgets";
  createRoot(root).render(
    <StrictMode>
      {launcherFixturePage ? (
        <LauncherFixturePage />
      ) : convergedFixturePage ? (
        <ConvergedFixturePage />
      ) : projectFixturePage ? (
        <ProjectFixturePage />
      ) : loadingFixturePage ? (
        <LoadingFixturePage />
      ) : launcherLiveFixturePage ? (
        <LauncherLiveFixturePage />
      ) : projectLiveFixturePage ? (
        <ProjectLiveFixturePage />
      ) : widgetsFixturePage ? (
        <WidgetsFixturePage />
      ) : (
        <Gallery />
      )}
    </StrictMode>
  );
}
