# FE-SEND-UI-LIVE-CHAIN-1 (171) 真实 UI 断点定位+修复 — RESULTS

- 执行：GLM-5.3 L2 PC ZCode 171 卡会话 / 2026-10-22 20:1x–22:2x +0800
- 基线：前端 main 2007cb1（worktree project-vit-daw-frontend-senduichain1@port/fe-send-ui-live-chain-1）；内核 K3 722A79D7 只读复用；agent 04D52490（与用户手测会话同一构建，哈希亲核）；栈占用登记 2bc46406@coord/fe-send-ui-live-chain-1-claim

## 一、断点定位结论（卡面四条逐条裁决）

**核心结论：前端消费链功能面完好；用户三症状 = ①VSP hub 停摆致写通知延迟 2m16s（agent 面，上交）+ ②折叠/展开态设计差异被误读 + ③展开重建 freed-object 引擎错误（本卡修复）。**

1. **connect source=1032 亲核（卡④）**：内核原始载荷 `VitHeadlessServer2026-10-22_19-42-42.log` 行 82120：`source_track_id=1032, auto_create_fx_track=Reverb Bus, undo_label=Agent: track.send.connect`——用户会话 source=vocals/1032 断言成立。
2. **后端事实链全部健康**：内核日志行号排序还原——agent connect(82120, 19:44:32) 之后，GUI 桥多轮 `track.send.list`(request_id=track_send_list，如 131404/191053/212863/222567) 每轮回执都完整含 `1032 vocals→1037 Reverb Bus @-6.0dB`（最后一轮 235016 空=用户主动断开后）。**「repo 拓扑未落库=消费链断裂」的预设被推翻**。
3. **真实 UI 与 headless 的覆盖缺口补齐**：新增 env 门控驱动 autoload（sendui1_repro_driver.gd，走 start_page/顶栏生产链新建工程+导入 spv1_p06 stems+聚焦人声轨，注入 agent 同构 foreign 写，零操作采样机架/repo 内部态），窗口态真栈四轮迭代复现。
4. **用户悬空窗口的机制**：`vsp_hub_last.log`——19:43:12→19:45:56 零事件，agent 双发布者 19:46:52 **重新注册**（旧连接断）；agent_last.log 同刻 `VSP realtime publish pending failures Post http://127.0.0.1:8787/vsp: context deadline exceeded`。写（19:44:32）落在停摆窗内 → GUI 的 delta→VitTelemetryManager→刷新链断供 2m16s → 悬空期 = 停摆窗，非持久缺陷。**停止条件命中：断点在 agent/hub 面（实时发布停摆+恢复慢），锚点上交拆卡，不在前端。**

## 二、真实修复（前端域）

`app/rack/Vit_Graph_Rack.gd` `_clear_graph_canvas_for_rebuild`：`n.free()` → `remove_child(n)+n.queue_free()`。按钮 pressed 信号发射链内同步重建会析构发射源节点（engine freed-object 错误；用户会话 4 次，本卡复现 2 次/轮）。摘除立即生效（画布同帧清空、load_nodes 同步契约不变），析构延迟到帧末信号链收尾。首版 deferred 重建方案被 K3 探针 9 断言合法打回（同步契约），v2 保留同步语义。

## 三、验证

- **真栈真 UI 终验**（sendui1_final_20261022_2210，窗口态+驱动零操作采样）：写后 B=合一卡零操作出现+徽标 Reverb Bus（非悬空）；B2 按钮路径展开=对卡+**ParamPortRow_send_level 量控件**+**电缆 SendRoute_out_0_1042→SendRoute_in_1_1043**；B3 折叠回合一；C 断开后卡/电缆/确认集/repo 全消+读回 sends=0；freed-object=0（修前 2 次/轮）。
- **回归面**：K3 headless 探针 289/289 PASS（k3_probe_289_post_fix.log）；popup 111/111 PASS（popup_probe_111_post_fix.log）。

## 四、端测边界声明

- 驱动为程序化生产链入口（start_page._create_new_project/顶栏导入链/按钮 pressed.emit），非鼠标旅程；渲染像素/窗口态视觉归用户手测目检。
- mixer 视图默认 2 位态未实例化，mixer 条 SEND 槽面本轮未采样（其 161 腿有 ext161 探针 13/13 覆盖）；话术（LLM chat）腿未跑——本轮写=agent 同构注入（164 L8 先例），话术腿留主管裁定是否补。
- 用户会话的 hub 停摆复现（含恢复慢）不在本轮：需 agent 面修复卡。

## 五、上交（agent 面锚点）

vsp_hub_last.log 停摆窗（19:43:12→19:45:56）+agent POST deadline+发布者重注册（19:46:52）→ 建议 hub/agent 面「实时发布停摆韧性+GUI 写通知断供兜底」卡，归主管裁定。

## 六、工件

run 目录=本目录（9 轮 attempt 日志+verdict+bringup 脚本）；前端实现=port/fe-send-ui-live-chain-1（修复+驱动 autoload+project.godot 注册）。
