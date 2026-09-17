# PORT-A4：tracktion_engine mac 构建链（Xcode CLT 编译 + VitApp 链接）

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天（PORT_AUDIT 层 A）/ **依赖 A3**（基础编译链就位后做链接验证；子模块独立编译试探可先行）
- 模型分级：L2 / GLM-5.3（CMake 非 MSVC 分支已在，预期主要是环境问题）
- 目标：tracktion_engine（私有镜像 submodule @`bae0331`，含内联 modules/juce）在 mac Xcode CLT 下编译出 VitApp 可链接产物
- 文件域：`VitApp/CMakeLists.txt` 与 `VitApp/cmake/**` 中 tracktion 接线相关段；**子模块内不改**（上游 pinned）——确需补丁走独立分支 + 说明上交，不直接推镜像
- 验收标准：① mac 编译 tracktion 相关目标 exit 0（构建目录写独立临时位置）；② VitApp 对 tracktion 的链接解析通过（或如实记录首个不可达点与证据）；③ 构建命令与退出码入回执
- 停止条件：需升级 tracktion/JUCE 版本才能编过 → blocked（对应 R2 预案：升级或补丁另开卡）；WaveShell mac 枚举/加载验证不在本卡（A5 真机）
- 领取：2026-09-17 18:59 CST / origin/main=57ee038（领取时工作树干净，HEAD=origin/main；子模块实际 pin b439749=重 pin 后值，卡面 bae0331 为旧值） / 分支 port/a4-tracktion-mac-build
- 回执：
- 验收：
