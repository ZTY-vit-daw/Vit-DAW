# RLM-PROFILE-2：交付 profile 消费方接线——render 绑定（一 render 一 profile）+RLM 披露面+计量现状实勘

- 优先级 / 预估 / 依赖：P1 / 0.6–0.8 天 / L2-1-RLM-1 已合入（rlm/profile.go 数据层+内置 profile 在案，2026-09-27 pass）；蓝图 D9（AGENTIC_OBSERVATION_ROADMAP §2 D9）；**文件域注意：若需改 capabilitycontext 则与 CCB-PARAM 交叉，须在其合入后领取——本卡目标是零触碰 capabilitycontext（披露走 rlm 投影自带输出面）**
- 模型分级：L1 / Mac 端可接（纯 agent 侧 Go）；flash 亦可
- 已核实事实：DeliveryProfile/RenderProfileBinding schema（rlm.delivery_profile.v0 / rlm.render_profile_binding.v0）+内置 profile（EBU R128/AES TD1004/Apple Music/GY/T 282-2014）在 profile.go；**全仓零消费方**（grep 仅 rlm 包内自引用）；RLM Build/ContextProjectionMap 现行为零改动（上卡验收口径）。
- 待证假设（领取时实锚）：render 面挂点（render.start/交付物命令面的稳定绑定位置）；kernel 流式计量现状（VitApp/Source/Service/AudioFeatureTypes.cpp 含 loudness 面——形态与可消费性待勘）。
- 目标：
  1. **render 绑定接线**：一个 render 挂一个 profile（schema 已在案）——找到 render/交付面的稳定挂点接上；绑定关系持久化随工程（§11 旧态缺省=无绑定，行为不变）。
  2. **RLM 披露面**：RLM 投影输出披露当前绑定 profile 与断言语义（目标带不是单向阈——LoudnessBand 语义已定），走 rlm 包自带投影输出，fail-closed。
  3. **计量现状实勘（入报告不强制接线）**：kernel AudioFeatureTypes loudness 面形态、render 后/播放中监督的可行落点——供后续流式监督卡。
  4. **门**：cd agent && go build ./... && go test ./... -count=1 全量 0 FAIL + vet/gofmt（blob 级）+ rlm/capabilitycontext 契约测试（含反例：未知 profile ID 拒绝/无绑定旧行为不变）。
  5. 回执：接线锚点清单+实勘报告+红绿+泊位声明+HEAD。
- 文件域：agent/internal/rlm（主）+render/交付面挂点（领取时实锚）；capabilitycontext 零触碰（若避不开→停止上交，与 CCB-PARAM 排序）。
- 验收标准：绑定+披露接线绿+全量 0 FAIL+旧行为零变化（无绑定工件往返不变）
- 停止条件：render 面无稳定挂点（命令语义缺口）→ 实证清单上交，不擅扩内核命令面
