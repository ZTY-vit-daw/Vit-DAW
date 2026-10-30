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

## 领取

- 2026-10-30 22:03 +0800 / origin/main=4d3e8023497761bef3ddfd4d5f44ba7d24e96056（注：发卡提交 d753565c 系主管本地未推，与远端 222 回执 4d3e8023 在 c80e33e7 分叉——本领取提交按 §2.1.3 从新 origin/main 重建本卡状态：发卡内容原文取自 d753565c，直接落 doing/；主管后续重推时本文件取 doing/ 版本。领取基线=本地 main d753565c，主检出预存 M/?? 与本卡文件域无交）/ owner=ZCode GLM-5.3 PC 执行会话（228 卡会话，用户指派领取）/ 分支=port/m3-track-plugin-disclose-1 / worktree=D:/Vit_DAW_wt_228 / 真栈与 227/226 错峰，占用前按 §2.2 重读登记面+即时复核进程端口
