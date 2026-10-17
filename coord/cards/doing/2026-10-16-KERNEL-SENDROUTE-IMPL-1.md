# KERNEL-SENDROUTE-IMPL-1（K1）：send/return 内核命令面实现

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=MIX-SENDRETURN-DESIGN-1 设计件评审 pass+GUI 定稿 §9；用户「一起执行，好好安排」）
- 派发确认：**已确认（设计评审+GUI 定稿链完整，用户 2026-10-16 授权安排）**；验收负责人：GLM 主管决策流
- 池序 127；目标仓库=D:\Vit_DAW；P1 / 2-3 天 / 无依赖（可即领）
- 模型分级：L2 / GLM 执行会话（内核 C++ 面）

## 规格（权威=设计件 [coord/runs/MIX-SENDRETURN-DESIGN-1/README.md](../../runs/MIX-SENDRETURN-DESIGN-1/) §1+§9）

1. **SendRouteService**（新文件，域清晰勿并入 PluginRackControlService）：5 命令——`track.send.connect`（含 `auto_create_fx_track(name)`：单命令事务=add_track+aux 插件对插入+建边；**undo 原子单事务/幂等重发不重建/重名追加序号**）、`track.send.set_amount`、`track.send.disconnect`（成对移除+FX 轨零源时拆内部节点回收 bus 对）、`track.send.list`（拓扑读回=观察面唯一读源）、`track.send.set_fx_role`；
2. **双 bus 三明治**：每 FX 轨一对 bus（send_bus 汇干/return_bus 播湿），分配器取最小未占用偶数对，轨删除回收；FX 轨=普通混合轨+`vit_fx_role`/`vit_send_bus`/`vit_return_bus` 属性标注（零新轨类型）；
3. **防环三道闸**：命令面拓扑 DAG 校验（FX 轨不得作 send 源）+封装层链序强制（源轨 AuxSend@SendOut 位/AuxReturn@SendIn 位；乱序命令面不可达）+引擎图级剔除兜底；
4. **渲染收尾**：connect/set_amount 后 restartPlayback+ensureContextAllocated（对齐 handleSetFolderRoutingBus 模式）；
5. **侧链（控制发送）v1**：`track.send.connect{target{kind:sidechain}}` 走 tracktion 原生 `Plugin::setSidechainSourceByName`（tap 固定链后，无量），统一 schema 已定（§9.0-2），实现映射原生；
6. **测试**：SendRouteTests 收编底座五腿断言（多目标回流/单回授/静音/乱序有界）+旧工程 fail-open 兼容（无新属性轨=普通轨）+新工程旧内核 unknown-plugin 边界行为记录；烟测脚本按设计件 §5 场景 1 扩展。

## 文件域

VitApp/Source/Service/{SendRouteService.cpp/.h,CommandDispatcher.cpp}、VitApp/Tests/、scripts/；领取前与决策侧核对 seam。

## 验收标准

- CMake 构建 exit 0 + SendRouteTests 全过（五腿+兼容+防环拒绝）；
- 真栈烟测 exit 0：connect→list 读回核对→渲染断言湿分量→disconnect→list 归零对照；乱序/环配置命令面拒绝断言；
- auto_create 一步化验收：undo 一次全撤、重发幂等、重名序号；
- go test ./... -count=1 0 FAIL（agent 零改动不回退）。

## 停止条件

- tracktion aux 原语在 VitApp 封装层（graph swap/revision 体系）下行为与探针实证不符 → 停手上交（探针为直接驱动，封装层差异属架构级）；
- 侧链原生面在命令面暴露遇阻 → 侧链腿如实记录挂账，音频发送腿独立交付，不硬凑。

- 领取：2026-10-17 早窗 / origin/main=5942bb65 / GLM-5.3 L2 PC 执行会话（ZCode，本卡会话）/ 分支 port/kernel-sendroute-impl-1（worktree D:/Vit_DAW_wt_sendroute1，领取基线 HEAD=5942bb65，status 干净，diff-stat 空）+ 协调分支 coord/kernel-sendroute-impl-1-claim（worktree D:/Vit_DAW_wt_sendroute1_coord）/ 领取提交=本提交；领取核对：卡在 todo+派发已确认、依赖无；doing 三卡（LOUDNESS-B=agent Go 域+fs_largeproject_smoke.ps1、GENSHELL-PCA=Mac processor 三包+Mac 脚本、GEN-SUNO=纯设计勘察）文件域零交集（本卡烟测扩 dev_agent_smoke 体系，不碰 fs_largeproject_smoke.ps1）；seam 亲核通过：CommandDispatcher 服务注册模式（unique_ptr 服务+handlers.emplace，:2303 先例）、TrackService handleAddTrack :509 / handleSetFolderRoutingBus :669 收尾模式、AuxSendPlugin/AuxReturnPlugin 原语（busNum 属性+setGainDb）、VitKernelUtils.ensureSingleRackForTrack、Plugin::setSidechainSourceByName 原生侧链面、Tests juce_add_console_app 模式——与设计件 §1/§9 锚点一致，停止条件未触发；真栈烟测腿按 PC-RUNTIME-STACK 另行登记（实现阶段不占栈）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
