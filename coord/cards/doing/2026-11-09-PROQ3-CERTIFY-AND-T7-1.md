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
- 回执：（回填）
- 验收：（主管回填+用户 GUI 终审）
