# D3-REF-ALIGN-THIN-1：D3 参考曲对齐薄层落地（第一档=内置参考标准；复用 RLM）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗，**2026-11-07 凌晨范围修订**（依据=[用户澄清 D3 原始意图]+[PD-2 关闭记录](../../PENDING-DECISIONS.md)：第一档=内置参考标准对齐，无渠道依赖；第二档=自定义参考音频上传渠道列 backlog 小卡；audition.* 试听命令族维持缺口登记与 D3 无关——勘察报告 [2026-11-06-d3-audition-commands-recon.md](../../reports/2026-11-06-d3-audition-commands-recon.md) 留档）
- 派发确认：已确认（主管发卡+用户澄清修订）
- 验收负责人：GLM 主管决策流
- 池序 266；目标仓库=主仓 agent；P2 / 1 天 / 依赖=D2 已收口（runtime 模式先例在库）；L1-L2 / PC 执行会话；验证腿需真栈一次

## 背景

D3=参考曲对齐（not_built）：观察面已建（M1-4：RLM reference_level_model.projection.v0 复用 B1 同款 Build+user intent+RenderBindings 披露）。runtime 未建。**范围裁定（2026-11-07）**：第一档=对齐目标用 RLM 内置交付预设（builtin:spotify 等——254 的 loudnesswatch builtin 同源）+投影差距判据+建议挂确认卡；自定义参考音频上传（track.add_audio 导入+导入期 loudness+custom profile 先例拼装）=第二档另卡。

## 目标

1. **判据+验证器（零 LLM 单测先行）**：RLM 参考电平投影（builtin 预设）+user intent→对齐差距判定纯函数（verdict 三态同 D2 模式：aligned/gap_found/unmeasurable——未绑定参考标准/投影缺数据=unmeasurable）；
2. **薄 runtime**（模式同 D2 卡：verdict 落 store→建议挂确认链→queue view D3 点亮；入口识别先测裸话术过 semantic_entry）；**动作面参照 D2 先例**——若确认链现有操作无法改变对齐差距（如纯电平类建议可走既有 gain 动作则挂卡，否则 v1 纯披露+申报）；
3. **验证腿**（真栈单 run ≈¥1-3，§8 成文）；指引面不在本卡。

## 约束

- 文件域=chat D3 runtime 新文件+判据纯函数+测试+冒测加参；**零 catalog 扩面**（audition.* 不碰，第二档上传渠道不在本卡）；禁改 RLM 投影本体（消费不改写）；禁重型 stage 机；治理红线不变；持久化新字段按 §11 缺省语义。

## 停止条件

- RLM 投影字段不支撑差距判定→取证上交转 M1/RLM 系卡；验证腿断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07T00:3x+08:00 / origin/main=b5db5157（领取时最新；HEAD=工作树零 diff 亲证 status/diff --stat 均空）/ owner=GLM-5.3（PC，ZCode 主会话 L2-worker 子代理 266impl）/ 分支=port/d3-ref-align-thin-1（worktree=D:\Vit_DAW_wt_266impl，实现基线=b5db5157）/ 领取提交=6e36afa5（已推 main；首推被 270 PROGRESS 笔抢先→按 §2.1.3 重读远端无 owner 后于新基线重建重推）
- 回执（执行侧 2026-11-07 凌晨）：
  - **实现**：`port/d3-ref-align-thin-1`（已推 origin）30877292 五文件——新 `agent/internal/chat/d3_ref_align_runtime.go`+test：判据纯函数 `d3AssessRefAlignment`（参考标准解析次序固定：投影 RenderBindings 首个 bound 行→`rlm.LookupDeliveryProfile`（消费不改写投影）；否则意图点名 builtin（词法层：spotify/ebu/apple/aes/gy|国标，ASCII 边界防 aesthetics 误中）；两者皆缺=unmeasurable `no_reference_standard` 一等公民。差距计算**委托 loudnesswatch.Compare**（builtin: ProfileSource 直通，继承 approximate 口径限定词/band 两侧语义/peak 必要条件语义，不另立标准）；verdict 三态 aligned/gap_found/unmeasurable 映射 LW pass/breach/unmeasurable）；verdict store `ref_align_verdicts.v0`（LW/D2 同款：外来 schema fail-closed/dedupe key=uuid|rev|kind/trim50/失败只 WARN 不挡响应）；`d1d3` handler 显式 D3 分支；queue view D3 挂 `pre_master.ref_align.v0`（mix_workflow_queue_view.go 节点目录+注释）；冒测加参 `-D3RefAlign`（run_free_state_d1_smoke.ps1 +315 行：face P 裸话术 semantic_entry 探针「帮我看看这首歌跟 spotify 的标准差多少」+门 A check turn[stop_reason=d3_ref_align_checked/三态/profile=builtin:spotify/basis/grade/零 mutation]+门 B ref_align_verdicts.json 落盘[UTF8 读取]+门 C 动作面诚实[gap+可定位=挂 mix_tick_confirmation 卡零 mutation+取消腿 mix_tick_rejected；其余=无卡披露]+门 D mix.report D3 节点点亮；PARSE_OK 亲证）。**py 零改动**（D3 门组全部为 runner 后 ps1 腿，与 D2 同款——ps1/py 透传缝无需扩展）。
  - **置顶申报①（动作面=挂卡，与 D2 纯披露不同，请主管裁定）**：第一档对齐差距全部是电平类（integrated band 越界/sample peak 超 TP 上限），既有 `loudnesswatch.SuggestGainAction` 有界 track_gain_adjust 可诚实改变它们——按卡面「纯电平类建议可走既有 gain 动作则挂卡」实现为 **gap_found+可定位载体→挂既有 mix.propose_tick→确认→mix.apply_tick 治理链卡**（D1 settle 同款，确认前零 mutation，完全访问模式 auto-apply 同 D1 改判先例）；建议 nil（最越界 clip 解析不出轨道/剂量钳零）→纯披露+置顶理由，不发明动作。动作面三值 payload：gain_card_v1/none_needed/disclosure_only_v1。
  - **置顶申报②（入口基线，263 教训应用）**：检查请求词（参考对齐/参考曲对比/对齐检查等）**不进** inferProjectAwareCapability（单测钉死 TestD3RefAlignCheckPhrasesStayOffCapabilityEntryWordList）；深面入口=显式检查意图词（d3RefAlignIntent，在 pack 内 handler 分支，词法命中先于 semantic_entry）+既有 d3 标记（M1-4 语义零改动）。裸话术能否过 semantic_entry=face P 探针，真栈验证腿留证。
  - **零栈验证**：15 个零 LLM 单测全绿（`go test ./internal/chat/ -run TestD3RefAlign -count=1`）；chat 全包 `-count=1` 119.9s 绿（零回退）；`go build ./...` 全量绿；loudnesswatch/rlm/mixboard 消费包全绿。**真栈验证腿未跑**（按派发令留主管栈调度；276 rerun1 占用中未动栈）。
  - **真栈验证腿就绪度**：`scripts/run_free_state_d1_smoke.ps1 -D3RefAlign`（可与 -LoudnessWatch/-D1LoudnessSettle/-D2StereoCheck 叠加）门组就绪；§8 跑前口径预写：**单 run 无重跑**；预算 ≈¥1-3（face P 一腿 LLM，门 A-D 零 LLM；fixture 无 render 绑定→basis=intent_named 判定腿确定走意图点名面）；失败分类=门 A-D 断言失败/face P 仅记录不挡门/环境中断（仅原始工件在盘时可排除）；止损线=同一代码版本同断点连续两败→取证上交不重跑；预期 verdict=fixture 依赖三态如实（测量行缺席则 unmeasurable 如实，不凑数据）。
  - **域外观察**：main a775cf96 并入 270 的 ps1 两行 -Encoding UTF8（:445/:630）与本题 ps1 改动（参数块/互斥块/D3 门组 ~1560+）区域不相交，主管合入时 git 可自动合并（已在位核实 diff 无重叠行）。
  - **端测边界声明（AGENTS §5）**：本卡改动=agent 判据/runtime/queue 挂点+冒测脚本——未触 webui 渲染面与 demo 用户旅程，两门槛（E2E-WEBUI-1/JOURNEY-1）不适用；真栈验收=run_free_state_d1_smoke.ps1 -D3RefAlign 门组（本腿未跑，留主管调度）。
- 验收：（主管回填）
