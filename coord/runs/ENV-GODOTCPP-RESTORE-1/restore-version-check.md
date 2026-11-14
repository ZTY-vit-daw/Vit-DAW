# 恢复源版本核对（restore-version-check）—— 2026-11-14 freeflash

## 结论：备份版本相符（与"已被今晚真栈证明可用"的配置一致），不触发停止条件；恢复 D:/Vit_DAW/godot-cpp 放行，编译验证腿作最终仲裁。

## 备份实态（D:/备份/vit-daw-frontend/godot-cpp，作为不受信数据先核后动）

- git 仓库：branch **4.2**，HEAD = 4bc6e67d51df4c12e150fe4769911e239c627a47（**godot-4.2.2-stable-74-g4bc6e67**，末次提交 2025-01-28 "Merge pull request #1694 from dsnopek/4.2-cherrypicks-9"）
- extension_api.json header：**Godot Engine v4.2.2.stable.official**（version_major=4 minor=2 patch=2）
- gen/include/godot_cpp/core/version.hpp：GODOT_VERSION_MAJOR 4 / MINOR 2 / PATCH 2（gen 为 4.2 API 预生成）
- SConstruct：**无 api_version 变量定义**；明文含 "WARNING: Unknown SCons variables were passed and will be ignored"（未知变量=警告后忽略）
- git status --short：**0 行**（工作树与 HEAD 零差异、零未跟踪——包括 bin/ 在内全部已提交）；`git fsck --connectivity-only` exit 0（对象库自洽）
- 规模：3832 文件（不含 .git），467MB（du）；整目录含 .git 488MB

## 哈希抽样（SHA-256）

```
a51bac563c496aeea5c514610a4dad3a0101bf99c492a252c3a592d63abdd771  gdextension/extension_api.json
6643d1a733eb8f6297438f2553a500d307485c2aae848cc233bddc6eee36e500  gdextension/gdextension_interface.h
59cdeb14a849455f0423a95d86012708ff68d7c09d6a08b33a208198d162f435  SConstruct
ca54bd8932172914b68e5162262a169c6746fe5004b43426c92a7de21f56efdb  include/godot_cpp/godot.hpp
4e62874faf7de05b711e87d7a9719fa5f94aafa5dcbdc08c707c794a4b8c7395  gen/include/godot_cpp/core/version.hpp
```

## 版本相符性分析（对照卡面"api_version=4.6"前提）

1. **卡面前提被证据修正**：OS-1 PROGRESS S5 "scons api_version=4.6 双变体编译过"中的 api_version=4.6，在 4.2 版 godot-cpp SConstruct 上是**未知变量→警告后忽略的 no-op**（SConstruct 明文）。故该次编译实际按 **4.2 API** 执行且成功；"api_version=4.6"是会话叙述面的标记，不是生效配置。无任何证据要求 A 是 4.6 内容（A 已空，无法直核；其全部可观察行为——编译成功+dll 被 4.6.1 运行时加载+真栈 PASS——均可由 A=4.2 解释）。
2. **项目声明面一致**：前端仓 extension/vit_extension.gdextension 第 3 行 `compatibility_minimum = "4.2"`；extension/SConstruct 无版本 pin（相对路径引用 ../godot-cpp 的 include/gen）。GDExtension 向后兼容：4.2 API 构建 dll 可被 4.6.1 运行时加载——今晚 HOST（经 junction→B 构建）与 OS 的真栈四腿+八腿 PASS 全部实证此链路。
3. **恢复源同源**：备份目录名即"备份/vit-daw-frontend/godot-cpp"（前端主检出的原始备份）；前端主检出 B 的 CreationTime 2026-03-23 与项目起步同期；B/A 今晚被清空前的可观察行为（编译成功）与 4.2 内容自洽。
4. **最坏情形兜底**：即使 A 曾为其他版本，恢复为 4.2 内容后得到的配置 = HOST 卡今晚经 junction→B 实际构建并真栈 PASS 的配置（HOST 借用的"主检出"即 4.2 系）。编译验证腿（scons exit 0）是最终仲裁：若 OS 卡新增 cpp 用到 4.2 绑定没有的 API，编译会失败并如实上报，不做凑过修补。

## 停止条件核对

- "备份版本不符且无其他恢复源" → **不符**（版本相符，分析如上）
- "恢复会覆盖现存有效内容" → **不符**（A 现态为完全空目录，21:49 亲证 0 文件；robocopy /MIR 无覆盖面）
- 放行恢复 D:/Vit_DAW/godot-cpp；前端主检出 B 是否恢复 = 卡面留候裁定，本腿不动 B。
