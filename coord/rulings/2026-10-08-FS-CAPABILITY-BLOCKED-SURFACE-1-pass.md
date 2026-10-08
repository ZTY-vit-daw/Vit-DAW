# Ruling：FS-CAPABILITY-BLOCKED-SURFACE-1 —— pass（2026-10-08 晚窗，决策侧）

- 执行侧：flash（用户转交）；实现 `632776c2` + 回执 `35012e37` @ `port/fs-capability-blocked-surface-1`（worktree D:/Vit_DAW_wt_fs_cap，base 477e04cd）
- 落位：cherry-pick → main `ad21313a`（实现）+ `0c9fc3aa`（回执）；复现轮 run 工件保全至主树 coord/runs/FS-CAPABILITY-BLOCKED-SURFACE-1/（repro1_184329 / repro2_184543，不入库）

## 验收亲核（diff/复跑/红线/裁定申报四层）

1. **diff 亲读**：纯插入 +87/-0 单生产文件（goalrunner_chat.go 路由门前边界分支+4 helper）+ 262 行新测试——既有行零改写，检测不命中时逐字节走原 plain return；三既有路径负回归钉住（测试 3：awaiting_action/方案乙 parked+stale receipt/judgment boundary 不被劫持）。
2. **红线四项**：agentloop 包零 diff（亲验）；improvement_proposal_workflow.go 零 diff（亲验）；G 门零触碰；语义红线=无重试无绕门（分支为终态响应组装，明示不执行不自动重试）。
3. **实现裁定申报采信**：卡面"或"臂（独立 admission receipt 触发）改判为 AND+终态护栏——决策侧亲验代码论据成立（chat/free_state_reasoning_loop.go:1053-1077 gate 拒绝出口先重写 decision.Status=capability_blocked+loop.Status 再落 receipt，双条件必然同现；OR 臂只会新增两合法面劫持风险、零覆盖增益）；负回归测试 3 钉死该裁定。
4. **测试与门**：4 新测试（夹具经生产 recordFreeStateDecision 真实造形=R3 忠实度）我方 worktree+主树拾取后两轮全绿；全量 90 包 0 FAIL（回执申报+我方抽查采信）；gofmt 本卡零新增偏差（基线两处既有偏差不在本卡域）。
5. **复现轮（§8）**：两轮真栈泊位 exit 0 全链走通（admitted 8/8 门，auto_applied_no_card）——卡面成功判据"capability_blocked 形显式表面化【若触发】或全链走通"达成；该形概率面未触发（1/4 采样两轮未中）如实申告采信；run 工件在案可回指。

## 后续挂账（随本裁定）

- capability_blocked 形真栈端到端样本：留该形自然出现或旅程轮采样（回执端测边界声明）；烟测分类器粒度（capability_blocked 归 no_candidate_found 族）=RECON-1 §7 待办 a 预存项，不在本卡域。
- **J4 复活条件①落地——J4-REV 随本裁定立卡**（承接 J3 链+确定性判定面）。
- 可选 nudge 腿未做（卡面允许留后续）。
