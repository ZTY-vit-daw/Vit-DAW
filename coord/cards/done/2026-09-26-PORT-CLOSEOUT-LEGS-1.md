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
- 领取：2026-09-26 13:29 +0800 / origin/main=b46c1fd（HEAD 同基线；领取前工作树既有：M Settings.xml+default_project.xml（运行时状态）、?? VitApp/Workspace/Artifacts/、?? coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/，均与本卡无关不动）/ 分支：scripts 回撤=port/closeout-knob-revert；前端仓=port/leg5-telemetry-probe（领取时拟定）
- 回执（2026-09-26 执行侧自验完成，工件根 `~/Documents/vit-closeout-legs-artifacts/`，token/key 零入工件——run_meta 仅记 has_api_key 布尔+模型名+base-url host，脚本既有设计）：
  - **腿1a 默认旋钮实证**：journey 原命令原素材（`--stems-dir ~/Desktop/cases/spv1_p01/stems`）、零 scan_poll_interval_ms（各轮 bodies/scan.json args 仅 paths 逐一核实）。扫描段 4/4 轮全绿：status=ok+plugin_count=719 全量+零看门狗超时（run1 262s/run2 259s/腿1c 263s/run3b 254s）；默认节奏精确折算 2000ms（kernel 日志 `^Command executed: plugin_scan_status` 计真实 poll：131/262s、129/259s、131/263s、127/254s≈2.00s 均隔，与 harness.go:8676 defaultPluginScanPollIntervalMS=2000 吻合；旧 250ms 形态应 ~1000+ polls 零相似）。**旅程全绿=run3b exit 0、9/9 all_green**（run_id journey1_mac_20260926-145631，repo_head=9e5f7b6 回撤态，完整形态含 phase B 重开；a3 含 audition.candidate.ready/prepare.started/ready+mix_tick.pending+card_mounted=True）。§8 账目：run1/run2 a3/s5 两轮同型红（模型自挂 EQ 后以无观察证据拒写参数，应答全文在工件）→止步取证→腿1c 同代码复绿判定 LLM 随机分支→run3 环境中断（FetchContent libzmq 下载连接中断 exit 2，原始日志在案，排除出有效轮）→run3b --fetch-src 离线兜底成绿；**§11 F6 登记候选**（不稳定面首例候选：原始输出保全+隔离复绿+与本卡 diff 零关联三项齐，随卡上交裁定）。证据文档：LEG1_SCAN_KNOB_EVIDENCE.md+LEG1_A3S5_FORENSICS.md+LEG1_A3S5_MODEL_REPLIES.md。
  - **腿1b 回撤**：port/closeout-knob-revert 分支 commit **9e5f7b6** 已推 origin（基于 1f65077，diff 仅 scripts/journey1_demo_journey_smoke_mac.sh -8/+1，revert ebd0428）——**待决策侧合入 main**。回撤后扫描面复验：腿1c（--skip-reopen 冒烟，red=0、a4=not_observable 属设计态）+run3b（全形态 exit 0）双绿。
  - **腿2 补件应用**：前端仓 ~/Documents/vit-daw-frontend（领取时 main=a9890ba）分支 **port/leg5-telemetry-probe** commit **95e362f**（本地仓无远端，决策侧直审；与 L3READY 卡同口径）。应用方式=**BELL 纯增量三块锚点插入**（+83 行零删除：键集常量×2+写盘路径 _annotate_mixboard_snapshot_freshness() 调用+标注函数族），三块与投递件逐字节一致（脚本断言校验）；probe 按 PC 工件同款路径 tools/diagnostics/ 落位（sha256=ee620e8b… 与投递件一致）；**非整文件覆盖**——投递件（PC 提交版）缺 mac 仓两块已提交功能（PORT-FE-L3READY-1 的入口对账+_reset_mixboard_prepared_row；B4A 基线已入 fa1d74b 的 _mixboard_feature_rows_same_material，即 PC 仓按裁定剥离的他人 WIP 在 mac 侧是已提交内容），整覆盖即回退，故保 mac 现状叠加增量，成品 telemetry_manager.gd sha256=f0234a15…（源注记 eb93e210… 在 commit message）。验证：probe headless 真跑（godot 4.6.stable.89cea1439，VIT_MIXBOARD_FRESHNESS_PROBE_ROOT 沙箱）**exit 0 全断言 PASS**（六场景快照落盘，工件 leg2_probe_green_20260926/）；autoload 栈加载零 SCRIPT ERROR（standalone --check-only 的 VitDebugFlags 报错为单文件模式不注册工程 autoload 的形态性误报，216 行远离改动区；vsp_realtime_adapter TCP 告警为既有环境面）。
  - **两端 HEAD**：决策侧发卡=origin/main b46c1fd；mac 领取提交=1f65077（本回执 coord 提交=收口推送）；回撤分支=9e5f7b6（origin/port/closeout-knob-revert）；前端仓=port/leg5-telemetry-probe 95e362f（基 a9890ba）。
  - 运行账目（§8/§9）：journey 5 轮（run1 红/run2 红/腿1c 冒烟绿/run3 环境中断/run3b 全绿 exit 0），每轮新工件目录不覆盖；领取前工作树既有运行态两文件+两 untracked 目录全程未触碰；主仓零代码改动（回撤仅 scripts/ 走 port 分支）。
- 验收：
