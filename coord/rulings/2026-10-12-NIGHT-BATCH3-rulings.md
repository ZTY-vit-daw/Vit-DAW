# Ruling：深夜批量三——QUOTA-RECOVERY-LEGS-1 / M3-KERNEL-1 / FE-GUI-RACK-LIST-1 / FE-GUI-START-FEED-1 四卡验收（2026-10-12）

- 裁定：GLM 主管决策流 / 2026-10-12 深夜（用户"任务都已完成，可以检查"）。
- 独立复核（决策侧亲跑）：port/m3-kernel-1-automation（12da018c）与 port/quota-recovery-legs-1（工件精简件）合入验收线 accept/pc-20261012c（双 merge 零冲突）；`go build ./...` exit 0；`go test ./... -count=1` **95 包 ok / 0 FAIL，通过包清单与 main 逐包一致**（首数 94 系输出竞态假象，复跑清单 diff 为空实证）；M3 冒测 summary.json 亲读（status=ok，13 检名目与回执逐一相符，CAS 反例双门在场）。

## ① QUOTA-RECOVERY-LEGS-1：pass（腿① 上交换修复卡 106；腿③ 模型分支如实分记）

- **腿② 收口**：harness_ab 双模式健康链 exit 0、quota_blocked=0——G3 重开条件②证据补齐。**G3 终判（主管）：维持 push 缺省不切换**——新模型健康链轮级调用 push 10 vs pull 6（n=1/模，方向与 G3-ATTRIB-1 17v6 一致但样本薄）；pull 侧另有 durable_checkpoint_persist_failed 一轮（如实分记非额度非模型）；probe 物理成本对称（批量二已收）。G3 账面两条件均已取证、均不足以支撑切换，挂 10-14/15 中期盘点复评。
- **腿① =非额度面功能缺陷，采信上交**：新配置接线验证过（第一跳成功+0 quota 错误）；链路 G1-G7 全绿+域选对+冻结轴单主轴（9ae3aa91 目标形态）；但轮内治理过滤输出空集→no_axis_covered——独立面三绿（pcactl eligible promoted+白名单+v2 认证）=缺陷在运行时过滤路径。**M2-TIMEFX-1/2 两笔挂账不可收**→FIX-TIMEFX-AXISFILTER-1（池序 106）立卡。
- **腿③**：两轮 exit 0 但 settle 面走诚实无记录分支（新模型 0/2）——按模型行为差异分记采信（§8 止损正确）；**M1-5 正面真栈证据仍欠**（挂账维持；确定性生产者路径已勘察=settle_deliver -M1Survival，接 mix_report 需管线补腿，后议）。
- 跨模型不可比声明与模型 id 记录（gpt-6.1-sol @ hejuapi）合规。

## ② M3-KERNEL-1：pass（机制层偏离采信+回执工件句失准修正）

- 卡面三目标+六裁定全兑现：三命令全栈（内核 4 文件+catalog 114 行+harness 摄入 184 行+单测 19 例）、真栈冒测 13 检 exit 0（含 fail-closed 与 CAS 双反例）、95 包 0 FAIL、gofmt 净；context_manifest 四处 Excluded 未动（裁定 5 遵守）。
- **设计偏离采信**：clear_region 弃 removePointsAndCloseGap 改 removePoints——真栈证 closeGap 有边界补偿锚点（AutomationCurve.cpp:630-635 区域覆盖后余补偿点），与「区域清除」语义不符；机制层修正、六裁定语义未动，登记为 RECON §1.3 的在册边界特性。
- **回执失准一处修正**：「工件随 port 分支入库」不实——run 工件仅存盘未提交；决策侧验收时亲手补录精简件（automation_curve_smoke_220826/221700，.log 留盘按 *.log 政策）。执行侧不返工，记入检查单参考（SMOKE 工件入库=回执声明必须与 git 状态核对）。
- M3 批次推进：步骤 2 ✅→步骤 3（包络车道 UI+机架状态面）/步骤 4（goal 编舞单轮+意图层）拆卡待主管后续立卡。

## ③ FE-GUI-RACK-LIST-1：pass（用户目检待排期）

- 前端仓 7c3a7f2@port/fe-rack-list-1（基线=Phase A 活线 tip，main 未触碰）；域=卡内 2 文件（摊平 ItemList→厂商/类型两级 Tree+探针），装载链/资料库本体零 diff；探针 109 钉×2 exit 0+回归三面 PASS+构建门零错。v1+v2（检索框）双交付。**手测清单 6 条在 runs/FE-GUI-RACK-LIST-1/RECEIPT.md 待用户目检**；worktree 保留供复验。

## ④ FE-GUI-START-FEED-1：pass（用户目检待排期）

- 前端仓 76201ea@port/fe-start-feed-1（同基线平行分支，与 RACK-LIST 域不相交）；域=卡内 9 文件（磁带流呈现层+哈希气氛纯函数+WAV 装载纯函数+探针）；探针 140 钉×2 exit 0（三通道切换含 0.28s 滚轮节流/双态试听/双视图同源/进入回归）+回归四面 PASS+f5 启动烟测 PASS（535ms 零慢帧）。
- **试听数据源不可寻址采信**（内核仅回 job_id 无产物路径、无 render 索引）——按卡面降级占位（按钮诚实禁用+播放机构受夹具验证）；**数据缺口上交登记**：v2 需 start_render 成功回执登记产物路径或导出默认目录固化（后置卡候选）。recent 时间字取 mtime 口径采信。**手测清单 6 条在 runs/FE-GUI-START-FEED-1/RECEIPT.md 待用户目检**。

## 后续

- FIX-TIMEFX-AXISFILTER-1（106）立卡（本批随裁定入池）；FE 两卡目检清单待用户；前端仓归一卫生项维持挂账（现三条活线：shrink c56e38f←rack-list 7c3a7f2 / start-feed 76201ea 平行）；m3k1/qrl1 worktree 拆除，FE 双 worktree 保留复验。
