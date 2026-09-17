# PORT-A2：Go 侧 shm_darwin.go（共享内存读段 mac 实现）

- 优先级 / 预估 / 依赖：P1 / 1 天（PORT_AUDIT 层 A）/ 无（不依赖 tracktion_engine，B-1 期间可开工）
- 目标：darwin 平台的共享内存读段实现，对齐 `shm_windows.go` 语义；harness 与 vsphub 两处读点（现由 `_unsupported.go` 显式报错降级）获得真实现
- 文件域：`agent/internal/harness/shm_darwin.go`（新）、`agent/internal/vsphub/audio_feature_shm_darwin.go`（新）；如需调整共享接口定义，涉 `shm_windows.go`/`audio_feature_shm_windows.go` 的改动必须在回执中单独列出并说明理由
- 验收标准：`GOOS=darwin GOARCH=arm64 go build ./...` exit 0；`go build ./...` 与 `GOOS=linux go build ./...` exit 0（不回退）；新增单测覆盖段打开/读取/缺失时 fail-closed 路径（POSIX shm 模拟或构建标签守卫的接口测试）；语义对照表（与 shm_windows 逐行为对照：打开失败/段缺失/尺寸不符/读取偏移）
- 停止条件：发现 Windows 侧语义本身有缺陷或接口需要重构 → blocked 上报（A1 内核接口化属另一张卡，不得越域）
- 领取：2026-09-17 上午（Mac 值班流，用户批准领 A2）/ origin/main `18ac692ae78489a07e5b3a5696da7cc5ebd68c34` / 分支 `port/a2-shm-darwin`
- 回执：实现 commit `6a54aa2`（分支 `port/a2-shm-darwin`，已推送待验收合并）。验证（本机 darwin/arm64，go1.24.1）：`go build ./...`（宿主）、`GOOS=darwin GOARCH=arm64 go build ./...`、`GOOS=linux go build ./...`、`GOOS=windows go build ./...` 全部 exit 0；`go vet ./internal/harness/ ./internal/vsphub/` exit 0；目标单测 harness 7 例 + vsphub 4 例全 PASS（真实 POSIX shm 段，真实系统调用，非 mock）；`go test ./internal/vsphub -count=1` 整包 ok；`go test ./internal/harness -count=1` 仅 PORT-MAC-HARNESS-FLAKY-1 已记录的 2 例既有失败（与本卡无关，无新增失败）。gofmt：本卡文件全过（`processor_load_gate_fullaccess_test.go` 为既有未格式化文件，未触碰）。
  - 文件域说明：卡面列名外触碰 2 个 fallback 文件 `agent/internal/harness/shm_other.go`、`agent/internal/vsphub/audio_feature_shm_unsupported.go`——仅收窄 build tag `!windows` → `!windows && !darwin`（darwin 让位真实现、linux 保持显式报错降级）+ 注释更新；无共享接口定义改动，`shm_windows.go`/`audio_feature_shm_windows.go` 零触碰。
  - 语义对照表（harness `readSharedFloat32Array` 与 vsphub `readSharedMemoryFloat32` 同构，各对齐本包 windows 兄弟）：
    | 行为 | windows 侧 | darwin 侧（本卡） |
    |---|---|---|
    | 输入校验 | harness：TrimSpace 后空名或 floatCount≤0 → 显式 error；vsphub：无 trim，同文案 `invalid shared memory request` | 逐字相同（各随其包） |
    | 段缺失/打开失败 | handle==0 → 真实 errno（非零时），否则 fallback 文案 | shm_open errno 直接返回；fail-closed，绝不返回零填数据 |
    | 尺寸不符 | MapViewOfFile 精确请求 floatCount×4 字节，超出段即映射失败 | mmap 前置 fstat 检查 `size < floatCount×4` → 显式拒绝。**平台差异**：darwin shm 存储按 16KiB 粒度向上取整且 fstat 报取整后尺寸——请求超出段（>取整尺寸）时 fail-closed 与 Windows 等价；亚 16KiB 的逻辑尺寸不符不可见（零填读出），该粒度守门由调用方 float_count/stride 一致性检查承担（harness.go:5070 / asset_materialization.go:79 → shmReadMiss），两平台一致 |
    | 读取偏移 | 偏移 0 起；harness 版 LittleEndian 逐 4 字节 → Float32frombits，vsphub 版 native copy | 偏移 0 起；两处统一 binary.LittleEndian 显式解码（与宿主字节序/对齐解耦，arm64 上结果与 native 等价） |
    | 名字规约 | 内核发布无前缀名（`Vit_AudioFeature_*`，见 WaveformEnvelopeBaker.cpp:888）直传 OpenFileMappingA(W) | 同名经 posixShmName 补前导 `/` 后 shm_open；单测覆盖无前缀/已带斜杠/空白 padding（harness trim 语义）三形态 |
    | 资源清理 | defer UnmapViewOfFile + CloseHandle | defer Munmap + Close(fd)；fd 以 O_RDONLY 打开，读端不可能改写内核段 |
  - 平台发现（本机实证探针，影响 A1）：① **macOS shm_open 段名上限 31 字符（含前导 `/`，PSHMNAMLEN）**——内核现有命名 `Vit_AudioFeature_waveform_<bakekey>_g<gen>_<tile>` 光前缀即 27 字符，必然超限。**[给 A1 的硬约束：mac 侧内核发布段名 ≤30 正名字符（短前缀或哈希方案）]**；② shm 存储按 16KiB 粒度取整（8B 请求→fstat 16384；16385→32768；1MiB→1MiB），首次 ftruncate 后再次截断返回 EINVAL；单 tile 数据量 = framesPerTile×6×4B 常超 16KiB，但 shm_open 无总量上限（1MiB 实测 OK），粒度取整仅浪费尾部空间，不构成阻断。
  - 端测覆盖边界声明（AGENTS §5）：本卡验证 = 交叉编译面 + 真实 POSIX shm 段单测；未含真实栈端侧烟测——内核尚无 darwin 侧 shm 发布端（A1 未开工），端到端读链路须由 A1 落地后 A5 内核 mac 冒测覆盖。由决策侧裁定是否足以交付。
- 验收：
