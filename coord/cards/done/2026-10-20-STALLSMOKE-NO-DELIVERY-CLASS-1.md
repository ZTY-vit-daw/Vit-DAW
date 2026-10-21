# STALLSMOKE-NO-DELIVERY-CLASS-1：停摆烟测「零 chain_result」形态独立缺陷分类

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=149 取证验收 pass，拆卡 B 采纳）
- 派发确认：已确认（149 卡验收裁定，2026-10-20 晚窗）
- 验收负责人：GLM 主管决策流
- 池序 153；目标仓库=D:\Vit_DAW；P2 / 0.25 天 / 依赖=**151 合入后跑**（A 修复后 healthy 路径才可复现 exit 0 回归）；文件域=scripts（与 Go 面不相交，可与 152 并行）
- 模型分级：L0 / flash 可接
- **规格全文=[SPLIT-CARDS.md 卡B](../../runs/PARKING-DELIVERY-1/SPLIT-CARDS.md)（权威）**——`scripts/d1_stall_repro_smoke.ps1` 判定循环新增形态：goal=waiting_continue && live==0 && chain_result==0 稳定 ≥N 周期 → 新分类 `parked_owe_no_delivery` → exit 1（缺陷类）；既有 parked_owe/parked_owe_silent 语义不变。理由=现状该形态落 timeout(exit 2 环境类)，交付面死亡被环境分类吞掉。

## 验收标准

healthy 停摆仍 exit 0（回归）+ 零投递形态 exit 1 且 verdict 行含新分类名（工件=console+verdict 行）；真栈自验与 151 验收栈复用或注入复现。端测边界回执声明。

## 停止条件

ps1 判定循环结构与规格行号漂移致改动面扩散 → 上交现状锚点。

- 领取：2026-10-21 18:24 +0800 / origin/main=a2d3d596（=171640d5+155/156 领取提交，代码基线仍=171640d5）/ owner=GLM-5.3-Flash PC ZCode 执行会话（152/153 同会话顺序执行，用户口令指定领取）/ 分支=port/stallsmoke-no-delivery-class-1（worktree D:/Vit_DAW_wt_stnd1）+ 协调 worktree D:/Vit_DAW_wt_rfbl_stnd_coord（coord/rfbl-stnd-claim）/ 领取提交=本提交；领取核对同 152（同协调提交）；真栈自验时窗=实现与 stub 注入腿完成后，启栈前按 §2.2 只读核对进程/端口并登记（与 155 真栈腿错峰，以启栈前记录核对为准）
- 回执（**实现+自验腿完成 / 2026-10-21 19:1x，实现 07e0e3c9+harness 修复 e1210dc2+工件 c7285b0f@port/stallsmoke-no-delivery-class-1；栈占用/释放全链=605fbb5a→PC-RUNTIME-STACK 19:1x 释放节**）：
  - **实现**（smoke 脚本 +18/−1，判定循环邻域内）：新增 `parked_owe_no_delivery` 形态——goal=waiting_continue && live==0 && chain_result==0 连续稳定 ≥N 周期（`-NoDeliveryCycles` 参数化，默认 3）→ verdict=parked_owe_no_delivery → exit 1（缺陷类，含头注分类表与 exit 映射）；既有五类分支（finished/parked_owe/parked_owe_silent/defect_fake_complete/chain_*）条件与顺序逐行未动，新分支仅在 chainResults==0 时可达、其他状态出现即计数复位（「稳定 ≥N 周期」口径）。
  - **腿S（stub 注入自验，确定性零 LLM 零内核）5/5 PASS exit 0**：真实 smoke 脚本对五用例全分类表——新形态 no_delivery→parked_owe_no_delivery **exit 1 且 verdict 行含新分类名**（`D1_STALL_VERDICT parked_owe_no_delivery`，console+report 双源在卷）/healthy 回归 parked_owe→exit 0、finished→exit 0/既有 parked_owe_silent→exit 1、defect_fake_complete→exit 1 语义不变（且 defect 分支先于新分支 break，零劫持）。工件=legS_selftest_final_20261021_185223/（per-case console+report+prereq+summary）。harness 三轮迭代如实记录为执行侧自身缺陷（stub 漏 log 导入；-form 传不进 smoke 固定参数面→改 `-ldflags -X` 构建期烘焙；stub 响应缺 charset=utf-8 致 PS5.1 Latin-1 误解码 needle 失配），非产品断言失败。
  - **腿R（真栈 §8：轮次 ≤3、同签名 defect ×2 含 151 先例即止损）——1 产品轮止损**：run1=owner 中止不计产品轮（dwell 预热 project.open 未先切 authority→needs_confirmation，driver 自身缺陷，进程端口拆净）；run2=**legR_20261021_185849 verdict=defect_fake_complete exit 1**——dwell 内 authority→project.open 预开成功（requires_confirmation=false）后全链走通，模型第 4 拍 valid final 早收（goal=completed task=needs_experiment）=151 先例 20261020 2/2 同签名=146/154 上游已知缺口（非本卡缺陷），按预声明止损。真栈面证据：修改后脚本真栈全链可运行、既有分类零漂移（verdict 行+poll 行在卷：goal=running 观察期新分支计数器正确复位、零误触发）。**healthy-真栈 exit 0 未取得（如实上交）**：healthy 回归判读依据=腿S 确定性双形态 exit 0+实现 diff 零改证据+腿R 真栈运行面，裁定权交决策侧。
  - **端测边界声明**：覆盖=真实 smoke 脚本五用例状态注入面（stub 持 5555/7878，非真栈进程，确定性）+真栈全链一轮（K3 722A79D7 只读+worktree 构建 agent=main 生产代码+run 目录工程副本，主检出/staging workspace 零触碰）；未覆盖=真栈自然 parked_owe_no_delivery 形态（151 修复后按设计不可达，只能注入复现）/真栈自然 healthy exit 0（146/154 载体早收缺口阻塞）/Godot 渲染面与用户旅程（本卡零涉）。详见 coord/runs/STALLSMOKE-NO-DELIVERY-CLASS-1/RESULTS.md。
- 验收：2026-10-21 晚 / GLM 主管决策流 / **整卡 pass**：实现链 07e0e3c9+e1210dc2+c7285b0f cherry-pick 入 main（7f805e63/558d3746/74ffb0fe）；亲读判定循环 diff（新分支仅 chainResults==0 可达+计数复位语义+既有五类零改动+exit 映射）；腿S stub 注入 5/5 亲核工件（selftest_summary+case_no_delivery console verdict 行）——卡面明示「注入复现」为合法自验面，healthy 双形态 exit 0 回归+新形态 exit 1 verdict 行均以腿S 为验收依据；腿R 真栈一轮判读采信（修改后脚本真栈全链可运行+既有分类零漂移+新分类零误触发；healthy 真栈自然 exit 0 未取得如实上交——载体早收=146/154 上游缺口非本卡域，归 156 实现线后复验）；fixture 哈希防护在卷；验收 commit=本提交
