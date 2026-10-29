# M3-C4-AUTO-INTENT-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-29-M3-C4-AUTO-INTENT-1.md（池序 218；回执后移 done）
- 执行：ZCode GLM-5.3 PC 执行会话（218 卡会话）；worktree D:/Vit_DAW_wt_m3c4ai1 @ 分支 port/m3-c4-auto-intent-1
- 领取基线：origin/main=bf5fa216（领取提交 36f726bf）；实现 e703a544 + 真栈迭代修复 ecb31261（分支已推远端）
- 真栈登记：a7572986（首推 eb105b2f 误插历史段后归位修正）

## 一、冒测执行形态与两轮判定

- 命令（两轮同形，worktree 根执行）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_m3c4ai1/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationIntentChainSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir <run 目录>
```

- **R1（automation_intent_chain_20261029_r1）：exit 0 PASS**——腿本体约 22:4x（含 agent 构建+栈启动约 5 分钟）
- **R2（automation_intent_chain_20261029_r2）：exit 0 PASS**——独立轮（R1 拆栈后重启内核+agent）
- 被测栈：kernel=主检出 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（sha256 c1e32dfe… 217 同源只读复用；本卡 C++ 零改动）；agent=worktree 自建（含本卡全部 agent 面改动）；无 UI；vsphub 不需要。
- **零 LLM 参与声明（AGENTS §8）**：全链确定性（词表路由在语义入口 LLM 之前接管；确认/取消/读回全部机械词表判定），烟测无 LLM 轮次、无概率性分支——R1/R2 两轮全绿即确定性复现（非概率成功样本）。

## 二、六腿断言明细（两轮同形全绿）

| 腿 | 断言点 | 结果 |
|---|---|---|
| setup | 隔离工程（new_project+save_as_project+双轨，内核 ZMQ）+轨级 Volume&Pan 插件 id 解析（get_project_state plugins 行）+曲线基线 0 点 | R1/R2 ✓ |
| LEG A 正面链（agent /agent/chat 面） | 时变话术「0 到 30 秒，让这条轨的存在感慢慢强起来」→确认卡（workflow=section_automation_choreo+waiting_confirmation+六可编辑 fields 逐名断言 shape/start_value/end_value/curve/region_start/region_end+四形状 options+披露目标轨级 volume 参数+区间 0-30）→**确认前零写入（内核曲线 0 点）**→「确认写入」→settle（mutation_performed+written=2+读回容差披露）→**内核 get_curve 逐点核对（t=0/v≈0.30、t=30/v≈0.60=缺省渐入库）**→C4 双证据（mixboard_decision_status=recorded+record_id 在卷（如 mixdec_choreo_session_automation_choreo_cd38…）+workflow_queue_c4=C4/in_progress） | R1/R2 ✓ |
| LEG B 读回入口（agent chat 面） | 「读回刚才那条线」→automation_choreo_readback_ok+point_count=2+matches_written=true（逐点对照） | R1/R2 ✓ |
| LEG C 静态分界负面例 | 「把这条轨的存在感调强一点」（无时变词）→**不武装编舞**（workflow≠section_automation_choreo+无 mutation 报告+内核曲线仍 2 点零触碰） | R1/R2 ✓ |
| LEG D 无区间澄清负面例 | 「让这条轨的存在感慢慢强起来」（无区间无标记）→澄清反问（stop_reason=automation_choreo_clarify_region+区间问句+零写入+曲线不动）——不猜段落 | R1/R2 ✓ |
| LEG E 类别边界例 | 「让这条轨的空间感慢慢铺进来」（空间类+轨上无空间类效果插件）→如实报边界（stop_reason=automation_category_boundary_no_plugin+boundary=no_plugin_on_track+建议电平类回落+零写入）——挂了才有，没挂不虚写 | R1/R2 ✓ |
| LEG F C4 查证（/agent/invoke mix_report 面） | mix_report 回 mix_workflow_queue 节点视图→**C4= in_progress**（annotation fold 面） | R1/R2 ✓ |

## 三、迭代留痕（五轮，最终轮起被测链全绿）

1. R1 首跑死于 ps1 腿路径 BEL 控制字符（工具 JSON 转义层把 `\\a` 吞成 \x07——纯脚本面，被测面零执行；字节级替换修复）。
2. R1 次跑 LEG A 后半红：确认问句自身用「确认写入」而接受表不认它（CONFIRM-CHAT-1 同款接口自洽缺陷）→automationChoreoExplicitApply 机械接受表。
3. R1 三跑 LEG E 红：轨级 Volume&Pan/Level Meter 内建件被当「挂了插件」→空间/音色类改类别相关词表过滤（reverb/delay/space…；eq/filter/tone…）。
4. R1 四跑 LEG F 红：C4=not_started。两层根因：a) M1-2 骨架 queue view 有 annotation 写入面无显示 fold（缺口）→补 fold（无账本证据节点回落 annotation 态，completed 仍账本专属）；b) 每轮 chat 的 workspace activate 从盘读空态**覆盖内存 annotation map**（无持久化载体工程态下本轮写入被下一请求清空）→restore 空 overlay 不清内存。顺带尝试过 project.open（agent 面绑定工程身份）——重载后内核回紧凑快照（轨行无 plugins）+轨列表混入结构轨，回退不用（annotation fold 后无必要）。
5. 最终 R1/R2 两轮 exit 0（间隔拆栈独立栈轮）。

## 四、隔离与事故核对

- VIT_PROJECT_XML 隔离由 ps1 共享段（-AutomationIntentChainSmoke 已入条件列表——217 事故同族第三案的预防性修补，本腿为新开关首用）：run 目录留 gui_project_isolation.json+隔离副本 default_project.xml（内核写入被副本承接）。主检出 VitApp/Workspace/default_project.xml 全程零触碰（217 事故后保持的 21:06 态未再变化）。
- 残留 Godot 20876（登记时在场，非本腿启动，213 会话离线探针形态）不占栈端口，§2.2 留档不动。

## 五、回归与单测（修复后复跑）

- go build ./... exit 0；go test chat 全家（108s）+experiment+executionports+agentprotocol+rules 全绿零回退。
- 新增测试：chat/automation_intent_test.go（武装分界/类别映射/区间三通道/形状生成/修订合并/fields 编辑拒绝矩阵）+experiment/section_automation_test.go（域表行准入/结构拒绝矩阵/多轮 fail-closed/读回断言骨架含容差）；dose_bounds 一致性测试随扩 section_automation 分支+agentprotocol 词表协议测试随新常量过。

## 六、端测边界声明

- 覆盖=agent /agent/chat 真栈意图链两轮（确认前零写入/治理写入/内核逐点核对/C4 双面证据/三负面例）+agent /agent/invoke mix_report 面+内核 ZMQ setup/曲线核对面。
- 不覆盖=webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界——本卡 webui 零新建，确认卡走 interaction_requests+fields 既有通用面，渲染行为由既有测试族覆盖）；interaction respond 面的确认卡点击路径（本腿走 chat 消息确认；respond 面分支已实现但未入烟测断言——webui 旅程边界内）；段落标记通道的 marker 命中路径（本腿区间走绝对区间；marker 通道逻辑在单测覆盖，真栈 marker 场景未起）；LLM 话术→工具选择面（217 指引行模型行为——本卡链路在 LLM 前接管故不受影响）；手测面（用户自然话术一条收口+读回，待主管安排）。
