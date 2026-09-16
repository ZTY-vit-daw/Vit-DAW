# PORT-B1M：tracktion_engine 自包含私有镜像构建与重 pin【已执行归档】

- 领取：2026-09-16 / 决策侧自持 / main
- 回执：镜像仓 `ZTY-vit-daw/tracktion_engine` @`bae0331`（te@0981e7ce 快照 + modules/juce 93MB 内联 + MIRROR_PROVENANCE.md 溯源；41.8 MiB，单文件最大 28MB）；主仓 `.gitmodules` 改指镜像 + gitlink 重 pin `bae0331`；gh CLI 就绪（凭据令牌），决策侧自建仓，[等待用户] 解除
- 验收：pass（决策侧执行+自验：镜像推送成功、主仓推送成功、handover §1/§2 预期同步更新；Mac 侧拉取验证 = 下一张卡的第一步）
- Mac 侧生效命令：`git pull origin main && git submodule sync --recursive && git submodule update --init`（旧上游残留目录可先 `rm -rf tracktion_engine` 再 update，干净重取）
