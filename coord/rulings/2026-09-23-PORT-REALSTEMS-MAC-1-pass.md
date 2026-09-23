# Ruling：PORT-REALSTEMS-MAC-1 pass（2026-09-23）

- 卡：`coord/cards/done/2026-09-23-PORT-REALSTEMS-MAC-1.md`
- 提交：领取 `9050785` / 回执 `cdfecb7`（均 coord-only，双提交独立复核）
- 裁定：**pass**——mac 首次真实素材端到端旅程全绿入档

## 决策侧核验（Mac 决策会话，独立复算非采信回执）

1. **真实素材确实被内核消费（本验收核心关注点，三重证据链闭合）**：
   - `stems_provisioning.txt` 恰好 6×"copied"、零"wrote"（出现 wrote=合成替身冒充，未出现）；
   - run 目录字面路径文件（Windows 路径串为单文件名的 mac 机器事实）与桌面真实 stems **6/6 sha256 逐字节一致**（决策侧独立复算；p01/bass=`02039e43…b34f` 同时吻合 80085 套基准）；
   - 观察表 timbre_frequency 的 clip 源指向真实 `...spv1_p01\stems\bass.wav`（size=3528044）。
2. **断言与退出码**：`journey1_report.json` verdict=all_green，断言 9/9 全绿（a1-a4+s1-s5，`assertions_summary.txt` 逐行核对）；控制台末行 `JOURNEY1_MAC_VERDICT all_green`；脚本 `all_green) EXIT_CODE=0` 映射实核。exit 0 为双证推断（nohup 未捕获 `$?`，回执如实自曝）——映射确定性成立，采信。
3. **转交记录**：18 文件 sha256 清单与决策侧领取前独立计算的完全一致；912.vit 当前实算 `f6ca36fa…4d8bd` 同值（只读未动）；两端 HEAD 链完整（派卡 ead078d / 领取 9050785 / 回执 cdfecb7）。
4. **纪律核查**：`pull --rebase` 改道 `--ff-only` 的理由复核成立（入站 8 提交实测零 VitApp 文件，纯 fast-forward 无 stash 无回退）；`run_discipline.txt` 跑前落盘（12:32<12:41）；sk- 泄漏扫描全工件 0 匹配；④⑤ 合成夹具未动；工作树仅既有两运行时残留文件（与领取注记一致）。
5. **动机面观察表抽核**：假设文本『250Hz 附近约 1.5dB 衰减』与 events.json seq12 逐字吻合；`static_eq experiment is not configured…`+`capability_blocked` 与 seq17/18 逐字吻合；白名单 v5 五族已配/static_eq 与 broadband 留空实核（`~/.vit/free_state_experiment_plugins.json`）。**增量证据成立**：真实素材轮假设锚定实测带结构（bass 56.5dB→low_mid 42.5dB 台阶）+ 项目级 8 跨轨冲突候选，优于合成轮"55Hz 字面基频推断"。

## 流程小瑕疵（不构成返工，入档备忘）

领取提交 `9050785` 只含 doing/ 副本未含 todo/ 删除，悬到回执提交 `cdfecb7` 才闭合——终态正确。**此后领取提交应一次含完整 mv（todo 删除+doing 新增同批）**，已写入 EQ-1 派卡注记。

## 三项裁定（详见 [decisions/2026-09-23-realstems-eq1-p1-and-registrations.md](../decisions/2026-09-23-realstems-eq1-p1-and-registrations.md)）

1. **PORT-WL-EQ-1 升 P1**：EQ 动机化成立且被 capability_blocked 实证（模型想做的实验做不了=演示剧本含插件场景时硬缺口）；卡面已同步修订（含真实素材旅程回归验收+基线指针 run1_20260923_1232）。
2. **masking 不 ready 登记为独立已知缺口、暂不开卡**：`masking_analysis_not_ready_on_current_project_cut` 属观察层 project cut 面缺口，不由 EQ-1 闭合；待 EQ-1 完成后按演示剧本需要再定。
3. **开源就绪 backlog 登记（两件，不开卡）**：白名单构建器未入仓（`build_whitelist.py` 活在 vit-wl1-artifacts）；用户侧"扫描→认证→白名单"旅程未产品化（认证战役主体选择为开发期人工决定）。待 EQ-1 完成后按发布计划定。

## 遗留移交

- EQ-1（P1）执行：修订后卡在 `coord/cards/todo/2026-09-22-PORT-WL-EQ-1-mac-whitelist-eq-completion.md`，终极验收=真实素材旅程回归 capability_blocked 消失。
- demo-capture SOP 未定位（可选项已跳过）——若 PC 侧存在该 SOP，入 transfer 或仓内 docs 后旅程素材可后续采集。
