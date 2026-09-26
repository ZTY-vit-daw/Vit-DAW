# PORT-CLOSEOUT-LEGS-1：收尾腿二件——SCANPOLL 默认旋钮实证+旋钮补丁回撤；腿 5 补件应用（telemetry_manager.gd）

- 优先级 / 预估 / 依赖：P2 / 0.2 天 / FIX-KERNEL-HYGIENE-BUNDLE-1 验收移交（rulings/2026-09-26）+ FIX-HARNESS-SCANPOLL-1 验收转交腿（45f3c41）+ PC 投递已到（7563b23）
- 模型分级：L1 / GLM-5.3 flash 可接（验证腿+前端仓应用，零主仓代码改动预期）
- **执行侧（mac 侧卡）**
- 目标：
  1. **SCANPOLL 默认旋钮实证腿**：main 已含 harness 默认 2000ms（cfad7f9）；mac journey 一轮原命令原素材（**不传 scan_poll_interval_ms**，旋钮走默认）——断言扫描 warm-up 段无看门狗超时、旅程全绿（PC 卡目标③的 mac 腿）；**通过后回撤 ebd0428 驱动旋钮补丁**（journey 脚本显式 2000 传参行——harness 默认已 2000，传参冗余；回撤后 journey 再跑一遍冒烟级确认扫描段正常（可用 --skip-reopen 减时，断言面照旧）
  2. **腿 5 补件应用**：`~/Desktop/pcbatch_leg5/`（telemetry_manager.gd sha `eb93e210…`+probe `ee620e8b…`，决策侧已核=PC 已提交版本零 WIP 夹带）应用到 `~/Documents/vit-daw-frontend`（目标路径按 Godot 仓结构定位，probe 放 PC 工件同款路径或仓内 probes 惯例位）；前端仓 port/* 分支提交（含两件 sha 注记）；`godot --headless --check-only` 或加载确认零错
  3. 回执：两腿证据（journey 报告×2+前端仓 commit）+两端 HEAD（决策侧发卡=本提交后 main）
- 约束：零主仓代码改动（旋钮回撤是 scripts/ 仓库改动走 port/* 分支+决策侧合入）；token/key 零入工件；§8 照旧
- 验收：①默认旋钮 journey 全绿证据+回撤后冒烟绿 ②前端仓 commit+加载确认+sha 注记 ③回执两端 HEAD
- 停止条件：默认旋钮下扫描段看门狗复发 → 取证上交（PC 修复未吸收 mac 形态）；前端目标路径与 PC 仓结构冲突 → 记录上交
- 领取：
- 回执：
- 验收：
