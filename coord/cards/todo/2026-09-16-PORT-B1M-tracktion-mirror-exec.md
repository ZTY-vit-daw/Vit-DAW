# PORT-B1M：tracktion_engine 自包含私有镜像构建与重 pin（Windows 执行卡）

- 优先级 / 预估 / 依赖：P0（阻塞层 A 全线）/ 0.5 天 / **[等待用户:创建空私有仓 `tracktion_engine`，或完成 Windows 侧 gh 授权由决策侧自建]**
- 裁定摘要（决策侧 2026-09-16）：`0981e7ce` 曾为上游 develop 提交、上游改写历史后不可达（本地 origin/develop lineage 含它 + Mac 侧 API 422/318 ref 无一包含，三方证据吻合）；不再依赖公共上游，做自包含镜像
- 目标：
  1. 镜像仓 = `te@0981e7ce` git 树快照 + `modules/juce` 93MB 磁盘内容就地内联（gitlink 替换为实际文件，消除第二层上游依赖）+ README 记录来源与构建口径
  2. 推送到私有镜像仓（预计 150-200MB）
  3. 主仓 `.gitmodules` URL 改指镜像 + gitlink 重 pin 为镜像 commit；推送 main
- 文件域：主仓仅 `.gitmodules` + gitlink 条目；镜像仓为新建仓库
- 验收标准：Mac 侧 `git pull && git submodule sync --recursive && git submodule update --init` 一步成功且 `tracktion_engine/CMakeLists.txt` 存在、`modules/juce/CMakeLists.txt` 存在（内联生效）；Windows 侧构建不受影响（submodule URL 变更不触碰工作树内容）
- 停止条件：镜像推送遇 GitHub 单文件/体量拒绝 → 记录具体拒绝项转 blocked（备选：分批或 LFS）
- 领取：2026-09-16 / 决策侧自持 / main
- 回执：
- 验收：
