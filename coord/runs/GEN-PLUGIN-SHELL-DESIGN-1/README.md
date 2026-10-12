# GEN-PLUGIN-SHELL-DESIGN-1：生成插件壳设计基线（总览）

- 执行：GLM-5.3 ZCode 执行流会话（Mac）/ 2026-10-12；领取 origin/main=813cbf58（领取提交 47cd999e）；worktree `~/Documents/Vit-DAW-gpsd1` / 分支 `port/gen-plugin-shell-design-1`
- 性质：设计卡，零生产代码（全部产出=本目录设计件；对仓库源码零 diff）
- 定位：生成集成面②——自研 VST3 生成插件壳（轨道级生成便利服务，脱离 agent 总对话流）；消费统一化契约（GEN-UNIFY-CTX-DESIGN-1）的**产品消费方形态**；生成动作走 genapi 方言面（件四 §3）

## 1. 设计五件+风险表（索引）

| 件 | 文件 | 内容 |
|---|---|---|
| 一 | [01-channel-contract.md](01-channel-contract.md) | 三段式总体形态（壳/宿主服务/agent runner）+通道分层 T0-T3（IProcessContext/参数面/IConnectionPoint/chunk 各司其职）+T2 消息信封 vit.genshell.msg.v1+T3 chunk §11 语义+上下文真值单一性（对统一化件三逐条对齐核对）+实现基裁定（VST3 SDK 直实现+VSTGUI） |
| 二 | [02-async-job-model.md](02-async-job-model.md) | 作业生命周期（复用 VitAIGCJobRuntime 零结构改动，job_state 字符串扩值域）+提交请求结构+失败分类（genapi 11 类透传+新增 3 类）+队列轮询可靠性模型（不依赖 PUB）+取消语义+缓冲播放+fast/print 双落轨路径+孤儿清扫 |
| 三 | [03-template-gui.md](03-template-gui.md) | 核心裁定：模板=USER 块编排器永不触碰自动块+模板 schema（vit.genshell.template.v1，字段四类型受控）+内置四模板+GUI 六区+scene 消费映射（free_creation 不落轨/experiment_control 不进 GUI）+dry-run 预览=实发+genctx 对接清单 |
| 四 | [04-pca-admission.md](04-pca-admission.md) | PCA 现状锚点（加载门 agent 侧/family 9 枚举无 generator/生成动作游离 PCA）+"可控即达标"三条操作语义+generation_shell family 词表扩展（coverage v1 空集）+genapi 关系五条钉死+落轨分工（PCA 管装载不管产物） |
| 五 | [05-impl-card-specs.md](05-impl-card-specs.md) | 实施卡 6 张（CPP 骨架/宿主服务/agent 执行腿/GUI 模板/PCA/试听打印——每卡可直接发卡）+序列并行图+互斥文件清单 |
| 风险表 | [06-risk-register.md](06-risk-register.md) | 六项逐条结论（卡面点名三项+勘察新增三项：SDK 许可/签名分发/异步边界/通道检索锚点/实现基选择/壳崩溃隔离） |

## 2. 卡面验收项逐条结论

| # | 验收要求 | 结论 |
|---|---|---|
| 1 | 设计五件齐 | ✅ 本目录件一~件五+风险表 06+本总览 |
| 2 | 通道契约细化（上下文下行/状态上行/生成参数面/PromptFace 与模板引擎关系） | ✅ 件一：上下文下行=T2 context_snapshot（展示副本）+T0（展示）；状态上行=T1 五参数（JobState/Progress/ErrorClass/SceneMode/Bypass 枚举定稿）；生成参数面=submit_request 载荷（件二 §2）；PromptFace 与模板引擎关系=件三 §1（模板只编排 USER 块、自动块 genctx 独家）+件一 §6（真值单一性） |
| 3 | 异步作业模型（生命周期/状态参数枚举/取消/失败分类/缓冲播放） | ✅ 件二全件：生命周期 9 态+取消链（cancel_requested→ctx cancel→cancelled，provider 侧不可真取消如实声明）+失败分类 14 类（11 透传+3 新增）+缓冲播放（instrument 图内流播+线性重采样+落轨后 clip 权威） |
| 4 | 模板引擎+GUI 引导（含 gen_plugin profile 对接） | ✅ 件三：schema 定稿+内置四模板+GUI 六区+对接清单（件三 §5 逐条：profile 只裁不降/scene 调用级/用户块保真/未设置项可见/C 形态无机检/快照来源） |
| 5 | PCA 接入面（可管可控准入+落轨分工） | ✅ 件四：三条操作语义（agent 装载过门/作业全审计/密钥零内核）+family 词表三处扩展+落轨分工（PCA 管装载不管产物，ingest 链零新增面） |
| 6 | 实施拆卡达可直接发卡 | ✅ 件五：6 张（每卡目标/文件域/验收（命令+exit 0 断言）/停止条件/依赖/预估+互斥清单+序列图）；关键依赖 GEN-CTX-INJECT-1 已钉在卡 3 |
| 7 | 通道契约与统一化件三对齐无冲突 | ✅ 件一 §6 逐条核对（profile 单一传递/C 形态边界/scene 枚举/用户块保真/未设置项可见）+件三 §3 scene 消费映射（free_creation 不落轨=件四 §3 产物落点判据落实）+件四 §3 gen_plugin 双使用者共用渲染器零第二实现 |
| 8 | 风险表逐条结论 | ✅ 06：VST3 SDK 许可=Steinberg 协议路线（双许可闭源可行+VSTGUI Apache-2.0）；签名分发=自产自宿主 ad-hoc 可用/对外分发补 Developer ID；异步边界=六子项逐条裁定 |

## 3. 关键设计裁定汇总（主管验收聚焦点）

1. **三段式架构**：壳（内核进程内，VST3 instrument+GUI）→宿主 GenerationShellService（队列+通道+ghost）→agent genshell runner（genctx 渲染+genapi 执行+ingest 落轨）。壳零协议实现、零密钥、零自动块渲染。
2. **通道分层 T0-T3**（前提 3"任选"的升级：不是任选其一而是各司其职）——IProcessContext=展示、参数面=状态投影（厂商可达下限）、IConnectionPoint=富通道（消息信封 v1）、chunk=持久化；降级序在案（件一 §2）。
3. **作业模型零内核结构改动**：复用 VitAIGCJobRuntime 全既有面（upsert/attach/setGhostState/snapshot），仅 job_state 字符串扩值域（§11 兼容声明）；唯一新内核命令=gen_shell.pending（读，RiskDirect）。
4. **可靠性=轮询契约**：队列（内核+壳 chunk 双持久）+agent 2s 轮询 at-least-once；不依赖 PUB at-most-once；孤儿清扫→interrupted。
5. **模板=USER 块编排器**：可自调性与单一渲染器不冲突的机制解（模板管用户意图的专业表达格式，引擎管工程约束自动块）；预览=dry-run 同路径（预览=实发）。
6. **scene 落实**：in_project 默认强制落轨；free_creation 可选且不落轨（产物缓存目录+手动导入）；experiment_control 不进 GUI。
7. **PCA 最小扩展**：generation_shell family（词表三处）+coverage v1 空集+本地认证；生成动作准入=作业审计面（不与处理器控制准入混编）；typed apply=v2 停止条件在案。
8. **实现基=VST3 SDK 直实现+VSTGUI**（不走 JUCE 插件包装）：T2 控制权+免 patch 子模块+厂商标准栈；fallback（JUCE+通道降级 A）入风险表。

## 4. 交叉引用与依据清单

- 卡面：coord/cards/doing/2026-10-11-GEN-PLUGIN-SHELL-DESIGN-1.md（领取回填在案）
- 统一化契约：coord/runs/GEN-UNIFY-CTX-DESIGN-1/（件一~件五；本设计消费其件一 §4 快照/件三渲染器与机检/件四 scene）
- 生成 API 适配层：agent/internal/genapi/（GEN-API-ADAPTER-1，已合入；关系定位=件四 §3）
- MIDI 命令面与落轨基线：coord/runs/GEN-MAC-BASELINE-1/report.md（30/30 门=render 两腿+assets 六门实证本设计复用链）；coord/runs/GEN-API-ADAPTER-1/report.md（17/17=genapi 产物 ingest 同栈先例）
- 相关卡：GEN-CTX-INJECT-1（runner 硬前置）/GEN-CTX-CONSUME-1（编排线消费方，后置编曲口径）/AIGC-DRUM-GEN-1（"生成≠MIDI"裁定出处，状态修订在案）/GEN-VOCAL-SYNTH-1（SVS=壳的天然后端形态注记，件四 §3 条 5）
- PCA 文档族：docs/PCA_PLUGIN_LOAD_GATE_V1.md（历史）+PROCESSOR_CONTROL_ATTESTATION_V1/V2.md；RLM-PROFILE-2（renderResults 终态缓存消费先例）
- 端测体系：scripts/gen_chain_survey_smoke_mac.sh / gen_api_adapter_smoke_mac.sh（新卡烟测模式先例）

## 5. 工件清单

```
coord/runs/GEN-PLUGIN-SHELL-DESIGN-1/
  README.md                    ← 本文件
  01-channel-contract.md       设计件一：通道契约
  02-async-job-model.md        设计件二：异步作业模型
  03-template-gui.md           设计件三：模板引擎+GUI 引导
  04-pca-admission.md          设计件四：PCA 接入面+genapi 定位
  05-impl-card-specs.md        设计件五：实施拆卡（6 张）
  06-risk-register.md          风险表（六项逐条结论）
```

## 6. 端测边界声明（AGENTS §5）

本卡零生产代码：无 webui/渲染面/用户旅程改动，无 agent 行为改动；全部产出为设计文档。锚点均为只读勘察（文件:行+grep 实证，出处逐条内嵌于各件；关键锚点抽样亲验：harness.go:1942-1949 / CommandDispatcher.cpp:2784-2803,3168 / VitAIGCJobRuntime.h:8-35 / genapi errors.go+config.go+gen_generate.go）。无真栈执行需求（GEN-MAC-BASELINE-1 run4+GEN-API-ADAPTER-1 run4 已提供复用链行为证据，本卡引用不重跑）。实施卡各腿的端测边界声明见件五。
