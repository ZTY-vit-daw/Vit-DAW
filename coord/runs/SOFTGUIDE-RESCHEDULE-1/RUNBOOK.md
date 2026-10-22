# SOFTGUIDE-RESCHEDULE-1 真栈腿点火手册（RUNBOOK）

- 前置：[PREDECLARATION.md](PREDECLARATION.md)（§8 预声明，跑前锁定）；本手册只机械执行，不新增自由度
- 被测：worktree `D:/Vit_DAW_wt_sgr1` 构建 agent（bin/VitAgent.exe SHA 前 16=E7AE99B31F5B01E1）+ vsphub（CBECC9897175BAB6）+ K3 内核只读（kernel_expected_sha256.txt 现算哈希在卷）

## 0. 点火条件（三条全过才可起栈）

1. **栈空闲**：`bringup_sgr1.ps1 -Action Status` 四端口全 free，且 Get-Process 无 VitApp/VitAgent/vsphub；
2. **错峰**：PC-RUNTIME-STACK 无他卡在飞占用（162 已释出、164 GUI 腿已释出或已收口）；
3. **预声明在卷**：本目录 PREDECLARATION.md 已提交（0078e2ec，跑前落盘完成）。

## 1. 栈登记（起栈前写入 PC-RUNTIME-STACK.md 当前占用节，commit 到 main）

```
- 2026-10-22 <HH:MM> +0800 登记 / SOFTGUIDE-RESCHEDULE-1(159) 146 软引导实验重排腿——B2 臂 10 变体+B1 对照 5（§E 原协议）;
  owner=GLM-5.3-flash L1 PC ZCode 执行会话（159 卡会话）；worktree D:/Vit_DAW_wt_sgr1@port/softguide-reschedule-1（生产码零改动）；
  内核=主检出 K3 Release 722A79D7 只读复用（<HH:MM> 哈希亲核）；手动起栈三进程（kernel+vsphub+agent），VIT_PROJECT_XML 指 run 目录副本，不启 UI。
  §8 声明（运行前）：LLM=deepseek-flash ≤90 调用（预声明 §5）；成功=腿级机械计数面如实产出（不设通过线，判读交主管）；
  失败分类=F-inline/F-notopo/F-halluc/F-gate/F-env（§E 逐字）+驱动缺陷轮单列；止损=第 2 次 F-gate 全停上交（157 修复失效证据候选）/F-halluc 全停/驱动修复 ≤2 次。
  登记前只读核对（<HH:MM>）：四端口 tcp=0、VitApp/VitAgent/vsphub=0、用户 Godot 未动。
```

## 2. 起栈

```powershell
cd D:\Vit_DAW_wt_sgr1\coord\runs\SOFTGUIDE-RESCHEDULE-1
mkdir run_<yyyymmdd_hhmmss>            # 新工件目录（AGENTS §9，不覆盖旧 run）
powershell -NoProfile -ExecutionPolicy Bypass -File .\bringup_sgr1.ps1 -Action Start -RunDir <run目录绝对路径>
# 脚本自带：端口/进程预检（占用即零副作用中止）、内核哈希硬校验（对 kernel_expected_sha256.txt）、
# kernel_workspace/default_project.xml 隔离、pid 三件落 run 目录 logs/
```

## 3. 驱动批次（B2 臂主线；每批一条命令，批间执行 §5 计数与止损）

```powershell
python D:\Vit_DAW_wt_sgr1\scripts\softguide_exp.py --agent-http http://127.0.0.1:7878 --arm B2 `
  --variants A1,A2 --out-dir <run>\driver\arm_b1 --fixture-dir <run>\driver\fixture `
  --prompt-render-path <run>\logs\agent_prompt_render.jsonl
```

批次序（B2）：[A1,A2] → [A3,A4] → [A5,B1] → [B2,B3] → [B4,B5]；B1 对照臂：[A1,B1] → [B2,B3] → [B4]（B2 臂全跑完后执行）。
fixture 首批自建后复用（驱动自带 reuse）；每变体 project.open 复位副本。驱动 exit 0=捕获（分类失败也算捕获）；exit 2=环境中止。

## 4. 跑后工件固化

- 快照 `<run>\logs\`：agent_last.log 全量、agent_llm_telemetry.jsonl、agent_prompt_render.jsonl、message_loop_debug（如生成）、pid 三件、kernel_sha256.txt；
- 每批 console 输出留存 `<run>\driver\*_console.txt`；
- `git add` 轻件入库本目录（重件：fixture wav、工程副本留盘 run 目录）。

## 5. 机械计数规则（回执口径，判读交主管）

- **有效轮**：变体目录 chat_1.json 在场且驱动正常捕获；驱动缺陷轮（exit 2/异常形状）单列不占产品轮（127/146 先例）。
- **F-gate 计 1**（任一机械信号）：①终局 capability_blocked 且 receipt.boundary∈{proposal_missing, proposal_invalid}；②该会话 agent_last.log 出现 `free_state_final_gate` 弹回行（146 原签名 `closure candidate frontier requires…`）；③回复面自报边界句（146 ctrl-A1/A3 话术形态）。
- **到达计 1**：run 内出现提案面任一形态——needs_experiment 提案 / 交互提案卡 / workflow_data 提案结构 / 终答含拓扑计划文本。
- **幻觉硬判**：提案引用轨/send 目标 ∉（prompt_render frontier 披露轨 ∪ 本提案新建清单）→ 幻觉计 1（对照 send_list_after 读回）。
- **其余分类**：F-inline（装载落源轨且 fx_tracks 增量 0）/ F-notopo（无位置主张）/ F-env（带原始证据）——如实计数，**本腿不判优劣**。

## 6. 止损执行点（预声明 §4，机械执行）

- 批间若 F-gate 累计=1 → 暂停主线，追加 1 枚新变体复核；**累计=2 或同签名重现 → 全停**：立即固化工件，本手册 §7 拆栈，回执标注「157 修复失效证据候选」，上交主管。
- F-halluc 任何一次 → 全停上交。驱动缺陷 → 修复尝试 ≤2 次，之后停。

## 7. 拆栈与释放

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\bringup_sgr1.ps1 -Action Stop -RunDir <run目录绝对路径>
# 只杀 pid 三件记录的自起进程（进程名不符即拒杀）；复核四端口=0 输出在卷
```

- PC-RUNTIME-STACK.md 写释放行（登记/释放时刻、run ID、驱动 exit 码、F-gate/到达计数、拆栈复核、工件路径），commit 到 main。

## 8. 回执落盘

- `<run>\RESULTS.md`：判定面表格（§5 口径逐变体分类）+ 诚实边界 + 工件清单；
- 卡面回执（coord/cards/doing/2026-10-21-SOFTGUIDE-RESCHEDULE-1.md）：工件路径 / F-gate 计数 / 到达率 / 端测边界声明——**只计数，判读交主管**；卡移 done 待验收。
- 分支提交并 push：`git push origin port/softguide-reschedule-1`；轻件随分支入库待决策侧合入。
