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
- 验收（2026-11-07 凌晨，主管·实现面 pass，终验待验证腿）：判据设计合格——参考标准解析（投影 RenderBindings→builtin 点名→unmeasurable 三态）+**差距计算委托 loudnesswatch.Compare**（继承 approximate 口径不另立标准=正确复用）；**动作面挂卡判定追认**（置顶申报①）：电平类差距有既有 gain 动作链（SuggestGainAction→mix.propose_tick 确认链=D1 settle 同款）→挂卡正确，与 D2 纯披露的区分=「有既有动作则挂、无则披露」原则两卡一致应用；无可定位载体/剂量钳零→纯披露红线维持。5 文件 +1499，15/15 单测+chat 全包+消费包全绿+主管交叉复跑 D3 聚焦 ok；-D3RefAlign 门组与 270 修复区不相交。验证腿进栈队列（276 段 3b→269→266）。
- 验证腿回执（执行侧 2026-11-07 01:12–01:24，主管派发令执行）：
  - **前置合入**：port 分支合入 origin/main fe8ab880（276 整卡+269 终验+277 段1）**零冲突**（ps1 两行 -Encoding UTF8 与 D3 门组区域不相交自动合并亲证）；合并后 `go build ./...` exit 0+chat 全包 122.1s ok（merge 49a85a25）。
  - **占栈协议全程**：净空亲核 01:05 全零→占用登记 6b3157a1 推 main→01:11:23 ≤60s 即时复核全零→01:12:03 启栈（kernel wt_228 F9E3DA0A/agent worktree 预构建 F4B5A215 双哈希活栈回读亲证）→01:23:56 **TEARDOWN_ALL_ZERO 亲核**（六口 TCP+UDP 4444/4445+四类进程+python 全零）→释放登记（本笔）。启动腿一次 bash 重定向路径失误（coord 分支上 artifacts 目录不存在）在**栈启动前**自纠——run 未消耗、栈未启动，非环境中断。
  - **单 run `20261107_011211`（01:12:11–01:17:54）：PS1 exit 1——§8 预写②门断言失败兑现，取证上交不重跑（单 run 授权用尽）**。失败点=**已收口 D1-settle 门组 gate A 观察面子断言**（loudness_peak_status=missing expected present）——**非本卡 D3 判定面**（D3/D2 门组脚本次序在后未及执行）；其余已跑面全绿（runner 1-mutation 主链 PASS+LW 门 1-4+E1+D1-settle stop_reason/verdict=breach/挂卡/取消腿/节点点亮）。**取证锚点**：kernel digest 自本 run 第一次 observe 起即无 D1 全局面（loudness_watch_observe1 result.digest：stereo ready 而 peak/rms/crest 全缺）+settle turn mix.observe status=ok 而 observation missing 无 observe_error——归因候选（四开关史上首跑/fixture 观察面跨 run 漂移[265 reverify 当日 stereo missing 本 run ready，同一 kernel 二进制]/fe8ab880 合入面）**交主管裁定，本腿零代码改动零扩查**。计费 7 调用 deepseek-flash in+cache ≈22.7k tok ≈¥0.01-0.05（D3 face P 探针未及执行，预算 ≈¥1-3 远未触）。
  - **D3 验证腿状态=未验证（门组未及执行）**：判据/runtime/门组代码零改动（零代码边界遵守）；是否重派验证腿（含降级 -D3RefAlign 单门组避开叠加交互）与 D1-settle 观察面子缺陷处置（域外锚点 ps1:1100-1105+d1d3 组装面+kernel digest 生产面）交主管。
  - 工件：coord/runs/D3-REF-ALIGN-THIN-1/verifyrun1/（RUNBOOK+console+ps1_exit+d1_settle/LW 证据+telemetry 快照+agent log 节选+binary sha+evidence.md，fc0790ef 随 port 分支已推）；栈占用/释放在 PC-RUNTIME-STACK.md。
- **验证腿裁定（2026-11-07 凌晨，主管）：域外失败+重派授权**——失败点=已收口 D1-settle gate A 观察面子断言（loudness_peak missing）且 D3 门组未及执行（脚本序在后）≠本卡断言失败重跑；归因候选三条在档（四开关叠加交互=派发裁量引入变量[主管责任]／fixture 观察面跨 run 漂移[265 当日 stereo missing vs 本 run ready=kernel 观察面选择性缺失非确定性证据]／合入面）；**授权重派单门组 -D3RefAlign**（变量收窄）；**D1 观察面选择性缺失挂账观察**（单次形态不定性，POOL-INDEX backlog 记录，再现即开取证卡）。其余已跑面全绿（runner 1-mutation 主链+LW 门+E1+D1-settle 大部）。计费 ¥0.01-0.05。
- 验证腿重跑回执（执行侧 2026-11-07 01:30–01:39，主管重派单门组授权执行）：
  - **verifyrun2 `20261107_013013`：PS1 exit 0——§8 ①PASS，D3 门组真栈全绿**（重派授权=上 ruling 行；单 run 第二次也是最后一次授权，无重跑兑现）。**face P**：裸话术「帮我看看这首歌跟 spotify 的标准差多少」过 semantic_entry（stop_reason=limit_reached 非 unresolved）——**置顶申报②不建词表基线真栈成立**；**门 A**：stop_reason=d3_ref_align_checked，payload verdict=**aligned**（fixture 依赖如实：19 载体均在带内）/profile=**builtin:spotify（intent_named）**/grade=approximate/band≤-14 披露/零 mutation；**门 B**：ref_align_verdicts.json 在盘携带 d3_ref_align trigger；**门 C**：aligned→action_face=none_needed 无卡零 mutation 诚实披露（挂卡分支 15 单测钉死+D1 settle 同款链在档，本 run fixture 如实产 aligned 不硬凑 gap）；**门 D**：D3 节点 not_started→in_progress；D1 runner 主链 PASS。
  - **挂账观察项（裁定：记录不追查）再现**：D3 check turn 观察 loudness_peak_status=missing/global 全 None/stereo missing——digest 全局缺席形态在**单开关 run 复现**（叠加交互候选①收窄排除后仍在→支持 kernel 观察面选择性缺失非确定性候选②）；同 run 判据消费面 19 载体测量行健康（digest 披露面 vs 测量行判据面两源分离按设计）。证据 verifyrun2/d3_check_chat.json+verifyrun1/evidence.md 对照。
  - 占栈协议全程：净空 01:29→登记 ef87f3b4 推 main→01:30:06 ≤60s 复核全零→01:30:13 启栈→**TEARDOWN_ALL_ZERO 亲核 01:39:13**→释放登记（本笔）。被测不变量：agent F4B5A215/kernel wt_228 F9E3DA0A 双哈希 01:29 复核未变；引擎默认无注入。
  - 计费 19 调用 deepseek-flash in+cache ≈179.8k tok ≈**¥0.1-0.3**（预算 ≈¥1-3 内；两腿合计仍低于单腿预算上沿）。
  - **D3 验证腿就此闭合：exit 0 达成，终验签署留主管。**工件 coord/runs/D3-REF-ALIGN-THIN-1/verifyrun2/（12 件，port 0b581d2f 已推）。
- 卡面完整性注记（执行侧 2026-11-07 01:4x，发现即报）：本 doing/ 卡面在本笔前仅存 ruling 单行（395 字符）——dd043f73 在 doing/ 建档时只落了裁定行、30c3cd02 又删除了 done/ 全本（44 行），完整卡面（领取/回执/验收+verifyrun1 回执）一度仅存 git 历史（e2ef051f/a25e72d2）。本笔以 a25e72d2 done/ 全本为底**原样重建** doing/ 卡面（零改写），ruling 行（dd043f73 原文）与本回执追加在后；提交说明如实记录。请主管复核重建。
