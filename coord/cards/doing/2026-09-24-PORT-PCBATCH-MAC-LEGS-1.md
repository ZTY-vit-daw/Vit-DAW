# PORT-PCBATCH-MAC-LEGS-1：PC 深夜批次（四卡验收）mac 转交腿汇总——HYGIENE 真栈细腿 + CERTAUTH 47 例吸收 + CTXSYMLINK 复跑 + GD 前端同步

- 优先级 / 预估 / 依赖：P1（四条已验收卡的 mac 侧收口）/ 0.5 天 / PC 深夜批次已合入 main（a5bee6e：CERTAUTH-TOKEN 90766b7 / HYGIENE 65cbfc1 / CTXSYMLINK 26393c0 / GD-BELL 回执 f3338df）
- 模型分级：L1 / GLM-5.3 flash 可接（命令级+验证腿+记账）
- **执行侧（mac 侧卡）**：四腿按序执行；产出机器本地+coord 回执；零仓库代码改动预期
- **腿 1：mac 二进制重建（两件，前置）**：①内核——HYGIENE 是内核 C++ 改动（65cbfc1：PluginListHygiene.{h,cpp}+协调器取消二分），前端在用 VitApp/build/…/Debug/VitApp 为 09-17 源构建已滞后——按旅程脚本同款 cmake 配方重建并替换（保留旧件备份入工件）；②agent——CERTAUTH 是 agent Go 改动（harness.go token 回落），重建 agent/bin/vitagent
- **腿 2：HYGIENE mac 真栈细腿四件**（卡面既定 `[等待真栈验收]`）：①真 Settings 启动日志断言（惰性清理摘要行，现 mac Settings.xml 已干净可构造陈旧条目样本验证——构造后必须还原，§10 纪律）；②真扫描取消→重扫全程（取消不拉黑+已扫成果保留+重扫补齐——正好是 09-23 手测事故场景的正向复验）；③kill -9 崩溃对照（pedal 残留仍拉黑=防御未弱化）；④mac 真栈全量扫描+装载 spot（用设置页或探针均可，装载走 knownPluginList 解析）
- **腿 3：CERTAUTH-TOKEN 47 例吸收**：武装 token（`VIT_PCA_CERTAUTH_TOKEN` 环境变量或 `~/.vit/pca_certauth.token` 文件**非空**即可武装；token 值不入工件）→ `scripts/pca_autosweep.py --phase certify --families static_eq`（参照 PC 腿口径）→ 47 例处置计数（终态/记因/歧义分类）→ derive 复跑白名单（EQ 7→预期显著增长，四源零手编）→ overlay 回归+journey 一轮验证（披露面扩大照旧五环）
- **腿 4：CTXSYMLINK mac 复跑**：`cd agent && go test ./internal/contextruntime -count=1`——mac 回执落卡（26393c0 的 darwin /var 归一化在 mac 的绿灯确认；顺带全量确认 84 包 0 FAIL 首次达成）
- **腿 5：GD-TELEMETRY-BELL mac 前端同步**：PC 仓 `telemetry_manager.gd` 标注块+probe 同步到 `~/Documents/vit-daw-frontend`（注意 PC 注记"同文件他人 18 行 WIP 分账"——同步以 PC 已提交块为界，勿带入其本地 WIP；前端仓 commit 走其 port/* 惯例）；同步后 Godot 侧加载无错确认
- 约束：零主仓代码改动；白名单/内核二进制/前端仓按各腿口径；LLM key 与 token 值零入工件；§8 纪律照旧（sweep/journey 轮）
- 验收：①二进制双重建+备份哈希 ②HYGIENE 四细腿证据（日志/行为各带指针）③47 例处置计数+白名单新版本+溯源+overlay/journey ④contextruntime mac 绿回执 ⑤前端仓同步 commit+加载确认 ⑥回执两端 HEAD
- 停止条件：腿 2 构造陈旧条目样本影响真实 Settings 无法还原 → 弃腿 2①如实记录（其余照跑）；token 武装后 47 例处置大面积 failed（>1/3）→ 取证上交；前端同步遇 WIP 冲突 → 分账记录上交
- 领取：2026-09-24（mac 执行侧）/ origin/main=e90aea6（与发卡基线一致）/ 分支=main（coord-only，零主仓代码改动卡）；领取时 HEAD=e90aea6，工作树残留：`M VitApp/Workspace/Settings/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`?? .zcodeignore`、`?? VitApp/Workspace/Artifacts/`（均不碰）
- 回执：
- 验收：
