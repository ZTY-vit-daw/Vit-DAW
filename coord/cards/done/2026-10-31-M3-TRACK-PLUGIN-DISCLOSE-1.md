# M3-TRACK-PLUGIN-DISCLOSE-1：观察面轨级插件披露——内核紧凑面白名单放行（222 次生面，227 重验腿前置）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=222 次生面定案：模型视角「轨无插件」vs 内核真值轨级 Volume&Pan 在场——剥除发生在内核紧凑归一化；锚点在卷）
- 派发确认：已确认（222 验收处置「另拆卡」+227 重验腿前置裁定）
- 验收负责人：GLM 主管决策流
- 池序 228；目标仓库=**主仓（内核 VitApp+agent）**；P2 / 0.5 天 / 依赖=无（与 227 开发面并行，其重验腿等本卡）；L1 量产档可做（锚点已定案）

## 标的（222 定案锚点，不重考）

- 剥除点：`VitApp/Source/Service/VspKernelReference.cpp:603` `normaliseTrackArray` 白名单不含 `plugins`/`plugin_count`（轨级 Volume&Pan 内置插件非 rack 节点，紧凑态只有 volume_db/pan 数值+plugin_param_digest 哈希，plugin_id 不可见）。
- 数据在：内核完整序列化面 `TrackService.cpp:120` `createTrackState` 有 plugins。
- agent 消费链已备：mixboard `project_package.go:163` `buildTrackSummaries` 已消费 `plugins/rack_nodes/plugin_chain`；放行点=`capabilitycontext/free_state_observation.go:997` `compactProjectTracksSummaryForStructure` 6 字段白名单。

## 目标

1. 内核紧凑面白名单放行轨级 `plugins`（最小集：plugin 条目含 id/name/type，不带参数全量——观察面够用即可，防 payload 膨胀；**只读披露，非执行面**）。
2. agent 侧 `compactProjectTracksSummaryForStructure` 白名单同步放行（LLM 观察面=free-state 轮模型可见轨级插件与 plugin_id）。
3. 披露边界维持 CCB bounded disclosure 语义：投影是 peer 非链、不自动升 readiness（AGENTS §5 红线照旧）。

## 探针义务

- 单测：紧凑面往返（含 plugins 轨→快照含条目；旧态无 plugins 轨→缺省空数组不报错——§11 兼容）；agent 侧白名单前后对照。
- 真栈腿：跑一轮观察面快照断言（轨级 Volume&Pan 条目在场+plugin_id 可见）——可并入 227 重验腿前置检查或独立最小轮，exit 0 为门。

## 文件域

`VitApp/Source/Service/VspKernelReference.cpp`（白名单）+`agent/internal/capabilitycontext/free_state_observation.go`（agent 白名单）+测试。越域即停。

## 停止条件

- 紧凑面 payload 体积回归（超大工程快照膨胀超既有阈值）→锚点+最小集重裁上交；
- 放行触及 VSP 紧凑面契约其他消费方（207 快照守卫依赖剥列形态的地方）→锚点上交（**注意：前端 207/208 守卫族正是防剥列驻留——本卡放行后剥列形态改变，须核对前端守卫语义零回退**，前端回归=probe_vsp_delta_schema_guard/probe_lane_catalog_fail_loud 两族）。

## 回执（执行侧自验，2026-10-30 22:40 +0800）

- 实现 commit：`606ba2d0`（分支 `port/m3-track-plugin-disclose-1`，已推远端；分支尖=`e10d5a6d`=工件 `dbfb2c6a`+运行时工程文件剔除 `e10d5a6d`；领取=main `b42c93c7`，登记=`874a4afe`）。8 文件 744+/2-。
- **双 diff 锚点**：
  - 内核=`VitApp/Source/Service/VspKernelReference.cpp:673-676`（`normaliseTrackArray` 接线 `compactTrackPluginDisclosure`：轨行 `plugins` 最小集+`plugin_count`，紧跟 `plugin_param_digest` 之后）+新纯函数头 `VitApp/Source/Service/TrackPluginDisclosure.h`（84 行，含 §11 缺省空数组语义）；
  - agent=`agent/internal/capabilitycontext/free_state_observation.go:1012`（`compactProjectTracksSummaryForStructure` 轨行白名单 6→8 字段：+`plugin_count`+`plugin_chain_summary`；裸标量 `plugin_id` 仍排除——既有锁定测试 `TestProjectStructureViewKeepsTrackIdentityFromTypedRows` 语义不变、原样通过）。
- 测试与回归退出码（全 exit 0）：
  - 内核单测 `VitTrackPluginDisclosureTests`（新目标，Tests/CMakeLists 注册；最小集压缩/旧态缺省空数组/JSON 往返三面）=exit 0；
  - 内核整体编译=worktree `build_release` Release exit 0（产物 sha256 `f9e3da0a…`）；
  - go `build ./...`+`test ./... -count=1`=exit 0（96 包全绿，含 capabilitycontext 新两测：放行对照+全量面不泄漏；旧态稳定零幻影键）；
  - **前端 207/208 守卫族**：`probe_vsp_delta_schema_guard` 30/30 exit 0+`probe_lane_catalog_fail_loud` 18/18 exit 0；另亲读两守卫函数确认空数组 `plugins:[]` 满足判定（208=`_project_state_tracks_schema_ok` typeof 数组判据、207=`_vsp_snapshot_tracks_have_plugins` 任一轨行判据）——剥列形态改变后守卫语义零回退，剥列驻留防御仍 fail-loud；
  - 真栈腿=`dev_agent_smoke.ps1 -StartKernel -RestartAgent -TrackPluginDiscloseSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe <worktree 内核>`=**exit 0**（新场景旗标纯增量+断言体 `scripts/track_plugin_disclose_smoke.py`；内核腿 6/6 轨数组型 plugins 在场+全部披露 type=volume 条目样例 1007→`{"id":"1008","name":"Volume & Pan Plugin","type":"volume"}`+条目仅 id/name/type；agent 腿 6/6 视图轨行链摘要披露 plugin_id=1033 可见+disclosure 14,660B+视图内 `"plugins"`/`"params"` 全量面键零泄漏+read_only=true）。run=`coord/runs/M3-TRACK-PLUGIN-DISCLOSE-1/trackdisclose_20261031_1`（RESULTS.md 在卷）。
- 红线核对：CCB bounded disclosure 维持（视图级白名单放行非 raw 通道；预算 14,660/65,536B）；投影 peer 非链零改动；无 readiness 自动升级（视图 status 语义未动）。零 LLM 轮（零计费）。
- **端测边界声明**：本卡零 webui 文件改动→未跑 `npm run test`（渲染面门槛不涉及）；用户旅程面未另跑旅程烟测（本卡为观察面披露，非交互旅程改动）——旅程级回归由 227 重验腿（本卡合入后）整体覆盖。
- 协作备注：领取时主管发卡提交 `d753565c` 未推远端（与 222 回执 `4d3e8023` 分叉）——领取提交按 §2.1.3 从新 origin/main 重建卡面（原文取自 `d753565c`）；227 领取 merge 带入的 todo/ 陈旧副本已清除（`20489009`）。

## 领取

- 2026-10-30 22:03 +0800 / origin/main=4d3e8023497761bef3ddfd4d5f44ba7d24e96056（注：发卡提交 d753565c 系主管本地未推，与远端 222 回执 4d3e8023 在 c80e33e7 分叉——本领取提交按 §2.1.3 从新 origin/main 重建本卡状态：发卡内容原文取自 d753565c，直接落 doing/；主管后续重推时本文件取 doing/ 版本。领取基线=本地 main d753565c，主检出预存 M/?? 与本卡文件域无交）/ owner=ZCode GLM-5.3 PC 执行会话（228 卡会话，用户指派领取）/ 分支=port/m3-track-plugin-disclose-1 / worktree=D:/Vit_DAW_wt_228 / 真栈与 227/226 错峰，占用前按 §2.2 重读登记面+即时复核进程端口

## 验收（主管裁定 2026-10-31，GLM 主管决策流）

- **整卡 pass**：双 diff 锚点亲核（内核 VspKernelReference.cpp 白名单放行最小集+agent 6→8 字段，裸标量 plugin_id 排除=锁定语义不变）；§11 稳定缺省+防膨胀设计正确；真栈 6/6 双面断言（VSP 快照+CCB 链摘要，bounded disclosure 红线核对过）；**前端 207/208 守卫族回归 18+30 检零回退**（剥列形态改变核对面兑现——本卡最大风险面）。合入 fdb8bc12。
