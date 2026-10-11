# PULL-PROBE-TIER-EXT-1：probe 分级表扩 clip.warm_waveform_bake（主管裁定落地）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §3 分级裁定](../../rulings/2026-10-10-EVENING-BATCH-rulings.md)：clip.warm_waveform_bake 归 probe 级——D5 语义 render/probe 逐笔计入，波形烘焙=真实渲染物理工作（L2-2 实证串行瓶颈）；warm 预热属性不改变物理成本事实）
- 派发确认：已确认（裁定即授权）
- 验收负责人：GLM 主管决策流
- 池序 52；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.25h / PULL-PROBE-METER-1 已合入
- 模型分级：L0 / flash 可接

## 目标

`agent/internal/agentloop/pull_session.go` probeToolTier 分级表 probe 案增 `clip.warm.waveform.bake`（名称归一后形态——下划线/点号归一已由既有 ReplaceAll 处理，领取时以实测形态为准）；补单测：该工具回执 elapsed_ms 入账 probeSpent。

## 文件域

`agent/internal/agentloop/pull_session.go` + `pull_probe_meter_test.go`；越域即停。

## 验收标准

`go build ./...`+agentloop 包+全量 0 FAIL+gofmt 净；新单测绿。

## 停止条件

领取时该工具名/归一形态与预期不符 → 实测形态上交。

- 领取：2026-10-11 21:47 / origin/main 8e66e674 / owner=GLM-5.3-Flash（ZCode PC flash 执行流会话，用户指派双卡）/ 分支 port/pull-probe-tier-ext-1 / worktree D:/Vit_DAW_wt_probe_tier_1 / 领取提交=coord-claim-20261011-flash-l0
- 回执：commit 445d4315（port/pull-probe-tier-ext-1，基线 55c997d3，分支已推 origin）；实测名称形态=`clip.warm_waveform_bake`（message_loop.go 六处等，与卡面预期一致，停止条件未触发），ReplaceAll('_','.') 归一即达卡面 `clip.warm.waveform.bake`——probe 案已增该归一形态；测试名=TestProbeToolTierTable（warm bake 两种形态断言翻 probe，原"两属性不私定"断言按裁定翻面）+TestPullSessionWarmWaveformBakeMeteredAsProbe（新：elapsed_ms 40+2 逐笔入账 probeSpent=42、probeCostKnown 保持 true）+TestPullSessionUnlistedTierKeepsUnknownSemantics（摘除 warm bake 例改 some.future.tool，unlisted→unknown 语义本身保留）；pull_session.go 三处注释随裁定更新（两属性措辞退役+裁定引用）。
- 验收命令：go build ./... exit 0；go test ./internal/agentloop -count=1 ok；go test ./... -count=1 exit 0（92 包 0 FAIL）；gofmt blob 级净（工作树 `gofmt -l` 命中为 autocrlf CRLF 检出伪影，去 CR 后净）。
- 端测边界声明：纯单测域（卡面口径同 PULL-PROBE-METER-1 回执）；真栈 probe 成本面读数归 FS-LARGEPROJECT-SMOKE-1（P1 blocker 未解前不启动，见 ruling §5）。
- 验收：（裁定文件 / 验收 commit）
