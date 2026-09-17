# PORT-A4：tracktion_engine mac 构建链（Xcode CLT 编译 + VitApp 链接）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天（PORT_AUDIT 层 A）/ **依赖 A3**（基础编译链就位后做链接验证；子模块独立编译试探可先行）
- 模型分级：L2 / GLM-5.3（CMake 非 MSVC 分支已在，预期主要是环境问题）
- 目标：tracktion_engine（私有镜像 submodule @`bae0331`，含内联 modules/juce）在 mac Xcode CLT 下编译出 VitApp 可链接产物
- 文件域：`VitApp/CMakeLists.txt` 与 `VitApp/cmake/**` 中 tracktion 接线相关段；**子模块内不改**（上游 pinned）——确需补丁走独立分支 + 说明上交，不直接推镜像
- 验收标准：① mac 编译 tracktion 相关目标 exit 0（构建目录写独立临时位置）；② VitApp 对 tracktion 的链接解析通过（或如实记录首个不可达点与证据）；③ 构建命令与退出码入回执
- 停止条件：需升级 tracktion/JUCE 版本才能编过 → blocked（对应 R2 预案：升级或补丁另开卡）；WaveShell mac 枚举/加载验证不在本卡（A5 真机）
- 领取：2026-09-17 18:59 CST / origin/main=57ee038（领取时工作树干净，HEAD=origin/main；子模块实际 pin b439749=重 pin 后值，卡面 bae0331 为旧值） / 分支 port/a4-tracktion-mac-build
- 回执：**零源码改动，纯验证卡达成**——tracktion mac 构建链在纯 pin 下直接可用，无需接线修补。验证环境：本机 darwin/arm64，Apple clang 21.0.0 / CMake 4.4.3 / Xcode CLT，8 核；基于 main@`2baabd2`（HEAD=origin/main）+ submodule tracktion_engine@`b439749`（重 pin 后纯 pin，无补丁叠加）——**A3 回执遗留的"纯 pin 可复现"验证点就此闭合**（A3 验收构建基于补丁分支工作树，当时主仓 gitlink 尚为 bae0331）。
  - **验收①** tracktion 相关目标编译 exit 0：`cmake /Users/timozty/Documents/Vit-DAW/VitApp -G "Unix Makefiles" -DCMAKE_BUILD_TYPE=Debug -B /tmp/vit-a4-build/app`（exit 0，35.1s，含 ZeroMQ v4.3.5/v4.10.0 FetchContent 下载）+ `make -j8 VitApp`（exit 0，约 5 分钟，201 个 CXX 编译单元）。构建目录 `/tmp/vit-a4-build/` 独立临时（AGENTS §10；A3 目录未触碰）。tracktion 编译单元 13/13 产出（JUCE 模块模式，挂在 VitApp 目标下）：tracktion_core.o（900K）+ tracktion_engine 11 分组 .o（airwindows×3/model×2/audio_files/playback/plugins/timestretch/utils，3M–28M/个）+ tracktion_graph.o（5.5M）。
  - **验收②** VitApp 对 tracktion 链接解析**通过**：产物 `VitApp_artefacts/Debug/VitApp` 101MB Mach-O 64-bit arm64 可执行档；`nm -u` 中 tracktion 命名空间 undefined 符号 **0**；tracktion 已定义 mangled 符号 **63230**（tracktion::engine 59460 / tracktion::graph 850 / tracktion::core 219+，含 operator 族）；`tracktion::engine::Engine` 构造 3 重载与 `tracktion::engine::Edit`（248 符号）构造均为 T 级全局已定义；产物 undefined 总 1039 全为系统框架 C API（CoreAudio/AudioFile 等，`otool -L` 全系统框架；ZeroMQ/sodium 静态链入）。
  - **验收③** 命令与退出码如上（configure 0 / build 0）；日志 `/tmp/vit-a4-build/build-vitapp.log`（0 error；1042 条 warning 为 A3 已记录既有风格警告，非阻断未触碰）。
  - 环境事实注记：子模块 `modules/juce` 为 vendored 内联（镜像 bae0331 提交说明），无嵌套子模块拉取面；卡面"submodule @bae0331"为重 pin 前旧值，实际 pin b439749（3009ca2）。
  - 端测覆盖边界声明（AGENTS §5）：本卡验证 = 编译面 + 链接面；未含 VitApp 进程运行冒测与真机音频行为（归 A5）；Tests 编译+运行 A3 已覆盖不重跑；WaveShell mac 枚举/加载不在本卡（A5 真机）。零源码改动 ⇒ Windows 零回归自明。
- 验收：**pass**（裁定 [2026-09-17-A4-pass.md](../../rulings/2026-09-17-A4-pass.md)，决策侧工件抽查：nm undefined tracktion=0 独立证实、日志零真实 error）；零改动纯验证卡无实现合入；A3 纯 pin 可复现验证点闭合
