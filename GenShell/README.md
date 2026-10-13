# GenShell — 生成插件壳体（VST3 直实现）

GENSHELL-CPP-1 骨架交付：`IEditController`+`IAudioProcessor` instrument 静音骨架 / T0 消费 /
T1 五参数 / T3 chunk / VSTGUI 编辑器静态六区 / T2 信封回环 echo 计数。

## 结构

```
GenShell/
  CMakeLists.txt          构建入口（SDK 定位/插件 bundle/单测/部署目标）
  resource/Info.plist.mac bundle 元数据
  source/
    genshell_cids.*       类 CID/参数 id/信封与 chunk schema 常量（契约头）
    genshell_convert.*    UTF-8↔UTF-16（IMessage 属性用）
    genshell_state.*      T3 chunk（vit.genshell.state.v1）状态模型
    genshell_params.*     T1 参数面注册（五状态参数+两 debug 计数）
    genshell_processor.*  IAudioProcessor：instrument 静音 + T0 缓存 + T3 持久化
    genshell_controller.* IEditController：参数面 + T2 IConnectionPoint 端点（echo）
    genshell_editor.*     VSTGUI 静态六区编辑器（件三 §4，视觉面零断言）
    genshell_factory.cpp  手写 IPluginFactory3（裁剪版 SDK 无 public.sdk/source/main）
  tests/genshell_selftest.cpp  壳单测进程（模拟宿主：T2 自环+chunk 全保真往返）
  thirdparty/
    vstgui4/              vendored（Apache-2.0，来源记录见 VENDORING.md）
    json/                 nlohmann/json v3.11.3 单头（MIT）
```

## VST3 SDK 来源（GENSHELL-CPP-1 卡内核实结论）

- **SDK = 仓库 `tracktion_engine` 子模块内 JUCE 内嵌 VST3_SDK**：
  `tracktion_engine/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK`
  （PluginProbe native-host 同源；PC 完整检出同路径成立——两端可复用，无全新 vendoring 需求）
- 该 SDK 为 JUCE 裁剪版：**无** `public.sdk/source/main`（模块工厂）、**无** vstgui4、无
  `vstaudioeffect.h` 便捷基类。本工程对策：工厂手写（`genshell_factory.cpp`）、processor
  直接继承 `Vst::Component`+`IAudioProcessor`、GUI 用 vendored vstgui4。
- **Mac worktree 注意**：worktree 检出内 `tracktion_engine` 子模块为空目录（已知工件）——
  构建时用主仓 checkout 的 SDK（`-DGENSHELL_VST3_SDK_ROOT=<主仓路径>/…/VST3_SDK` 或把
  本工程放在主仓内构建）。

## 构建（mac）

```bash
# 主仓根 = Vit-DAW checkout（含 tracktion_engine 子模块）
cmake -S GenShell -B GenShell/build \
  -DGENSHELL_REPO_ROOT=/path/to/Vit-DAW \
  -DCMAKE_BUILD_TYPE=Release
cmake --build GenShell/build --config Release        # → GenShell/build/GenShell.vst3
cmake --build GenShell/build --target deploy_vst3    # → ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3

# 壳单测（T2 自环 + chunk 往返，exit 0 为过）
GenShell/build/genshell_selftest GenShell/build/GenShell.vst3
```

真栈烟测：`scripts/gen_shell_skeleton_smoke_mac.sh`（scan 发现 → rack_add_node →
五参数 → SceneMode 读写 → chunk 往返 → show_plugin_editor 不崩）。

## 契约出处

通道分层 T0-T3 / T1 参数枚举 / T2 信封 / T3 chunk schema / CID 钉死：
`coord/runs/GEN-PLUGIN-SHELL-DESIGN-1/01-channel-contract.md`（件一）及其件二/件三。
