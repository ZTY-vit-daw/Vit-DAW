# 验收裁定：AF-FAMILY-HEALTH-1（257）——pass 合入；G1-G7 逐项裁定

- 裁定：GLM 主管决策流 / 2026-11-05 23:0x +0800
- 对象：done 提交 b16f0066；报告 6e0049a8+工件 @ port/af-family-health-1（随本裁定合入）
- 结论：**pass 合入**——双面全达：健康矩阵+拆减准备度矩阵（**A=0 / H=1（C4 首拆候选）/ F=B-D1 族 / C2 最重最后拆**）+固定路由面与 harness 可达面双清单+ablation 腿模板；「真回归=0」的判定链可信（D1 教训判定流程逐项走完）。

## 一、G1-G7 裁定（主管逐项）

| # | 裁定 | 去处 |
|---|---|---|
| G1 | flaky·loudnessWatchVerdictStorePath 全局 Temp 无隔离——**定性成立** | **259 CHAT-TEST-ISOLATION-1**（t.TempDir 注入+残留清理） |
| G2 | flaky·projectstore 全局 Activate 态顺序敏感——**定性成立** | 并入 259（同族全局态隔离） |
| G3 | c1 脚本三条 go test 后不查 $LASTEXITCODE——**定性成立**（FAIL 仍 exit 0=假绿风险） | **260 SMOKE-SCRIPT-HYGIENE-1**（与 255 上交的 ps1:147 RunDir 同卡） |
| G4 | harness flaky 复现条件未钉死——**记录观察** | 不单独开卡；再现并入 259 |
| G5 | compressor matrix 腿族无 PCA context 被门结构性拦（2026-08-08 门落位后腿未跟上）——**腿过时定性成立，门拦截正确** | **261 MATRIX-LEG-PCA-CONTEXT-1**（P2：invoke 接 full-project-access context）；修前 C2 真栈健康以 33 单测为准 |
| G6 | mix_tick observe 断言 vs 模型文本建议分支——**裁定：needs_confirmation 仍是现行契约**（依据=M3 治理链整体设计：提案类话术须挂正式确认卡；249 组 2 同型话术 needs_confirmation=True 先例；choreo-pending 腿 pending_intact 断言族）→ 属**行为回归取证**非断言过时 | **262 MIXTICK-CONFIRM-CONTRACT-1**（复现+归因 M3 措辞漂移家族+裁定修补方向：措辞指引 patch vs 行为收窄） |
| G7 | 环境陷阱两前置（正斜杠路径+显式 -KernelExe）——记录 | 并入 260 卡面「跑腿前置清单」标准化 |

## 二、体检结论采信

- 「断言过时=0 定性 + 真回归=0」采信——B/C 族链路与工具面健康基线成立；
- 拆减矩阵成为 A-F 主线进度权威底账（A 态计数基线归零）；C4=首拆候选（harness 形态已测+词表边界清晰+判据现成）→ **263 C4-ABLATION-1**（明日首排候选）；
- B4 复用面最多/C2 最后拆的排序建议采信入 backlog。

## 三、纪律核可

- 批 3 不跑决策记录在案（行为面断言风险代表性暴露+预算节制）——裁量正确；
- 零栈 5 失败的「隔离缺陷非回归」判定链完整（隔离复跑→判词表→定性）；计费 ≈¥0.2-0.4；栈批间错峰兑现。
