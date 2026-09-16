# 决策记录：私有仓部署与 Mac 移植启动（2026-09-16）

- **U4 裁定：推 GitHub 私有仓**（用户确认）。`ZTY-vit-daw/Vit-DAW`，2026-09-16 首推；推送前历史清洗（filter-repo 剥离 `VitApp/build/` 与 >50MB blob），全部哈希重写；30 分支 + 10 标签全量入库；原仓 `D:\Vit_DAW` 已对齐远程
- **spectrum_data.exr 移出跟踪**（用户裁定：旧测试频谱数据，磁盘保留）
- **B-1 裁定：tracktion_engine 自包含私有镜像**（te@0981e7ce + juce 内联；根因=上游历史改写除名 pin，见卡 PORT-B1M）
- **B-1 执行收口（同日）**：镜像仓 `ZTY-vit-daw/tracktion_engine`（commit `bae0331`，41.8 MiB，juce 内联自 Windows 构建机 ground truth，溯源见镜像仓 MIRROR_PROVENANCE.md）；主仓 `.gitmodules` 改指镜像 + gitlink 重 pin；gh CLI 经凭据令牌就绪（决策侧此后可自建私有仓）
- **B-2 裁定：tmp_sodium 裸 gitlink 清理**（已执行，`a2b2c241`）
- **协作机制升级：coord/ 总线 + 双端轮询**（本目录 PROTOCOL.md；用户退出指令中转）
