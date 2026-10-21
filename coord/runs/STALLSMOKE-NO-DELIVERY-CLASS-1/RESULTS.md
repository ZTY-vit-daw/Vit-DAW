# STALLSMOKE-NO-DELIVERY-CLASS-1 自验工件索引（153）

实现=07e0e3c9@port/stallsmoke-no-delivery-class-1（+harness 修复 e1210dc2）；领取基线 4ce24e41（代码=171640d5）。
被测物=scripts/d1_stall_repro_smoke.ps1 判定循环（新增 parked_owe_no_delivery 缺陷分类）+ scripts/d1_stall_repro_selftest.ps1（状态注入自验 harness，本卡新增）。

## 腿S：stub 注入自验（确定性，零 LLM 零内核进程）——5/5 PASS，harness exit 0

run=%TEMP%/d1_stall_selftest_20261021_185223（精简件=legS_selftest_final_20261021_185223/）。
方法：stub kernel exe 持 5555 + stub agent exe 按 `-ldflags -X main.form=<scenario>` 构建期烘焙场景，服务 /agent/runtime/status + /agent/events；真实 smoke 脚本全流程跑五用例（-SkipBuild 预置 stub、-KernelDwellSeconds 0 -PollSeconds 1 -NoDeliveryCycles 3）。

| 用例 | verdict | exit | 断言 |
|---|---|---|---|
| no_delivery | parked_owe_no_delivery | 1 | 新分类：waiting_continue+live=0+chain_result=0 稳定 3 周期→exit 1 |
| parked_owe | parked_owe | 0 | healthy 回归：欠账回执 needle→exit 0 不变 |
| parked_owe_silent | parked_owe_silent | 1 | 既有语义不变 |
| finished | finished | 0 | healthy 回归不变 |
| defect_fake_complete | defect_fake_complete | 1 | 既有语义不变（且先于新分支 break，零劫持） |

harness 迭代三轮（§8 执行侧自身缺陷，非产品断言失败，如实记录）：①stub 漏 log 导入（build fail）；②-form 无法经 smoke 固定参数面传入→改 ldflags 构建期烘焙（该轮 no_delivery 1/5，其余被缺省形态吞）；③stub 响应 Content-Type 缺 charset=utf-8→PS5.1 Invoke-RestMethod Latin-1 误解码、needle 不匹配（parked_owe→silent）。修后终轮 5/5。

## 腿R：真栈回归腿（§8：轮次 ≤3，defect 同签名 ×2 含 151 先例即止损）——1 产品轮后止损

- run1（18:54，owner 中止不计产品轮）：driver 自身缺陷——dwell 窗口预开 project.open 未先切 authority→needs_confirmation（project.open=RiskConfirm），复刻 151 run1 workspace=<none> 死链前兆；owner 主动中止拆栈，零产品结论。
- run2=**legR_20261021_185849/（产品轮，verdict=defect_fake_complete exit 1）**：预热修正后（dwell 内 authority→project.open status=ok，preheat_project_open.json requires_confirmation=false）全链走通——首片 goal=waiting_continue stop=limit_reached→poll×2 goal=running（观察中，live=1）→模型第 4 拍 valid final 早收→goal=completed task=needs_experiment→**既有 defect_fake_complete 分类正确触发（先于新分支），新分类零误触发**（chain_results 全程=0 但 goal 从未落 waiting_continue+live=0 形态）。签名与 151 先例（20261020 run2/run3 2/2）完全一致=146/154 上游已知缺口（改进提案段 final 零物化），非本卡缺陷。按预声明止损（151 先例 2/2+本轮再现）。
- **healthy-真栈 exit 0 未取得（如实上交）**：载体上游缺口阻塞自然 parked_owe/finished 形态；healthy 回归判读依据=腿S 确定性五用例（真实脚本对 parked_owe/finished 双形态 exit 0）+实现 diff 零改证据（既有分支顺序/条件逐行未动，新分支仅在 chainResults==0 时可达，见 07e0e3c9 diff）+腿R 证明修改后脚本真栈全链可运行且既有分类零漂移。裁定权交决策侧。
- 栈=worktree 构建 agent（=main 生产代码，本卡零 Go 改动）+K3 内核 722A79D7 只读复用+VIT_PROJECT_XML 指 artifacts run 目录工程副本（git-HEAD 六轨含 bass 轨，主检出/staging workspace 零触碰，run 前后 repo default_project SHA256 一致=947E4D00）。LLM=run2 真 deepseek-flash 单话术链 1 轮（首 chat+续片）。
- 拆栈=脚本 finally 自拆（prereq.txt port_released×3）+owner 复核五端口 tcp=0、四类进程=0。
- 重件留盘：worktree artifacts/d1_stall_legR/run_20261021_185849/（bin 36M、events.json、chat_response.json、agent_last.log、BridgeReplies）+ run1 目录。

## 端测覆盖边界声明

覆盖：真实 smoke 脚本五用例全分类表（stub 状态注入面，确定性）；真栈全链一轮（既有分类面）；新分类 verdict 行（D1_STALL_VERDICT parked_owe_no_delivery）与 exit 1 映射（console+report 双源）。
未覆盖：真栈自然 parked_owe_no_delivery 形态（151 修复后按设计不可达，只能注入复现）；真栈自然 healthy exit 0（上游 146/154 载体早收缺口阻塞，腿R 判读上交）；Godot 渲染面/用户旅程（本卡零涉）。
