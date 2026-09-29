# FIX-PLUGINLIST-WINPATH-BLINDSPOT-1 · mac 决策侧复跑（2026-09-29）

- 目的：验收取证——独立复跑执行侧单测，核对回执绿主张。
- 环境：detached worktree @468ad0e（`/tmp/vit-winpath-verify`），tracktion_engine 子模块重初始化（b43974980c），AppleClang 21 arm64，Unix Makefiles Debug。
- 命令链：`cmake -S Tests -B build/tests-verify -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug -DCMAKE_OSX_ARCHITECTURES=arm64 -DVIT_TRACKTION_ENGINE_DIR=<worktree>/tracktion_engine` → `cmake --build ... --target VitPluginListHygieneTests -j 6` → 直接运行 `VitPluginListHygieneTests_artefacts/Debug/VitPluginListHygieneTests`（ctest 不在非交互 PATH，直跑二进制等效）。
- 结果：`PluginListHygieneTests: all checks passed`，EXIT=0。清理摘要首项 `C:\Program Files\Ghost\g.vst3` 于 mac 被清；types removed 2/5、blacklist removed 2/4（含 `D:\Gone\pc-blacklist.vst3`）与全平台期望一致。MessageListener 断言与 DynamicObject 泄漏打印为该测试在案既有形态（不影响 EXIT）。
- 工件：build-test.log.txt（configure+build+运行输出）。worktree 用后清理。
