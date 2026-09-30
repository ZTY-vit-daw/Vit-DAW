# FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 · PC 回归腿复跑（2026-09-30，闲时任务车道）

- 目的：收口腿①——在 PC 上复证执行侧单测（mac 复跑见 decision-mac-rerun-20260929/）。
- 环境：detached worktree @468ad0e4（`D:\Vit_DAW_run\pluginlist-pc-rerun-20260930\wt`，origin/port/fix-pluginlist-absolutepath）；tracktion_engine 子模块 b43974980c480d4d3f378fe84f640ec9b4ad4300；生成器 **Visual Studio 18 2026**（VS2022 实例不存在，按任务预案回落到本机默认生成器并记录）；多配置生成器下 CMAKE_BUILD_TYPE 提示 unused 属正常（配置由 `--config Debug` 承载）。
- 命令链：
  1. `git worktree add /d/Vit_DAW_run/pluginlist-pc-rerun-20260930/wt origin/port/fix-pluginlist-absolutepath`（HEAD=468ad0e4a531006da9291be51a64e1629eae7e1e）
  2. `git submodule update --init --recursive tracktion_engine`
  3. `cmake -S <wt>/VitApp/Tests -B <run>/build -G "Visual Studio 18 2026" -DCMAKE_BUILD_TYPE=Debug -DVIT_TRACKTION_ENGINE_DIR=<wt>/tracktion_engine`（EXIT=0）
  4. `cmake --build <run>/build --target VitPluginListHygieneTests --config Debug`（EXIT=0）
  5. `ctest --test-dir <run>/build -C Debug -R "^VitPluginListHygieneTests$" --output-on-failure`（**EXIT=0**，`100% tests passed, 0 tests failed out of 1`，0.07s）
- 判定：**PC 回归腿 pass**——ctest 退出码 0；直跑二进制输出 `PluginListHygieneTests: all checks passed`（EXIT=0）。
- 输出形态平台差异（如实记录，不影响判据）：PC 侧直跑输出仅 pass 一行；mac 侧在案的清理摘要行（首项 `C:\Program Files\Ghost\g.vst3`）、types/blacklist removed 计数（2/5、2/4）与 MessageListener 断言/DynamicObject 泄漏打印**未在 PC 输出出现**——清理断言由测试内部校验承载（pass 即含），摘要打印为 mac 侧 Debug 输出形态差异。
- 工件：build-test.log.txt（configure+build+ctest+直跑全输出，本目录）。
- 处置：worktree 已 `git worktree remove --force`（主仓 `git worktree list` 不再含 wt）；构建树 `D:\Vit_DAW_run\pluginlist-pc-rerun-20260930\build` **保留供抽查**（含 VitPluginListHygieneTests.exe）；run 目录下分步原始日志（configure/build/ctest/direct-run .log.txt）同目录保留。
- 约束遵守：零源码改动、零 git commit/push、主工作树仅原有两个运行时文件改动（前后 status 一致，见任务回执）。
- 待收口提示（决策侧）：腿①（本腿）pass 后，该卡仅剩**腿②重建部署腿**（重建含修复的演示二进制 + leg D all_green 后替换演示路径）即可终裁。
