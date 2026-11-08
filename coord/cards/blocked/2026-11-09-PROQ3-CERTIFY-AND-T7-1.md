# PROQ3-CERTIFY-AND-T7-1：Pro-Q 3 static_eq 认证补缺+T7 产品面终演（模型裸手控制 Pro-Q 3）

- 发卡：GLM 主管决策流 / 2026-11-09 深夜（依据=320 终验遗留①：用户「看到模型实际调好的 EQ」诉求+Pro-Q 3 为 FabFilter 全家唯一无认证件）
- 派发确认：已确认（用户裁定+主管任务书全文；腿二按主管紧急纠偏 2026-11-09 修正为 BAREFREE 缝口径）
- 验收负责人：GLM 主管决策流+**用户 GUI 终审**
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 依赖=320 装置全套（分支 port/plugin-control-eval-proq3-free-1 可续用）；L2 / PC；真栈（腿一认证扫+腿二各需栈，可合并窗口）
- 目标：
  1. **腿一（产品状态变更，用户已裁定）**：为 Pro-Q 3（VST3-Pro-Q 3-a1aba170-ba5f7ab2）补 v1 static_eq 认证——勘察现成认证作业机制（勿新造）→识别器覆盖检查（EQ 参数拓扑→static_eq 家族 axes）→作业落 `~/.vit/pca_certifications/`→**写入 v1 库**（先备份原库）→亲证：v1 库新增 Pro-Q 3 promoted 条目+白名单披露面（free-state 上下文 static_eq 域）出现 Pro-Q 3；
  2. **腿二（T7×3 终演，主管纠偏后口径）**：**BAREFREE 缝配置**（`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`+`VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1`，320 终局批同款）——S5 剥 typed 工具目录（模型无法用控制脚本）+守卫许可（裸写落地）+腿一补缺后白名单含 Pro-Q 3（消除 r3 型拒调）。**用户实验意图**：验证**模型裸手控制 Pro-Q 3**——自行选择 EQ 调整（频段/频率/Q 值/增益量/滤波器类型全自报）、裸参数写入落地、保留终态工程、用户开 GUI 肉眼对账「它说的=它调的」；控制脚本零参与（S5 剥除亲证=装置门既有面）。装置门增补：语料级断言**零 typed_adapter 执行**（S5 剥除后任何 typed 调用=装置失效）；
  3. worktree 续 `port/plugin-control-eval-proq3-free-1`（fix#6 approve 面 150s 超时+修复#1-#4 在位）；一次栈窗口：布位三副本（含 Pro-Q 3 装载）→T7×3（320 冻结话术判据原样）→每轮自报告+before/after/reopen 三面 readback→三工程**保留**（用户 GUI 终审）→对账表（自报 vs readback diff vs 初始态）+用户交付物清单。
- 文件域：腿一=认证作业（`~/.vit` 产品状态：v1 库+白名单，先备份）；腿二=`coord/runs/PROQ3-CERTIFY-AND-T7-1/` 工件+`%APPDATA%\Vit-DAW\projects\` 三工程布位+装置（worktree 分支续）；**agent 源码零改动**；卡面本文件。
- 并行与资源：真栈两腿共用（`coord/resources/PC-RUNTIME-STACK.md` 登记）；与 doing 五卡（EDU-ASK-ANCHOR/ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-OFFRENDER-FIX/S1B）文件域不相交。
- 验收标准：腿一=v1 库新增 Pro-Q 3 promoted 条目（pcactl list 亲证）+白名单 static_eq 域含 Pro-Q 3+备份在案；腿二=T7 三轮各（自报告全文+三面 readback）+对账表+三工程保留+零 typed_adapter 断言+计费上界如实（≈¥1）。
- 停止条件：认证作业机制勘察发现需要新代码（无现成入口）→取证上交转裁定；作业失败=识别器不认 Pro-Q 3 拓扑→取证上交勿硬造；T7 三轮若全因 LLM 层失败→取证上交（LLM 层取证包已在卷）。
- 领取：2026-11-09 02:02 +0800 / origin/main=a177d4d16ba45426537937b61c2da0b4c0385b29（fetch 亲证，=320 终验收口提交）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 322 proq3 certify+t7，派发已确认）/ 协调分支=coord/322-claim（独立 worktree D:/Vit_DAW_wt_322coord，领取基线 fresh 自 origin/main，status=0 行 diff=空）/ 腿二执行分支（候用）=port/plugin-control-eval-proq3-free-1 @ worktree D:/Vit_DAW_wt_320proq3（HEAD=5b4fdd21，status 净亲证，fix#1-#4+#6 在位）/ 领取提交=本卡 todo→doing 推 main。主检出另有他人未提交改动（CURRENT-STATE.md/Settings/design 文件等，卡域外不动）。栈记录顶部亲读=空闲（320 于 01:45 释放）。
- 回执：（2026-11-09 02:04 +0800 收口）状态=**卡面前提被机制事实推翻→blocked（doing→blocked）——腿一停止条件触发：认证作业无现成入口，需新代码（agent 域收据种类枚举）；腿二前提（白名单含 Pro-Q 3 消除 r3 型拒调）随腿一阻塞推翻，付费轮不启动**。全部取证如下：
  - **腿一·机制勘察**（全程只读，零栈零 LLM 零计费）：
    1. 现成入口存在：autosweep EQ 通道（`scripts/pca_autosweep.py` certify_eq:918，FIX-PCA-EQCHANNEL-1 产物；`--phase certify --families static_eq --eq-subject`）；认证 token 已武装（`~/.vit/pca_certauth.token` 在位亲证）；
    2. **识别器覆盖检查 PASS**：2026-09-24 sweep classification.json Pro-Q_3→static_eq「freq/gain pairs=24」（24 带全键在卷）——拓扑识别认 Pro-Q 3；卡面预期的「识别器不认拓扑」失败模式**不成立**；
    3. **真卡点①（作业入口第一道即拒，未达栈/拓扑步）**：EQ_RECEIPT_KIND（pca_autosweep.py:348-351）仅 waves/plugin alliance 两键；certify_eq:924-931 对 manufacturer='FabFilter' 诚实拒 vendor_kind_missing（脚本注释明言：kind 扩容=agent code，sweep 文件域外）；
    4. **真卡点②（agent v1 导入门）**：processorauthority/receipts.go:21-27 EQ 收据种类仅 `waves.static_eq.phase3_live_smoke.v1`+`plugin_alliance.eq_two_level_live_smoke.v1` 两枚，ReadReceipt switch（receipts.go:155-193）精确匹配 schema_version——任何新种类收据 import 即拒「unsupported receipt schema」；receipts.go 自 2026-09-24 后零提交（git log 亲证）；320 分支同枚举（origin/port/plugin-control-eval-proq3-free-1 亲读）；
    5. **历史铁证**：FIX-PCA-AUTOSWEEP-1/20260924_005622/run_ledger.jsonl Pro-Q 3 三条 cert_done 全诚实拒绝——static_eq×2 vendor_kind_missing（EQCHANNEL 首扫+CERTAUTH 续跑两试两拒）+multiband×1 not_multiband_dynamics（v2 typed 通道诚实拒绝：Pro-Q 3 无可证多段动力学拓扑——**multiband 绕道不通且语义错误**）；EQCHANNEL 验收⑤「3 vendor_kind_missing（FabFilter×2/TDR×1，v1 导入门只收 waves/PA 收据种类）」+CERTAUTH 验收「vendor_kind_missing 扩容=独立最小 commit 卡候选登记不阻塞」——该小卡候选**从未开立**（本卡勘察发现即该注记的复活条件）；
    6. 其他门排查（全拒）：pcactl certify-compressor=broadband 专用（Pro-C 2 即此路进 v1，非 static_eq）；certify-processor=v2 family 枚举无 static_eq（main.go:98 亲读）；pcactl import=同一 ReadReceipt 枚举门；手工直写 v1 库/白名单=硬造（卡面禁止）；
    7. **披露链完整性（腿二为何被腿一卡死）**：认证→v1 store promoted→autosweep derive（白名单 v6 static_eq 段消费 store promoted+EQ 带锚，pca_autosweep.py:1495-1532）→`~/.vit/free_state_experiment_plugins.json`（现 v6：static_eq 48 条全 PA/Waves 无 FabFilter）→agent 消费面 free_state_d1_plan_table.go:24（注释亲证）→free-state 认证清单披露。run8 r3 拒调由该披露面驱动（BAREFREE 缝已全开仍拒——320 终验在卷「认证清单披露→模型选择提案链或诚实拒调」），缺认证时腿二只会复现已知行为（≈¥1 买 run8 已知结论）。
  - **腿二**：未启动（付费轮零消耗）；装置全套候用在 port/plugin-control-eval-proq3-free-1（fix#1-#4+#6+selftest 25/25，320 终验亲证），解锁后一次栈窗口可收官（BAREFREE 纠偏口径已入卡面）。
  - **腿一无变更亲证**：v1 库现状 static_eq 63/broadband 57（FabFilter 仅 Pro-C 2）——零 diff、原件未动（无备份需求）；白名单 static_eq 48 条无 Pro-Q 3；`~/.vit` 本卡零写入。
  - **零栈零计费亲证**：未登记栈占用（栈记录 320 于 01:45 释放后无新条目）；零 LLM 调用零 telemetry。
  - **上交点（候主管裁定）**：最小修法两件套=①`agent/internal/processorauthority/receipts.go` 增 FabFilter（或 vendor-neutral）static_eq 收据种类+case——eqCandidate 候选逻辑厂商无关可直接复用（不查厂商，resolveSubject 按 identifier 对 semantic index 解析）；②`scripts/pca_autosweep.py` EQ_RECEIPT_KIND 增 "fabfilter"→新 kind 映射。合入后现成链路一次跑通：token 武装（在位）→certify→import 晋升→derive→apply-live→白名单披露 Pro-Q 3→腿二 T7×3 收官。
  - **端测边界声明**：无 run 无任务轮数据；零渲染面/用户旅程主张；用户 GUI 终审物未产生。
- 验收：（主管回填+用户 GUI 终审）
