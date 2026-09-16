# PORT-A2：Go 侧 shm_darwin.go（共享内存读段 mac 实现）

- 优先级 / 预估 / 依赖：P1 / 1 天（PORT_AUDIT 层 A）/ 无（不依赖 tracktion_engine，B-1 期间可开工）
- 目标：darwin 平台的共享内存读段实现，对齐 `shm_windows.go` 语义；harness 与 vsphub 两处读点（现由 `_unsupported.go` 显式报错降级）获得真实现
- 文件域：`agent/internal/harness/shm_darwin.go`（新）、`agent/internal/vsphub/audio_feature_shm_darwin.go`（新）；如需调整共享接口定义，涉 `shm_windows.go`/`audio_feature_shm_windows.go` 的改动必须在回执中单独列出并说明理由
- 验收标准：`GOOS=darwin GOARCH=arm64 go build ./...` exit 0；`go build ./...` 与 `GOOS=linux go build ./...` exit 0（不回退）；新增单测覆盖段打开/读取/缺失时 fail-closed 路径（POSIX shm 模拟或构建标签守卫的接口测试）；语义对照表（与 shm_windows 逐行为对照：打开失败/段缺失/尺寸不符/读取偏移）
- 停止条件：发现 Windows 侧语义本身有缺陷或接口需要重构 → blocked 上报（A1 内核接口化属另一张卡，不得越域）
- 领取：2026-09-16 20:05 / origin/main=56ef1e5 / port/a2-shm-darwin
- 回执：
- 验收：
