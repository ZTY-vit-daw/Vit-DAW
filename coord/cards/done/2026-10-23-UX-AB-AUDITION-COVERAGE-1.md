# UX-AB-AUDITION-COVERAGE-1：AB 试听挂接路径全覆盖（174 增量——场次⑤非确定性反馈）

- 发卡：GLM 主管决策流 / 2026-10-23 凌晨（依据=场次⑤用户手测：同话术三次，AB 卡两次出现一次未出现；未出现那次模型「声称轨道已改变，大概新建了 fx 轨道」后无试听）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 177；目标仓库=D:\Vit_DAW；P1（提示词承诺「链末交付 A/B audition」在有突变效果的轮次存在旁路） / 0.5 天 / 依赖=174 已合入；模型分级：L2 / GLM 执行会话

## 现象与取证边界

- 已实证挂接路径：`executeSemanticDynamicTicket` 族（C2 批/C2 运行时/147 全访问链/确认续行）——两次出现均为 semantic_dynamic_execution_completed 终态+audition 挂载（日志 00:26:33/00:36:52 在卷，含用户实点试听+stopped 重落座机制工作）。
- **未出现那次**：模型回复声称轨道已改变+新建 FX 轨（connect auto_create 形态嫌疑=**不经 executeSemanticDynamicTicket 的效果执行路径**——如 agentloop 直发工具族[track.send.connect/rack_add_node harness.invoke]/plugin_grabber 装载路径），bracket 不触发。该会话日志已滚动转出不可回指——形态归因待复现确证。
- 已知挂接面证据：172 设计稿 §1 调用者族清单+挂接单点声明——**直发工具路径不在该清单**=覆盖缺口候选。

## 范围（两步）

①取证锚定：从 webui 会话历史（conversation graph）找未出现那次的会话（用户时间窗 00:27-00:34，终态/工具序列/stop_reason 亲核——若 history 面不可回指，跑同话术复现轮[预声明 ≥5 轮直到抓到无 audition 形态，LLM 预算声明]）；确认其执行路径是否绕过 executeSemanticDynamicTicket；②补覆盖：效果类突变（send 路由/插件装载/参数写）发生在非 ticket 路径时等价伴生 audition（同 bracket 纪律：before 时机不可逆的路径=取证后在可挂点补，或渲染降级方案上交）；③防回归：路径覆盖断言（各执行路径效果突变→audition 必在）。

## 文件域

以取证结论固定（预期 semantic_dynamic_audition.go 邻域+确认的旁路路径挂点）。越域即停。

## 验收标准

取证结论（旁路路径清单+每条挂点）+单测+真栈腿（复现轮抓到的无 audition 形态在修复后 audition 在场）；全量 0 FAIL；工件入 coord/runs/UX-AB-AUDITION-COVERAGE-1/。端测边界声明。真栈腿登记 PC-RUNTIME-STACK。

## 停止条件

旁路路径的 before render 时机不可逆且无等价挂点 → 设计锚点上交（该路径降级方案裁定）。

- 领取：2026-10-23 / origin/main=08588c8c / owner=GLM-5.3 L2 PC 执行会话（ZCode，177 卡会话）/ 分支=port/ux-ab-audition-coverage-1 / worktree=D:/Vit_DAW_wt_uxab177 / 领取提交=coord/uxab177-claim。取证腿已完成（零 LLM 复现轮，history 面四源可回指）：缺失轮=webui_mvjr15zw（00:21–00:23，卡面 00:27–00:34 窗系回忆偏差=内核空档）；三轮效果突变均走直发 bus 两腿 track.send.connect(auto_create)+rack_add_node（executeSemanticTopologyBusLegs，source=chat）零 bracket，ticket 参数写腿三轮均有 audition——结论+P1–P6 旁路清单见 coord/runs/UX-AB-AUDITION-COVERAGE-1/forensics_1/FORENSICS.md（工件随 port 分支入库）；本轮修 P1（实证行使面），P2–P6 上交裁定。
- 回执：（2026-10-23 执行侧自验完成，移 done 待验收）**①取证结论**：缺失轮=webui_mvjr15zw（00:21–00:23，四源证据链闭合零 LLM 复现轮；00:27–00:34 窗=内核空档，卡面时间窗系回忆偏差）；三轮（mvjr15zw/mvjr5hcm/mvjriarj）效果突变均走直发 bus 两腿（track.send.connect auto_create+rack_add_node，source=chat）**零 bracket**，ticket 参数写腿三轮均有 audition；旁路清单 P1–P6 见 forensics_1/FORENSICS.md——**本轮修 P1**（plugin_grabber bus 两腿，实证行使 3/3 轮），P2（单轨 insert 装载 executeDecisions 面）/P3（sendTopologyConnectWorkflow）/P4（C1/B4/C3/C2 运行时装载）/P5（goalrunner 装载）/P6（agentloop 原始工具面）均有可挂点未行使，逐条上交裁定是否另开卡。**②实现**：executeSemanticTopologyBusLegs 挂 runSemanticDynamicAuditionBracket（before 渲染先于 connect→两腿=mutation→after 渲染→挂 audition:dynamic:topo_<planID> 会话；fail-open 三降级码与 ticket 通道同机械复用；腿失败=无卡+错误逐字上抛；无会话身份计划按历史形态跑无 bracket）；条款 appendSemanticTopologyAuditionClause（挂卡共享入口行；降级行如实命名两腿回执不承诺参数回读）。**③单测**：新增 semantic_topology_audition_test.go 6 族全 PASS（计划构造与 topo_* 命名域隔离/bracket 机制复用时序+双事件/P1 路径覆盖防回归钉——两腿在 bracket 内 before 渲染先于 connect+腿失败无卡契约保持/无会话回退渲染零调用/条款措辞/第三身份通道判定 200 落账）；chat 包全量 ok 126s；go test ./... 全量 96 包 exit 0（gotest_full.log 在卷）；gofmt：两编辑文件仅既有 CRLF checkout 基线在列，去 CR 后 -d=0 字节（同 174 基线形态）。**④真栈腿**：run=coord/runs/UX-AB-AUDITION-COVERAGE-1/stack_uxab177（§2.2 登记 f2394513/释放 3b80be54，占用前四端口+四类进程零核实）；agent=worktree 构建 C72801D9（topology_bus 标记门硬校验）+K3 722A79D7 只读+vsphub B10CC412+VIT_DAW_AUDITION_BLIND=1（场次⑤同面）；场次⑤话术复刻一轮（预算 ≤3 轮用 1 轮，LLM=13 次全 deepseek-flash 0 error）：**直发两腿回执在场（walkhop_1_3：track.send.connect+rack_add_node+get_plugin_parameters）→topo 会话 audition:dynamic:topo_plan_f0521d5da3a421d6 挂载（audition.ready seq24+trajectory.user_judgment.requested seq25 同轮相邻+盲测 blind=true swapped+候选 A/B）**+ticket 会话 dyn_54f4e4c0 共存（预期共存形态）+双 render 资产（before_revision_2.wav/after_revision_4.wav 各 582904B 落盘）+错 turn 409+replaced-slot 409（"audition session identity mismatch"面）；断言六面 PASS exit 0（leg_assertions.json verdict=PASS）。拆栈 Stop exit 0 只杀自录 pid（26736/12696/26260）→四端口复核 free。**实现 commit=5d179142（port/ux-ab-audition-coverage-1@origin）**。**端测边界声明**：本腿=kernel+vsphub+agent 真件 HTTP/事件面断言，无浏览器渲染面/Godot——webui 判定卡以事件面契约字段代证（audition.ready 携 session.turn_id/round_id/blind/双候选 ready=卡渲染充分输入，174 F 面同代证形态）；装载文案条款在本轮复现形态被 post-load 语义 handoff 替换为语义确认文案（既有链式投递形态，174 已裁定 stamp 边界同族——卡面以事件面无条件在场，条款面在无 handoff 装载形态落地=单测钉⑤覆盖）；v1 判定探针身份误取重落座投影行（turn 缺席）命中 required-fields 409=驱动器捕获缺陷，v2 探针以挂载行身份重发为权威面（v1/v2 工件双留如实）；C 面资产溯源以修订名+commit refs 核对（本载荷形态 preview_ref=audio-buffer:// 无 sha 面，弱于首稿声明如实注记）；单测环境装载腿被 PCA 门拒（无真实插件二进制），两腿全链挂卡归真栈腿覆盖（既有边界同源）。
- 验收：（裁定文件 / 验收 commit）
