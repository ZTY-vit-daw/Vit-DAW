# Ruling：晚窗批量一——M2-TIMEFX-1 / LOUDNESS-A-CONFIG / M3-RECON-1 三卡验收（2026-10-12）

- 裁定：GLM 主管决策流 / 2026-10-12 晚窗（用户"PC 早窗任务和闲时任务都已完成，可以检查"）。
- 独立复核（决策侧亲跑）：port/m2-timefx-1（5de7c450+caf76fa2）与 port/loudness-watch-config-1（91ce26fd）合入临时验收线 accept/pc-20261012（自 origin/main=e8200bcb 起，双 merge 零冲突）；`go build ./...` exit 0；`go test ./... -count=1` **95 包 ok / 0 FAIL**（显式 FAIL 计数=0）；gofmt blob 级净（工作树检出命中为 autocrlf CRLF 检出伪影，四文件 blob 复核全 clean）；M2 工件亲证（run111841_api_quota_block.json 含 0.013072/0.026238 原文、run105611_admission_reverb.json 含 reverb_decay_adjust/decay_pct）；两分支域外文件零（M2 47 文件全在 agent/scripts/coord.runs 域内）。

## ① M2-TIMEFX-1：pass（一项补证挂账）

- 验收 1-4（域表两行+量纲实测定/inspector 四工具+真实插件表面实证/零新协议写回链/自由态可达注册契约+NL 准入实证）与验收 5 冒测扩展：**全证据采信**。PCA 两族 promoted 发证四轮 jobs+attestation 泛化（曲线样本源+三轴上限）按「先开枪后装安全」红线核查通过。
- **验收 5 真栈腿=门槛语义裁定（M1-2 先例）**：已证=话术→准入（run 105611）+写参→回读一致（认证线同 set_plugin_param 通道）+四轮门暴露全部修复；未证=修复后单 run exit 0——run 111841 API 额度耗尽（剩余 0.013072<预扣 0.026238，首个 LLM 调用即败，原始工件在卷）=环境中断成立，排除出有效轮次。**挂账：API 额度恢复后复跑 `scripts/run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 一轮 exit 0 即收**（归 M2-TIMEFX-2 烟测轮顺带或独立小腿，不另立卡）。
- 三轮门暴露修复（registry 计数/族映射/单主轴+发证泛化）认定=功能性门暴露非凑通过；sealed 测试扩清单（registry 9→11 等）核查为「按新现实扩清单」口径，无断言弱化。
- 合入 main=双 merge commit（accept 线 ff）；M2-TIMEFX-2 依赖解锁（probe-diff 观察侧可领）。

## ② LOUDNESS-A-CONFIG：pass

- 卡面九项单测逐项对应亲核（回执清单 1:1），94 包口径+决策侧合并态 95 包 0 FAIL 双证；blob gofmt 净。
- 两处锚点偏差（catalog :1069；render_profile_bind 分发形态）非阻塞采信；schema 名按卡 A 权威 `loudness_watch.v0` 采信。
- **边界申报采信并转卡 B 义务**：loudness.watch.configure/status 现仅在 chat/server.go HTTP invoke 面可达，agent-loop 执行器路径（harness.Invoke）无 case 会落 kernel 失败——**LOUDNESS-B-TRIGGER 立卡/实施时必须补 harness.go 两 case（该文件在本卡域）**，记入 B 卡前置条件。
- 合入 main=同 accept 线。

## ③ M3-RECON-1：pass（自验与裁定同会话弱分离声明）

- 本卡为闲时任务、由主管会话代跑：执行（e8200bcb 报告）与验收同会话，弱分离如实声明（同 Mac b4945b07 先例口径）。验收=用户"闲时任务已完成可以检查"+四问锚点当日亲核自证；报告 §六 遗留裁定六项转 M3 步骤 2 拆卡输入（待用户/主管裁定后立卡）。
- 结论要点：引擎 automation 能力完整在场（submodule b4397498）；VitApp「未暴露」精确成立；建议命令族 automation.get_curve/set_points/clear_region 未定案。

## 附：Mac 线 GEN-CTX-INJECT-1 裁定 PC 复核留档（b4945b07 请求）

- 采信 Mac 决策侧先行裁定 pass；PC 侧独立证据=本次验收线全量亲跑 95 包 0 FAIL **含 genctx 包**（0ba2d75d 已在 main，随全量套件通过）。不重复真键冒测（Mac 决策侧复跑 24/24+真键冒测 exit 0 声明在案）。

## 后续

- M2-TIMEFX-1 额度挂账腿（上）；LOUDNESS-B 补 harness case（前置）；M3 步骤 2 拆卡待裁定六项；worktree 三处拆除+六分支清理（零残留核验后）。
