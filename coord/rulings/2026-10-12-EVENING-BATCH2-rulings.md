# Ruling：晚窗批量二——M2-TIMEFX-2 / G3-REOPEN-COSTCMP-1 两卡验收（2026-10-12）

- 裁定：GLM 主管决策流 / 2026-10-12 晚窗（用户"PC 侧任务卡已完成，可以检查"）。
- 独立复核（决策侧亲跑）：port/m2-timefx-2（25a2b91a+9cd14877+d7ef491a）与 port/g3-reopen-costcmp-1（eba745df+6e66761d+3acfd0ff+712b9ddf）合入验收线 accept/pc-20261012b（自 origin/main=7e1c5b23，双 merge 零冲突）；`go build ./...` exit 0；`go test ./... -count=1` **95 包 ok / 0 FAIL**（合并态含双卡）；blob 级 gofmt 净（工作树命中=autocrlf CRLF 检出伪影，同批量一口径）；M2 工件亲证（reverb_smoke_20261012_195100.json：rmsΔ+0.59/peakΔ−5.741/crestΔ−6.331、determinism correlation 0.959、差分实数在场与回执一致）。

## ① M2-TIMEFX-2：pass（D1 额度腿继续挂账）——**M2 时域线收口**

- 卡面三目标全证：内核 timefx_dual_tap_probe（tfx2rp.v1 身份串+120s 窗+作用域强校验+成对三渲染+scope_revision 变化拒收）；FXM 差分生产者（fail-closed 解码+四钩点 best-effort 自动触发+com 收集器泛化）；契约测试（同源同窗同路由+latency 限信+负例）。
- 真栈双 PASS exit 0（reverb/delay 各 28/28 门）采信；差分 bypass≠processed 实数亲证；直发探针往返 sha256 核对在卷；内核构建 exit 0+RefSchemaTests T5c+RenderWatchdogTests。
- **复渲染方差诚实降级=设计发现采信**：Pro-R/Timeless 内部调制致复渲染真差（correlation 0.946-0.991 带内）≠测量误差——确定性证明降 partial+方差数值披露+同窗单渲染对仍有效+硬门收缩到格式/NaN/零信号/覆盖；agent 侧 partial→投影 partial 可观察不可 preflight。**登记**：调制类插件族的确定性证明上限=partial 属物理现实（非断言弱化）；后续如需逐族确定性画像另立卡。
- D1 reverb 复跑腿：run 20261012_195448 exit 3=额度仍未恢复（余 0.000190<需 0.011956）——环境中断如实记录，继续挂账（并入 QUOTA-RECOVERY-LEGS-1，见下）。
- **M2 线收口**（控制面 TIMEFX-1+差分面 TIMEFX-2 双验收）→ 用户手测体验场次触发条件就位（10-04 约定：M2 第一个用户可感功能落地）。

## ② G3-REOPEN-COSTCMP-1：pass（LLM 面腿并入额度恢复窗）

- 双模式各 2 run exit 0（192737/193106）+三轮启栈取证腿采信；三口径报告亲读。
- **口径③（probe 物理成本）完整有效**：push 886 vs pull 950ms/次（n=6/组，7% 差在调度噪音带内，同机同 fixture 同夜配对）——**G3 重开条件①的物理成本腿收口：无 pull 物理反转空间**。
- 口径①②被额度差异性截断（push 大上下文预扣全败/pull 小额偶过）——诚实申报不构成有效对比，健康链证据维持 G3-ATTRIB-1（17 vs 6）；LLM 面健康链对比并入 QUOTA-RECOVERY-LEGS-1。
- mix_report 断言腿：诚实无记录分支行使（0=0 折叠一致）；有结论正面分支未行使（额度阻断面非断言面）——并入同卡。
- **脚本修复追认=准**（6e66761d）：相对 `-RunArtifactsDir`→VIT_PROJECT_XML 相对透传→内核按自身 CWD 解析成双段路径必死且包装器吞错报 exit 0@1.6s——确定性缺陷钉死（wrapper 退出码捕获+部署目录内核日志双证）；IsPathRooted+GetFullPath 归一修法准。**登记为烟测脚本卫生先例**：栈脚本相对路径入参必须归一（G3-ATTRIB-1 当轮系偶然躲过，SMOKE_SCRIPT 卫生账记一笔）。失败分类「环境中断→确定性脚本缺陷」改记演进如实，采信。
- **G3 重开判定（主管）**：物理成本对称→不构成重开依据；LLM 面（轮级调用数）待额度恢复对比腿补齐后再裁，现行维持 push 缺省不切换。

## ③ QUOTA-RECOVERY-LEGS-1 立卡（池序 102）

三处额度阻塞腿聚合一窗（额度恢复后一次栈占用全部收口）：①D1 reverb 全链单 run exit 0（M2-TIMEFX-1/2 双挂账）；②G3 LLM 面健康链双模式对比；③mix_report 有结论正面分支断言。详见卡面。

## 后续

- worktree 两处+验收线拆除、本地分支清理（port 双分支远端留档）。
- 体验场次待用户排期（M2 收口触发）；10-14/15 中期盘点备料。
