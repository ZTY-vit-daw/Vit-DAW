# FS-ADOPT-CLOSURE-1：采纳路径不结算会话 closure——park 默认采纳后新 goal 绑旧 closure 吃 StopTaskSettled 罐头回复零执行（M1 第四轮实测，P1）

- 池序 25（P1 用户面）；目标仓库=D:\Vit_DAW（PC 执行侧，agent Go 域）；来源=M1 第四轮活栈取证（[runs/M1-RETEST-20261001/FORENSIC-NOTE.md](../../runs/M1-RETEST-20261001/FORENSIC-NOTE.md) 追记段+evidence/events-webui_mupimj6f.json，勿重取证）
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（FS-PARK/FS-STOP 两修复已合入——本卡是同族第三路径：**采纳路径**的会话级持久态收尾缺口，前两卡分别修了新输入与停止路径）
- 模型分级：L2 / GLM 首选（closure 状态机收尾语义；两姊妹卡先例同域）
- 已核实事实（2026-10-01 20:33-20:36 活栈，会话 webui_mupimj6f）：
  1. park→新输入→**采纳语义正确触发**（20:35:33 round.decision+settled=adopted 链完整，FS-PARK 主语义真机闭环）；随后新 goal_65e6beee（"可以听听看drums音轨的clip低频怎么样吗"）正常开启——**但零观察零执行即收尾**，回复=「任务已经满足其持久化契约中的证据与结算条件。」
  2. 回复源=audioClosureSettlementReply 的 **StopTaskSettled** 映射句（audio_closure_controller.go:1663-1664）。
  3. 机制链：FS-PARK Part A 采纳（judgment_park_continuation.go）结算 experiment+task（EventTaskSettled）+loop completed+CompleteGoal，**但全函数无任何 audioClosures/controllerOwners 调用**——会话 closure 留在活跃非终态、TaskState=Settled；新 goal 经 prepareAudioClosureContext（audio_closure_controller.go:62-66）绑定会话既有活跃 closure → 绑到已结算任务态 → audioClosureSettleFromResult 把 StateSettled 映射 StopTaskSettled（:1327-1333）立即结算收尾。
- 目标：
  1. **修复（首选方向=源头镜像，沿 FS-STOP-APPLY-1 Part B 先例）**：采纳结算路径（settleJudgmentParkOnUserContinuation 或其调用链）对「closure 非终态+任务已结算」的会话：closure 以**诚实采纳停因**结算（禁止伪造成 satisfied/diagnostic_complete——采纳≠问题解决；如需新停因（如 adopted_by_continuation 族）按最小面新增并保持旧值兼容）+ settleAudioClosureOwner 释放所有权。settleStoppedTurnClosure（FS-STOP 产物）可评估泛化复用（同为「任务终态+closure 非终态」的收尾），不强行共用则并行小函数。
  2. **纵深防御（绑定层守卫）**：prepareAudioClosureContext 对「活跃 closure 但其任务已终态」的绑定决策——不得把已结算任务的 closure 交给新 goal 当工作 closure（结算旧+新建新，或至少 WARN+拒绝绑定）；两层修复都做，红测各自钉。
  3. **回归**：a) 红测=今天场景（park→新输入采纳→**下一个**新 goal 必须真实观察执行，不得再出 StopTaskSettled 罐头句）——修复前红；b) FS-PARK 六钉+judgment_park_continuation 全部测试零回退；c) FS-STOP-APPLY-1 四钉零回退（settleStoppedTurnClosure 不被误触）；d) 正常任务完成路径（非采纳的 StopTaskSettled 合法场景，如显式 judgment POST 后再开新 goal）不劣化——如实区分「合法已结算」与「本缺陷的误绑定」。
  4. **真栈烟测**：run_free_state_d1_smoke 体系扩展（ContinuationAdoptionProbe 后追加第二个新 goal 断言其实际执行），exit 0 方算交付（AGENTS §5）。
- 文件域：agent/internal/chat/（judgment_park_continuation.go/audio_closure_controller.go/prepareAudioClosureContext 一带+测试）。
- 约束：真栈 §9——**用户活栈可能仍占 7878/5555（20:32 起）**：代码+单测先行，烟测等端口空闲（用户被通知关栈后）；泊位声明必附。
- 验收标准：双红测红绿+全量 0 FAIL+两姊妹卡钉零回退+烟测 exit 0+锚点清单。
- 停止条件：closure 收尾与既有 FS9/settle 语义冲突超 chat 域，或绑定层守卫需改 prepareAudioClosureContext 的核心契约 → 实证上交定扩域。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 红绿 / 烟测 run ID / 泊位声明）
- 验收：（裁定文件 / 验收 commit）
