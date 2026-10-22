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

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（旁路清单 / commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
