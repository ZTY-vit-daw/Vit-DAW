# Ruling：PORT-PC-VERIFY-1 pass（2026-09-18）

- 卡：`coord/cards/done/2026-09-17-PORT-PC-VERIFY-1-recompile-and-ps1-events.md`
- 实现：`port/pc-verify-1` @`cd734d12`（单 commit，单文件 `scripts/g_runtime_readonly_smoke.ps1` +5/−1）→ cherry-pick 入 main
- 裁定：**pass**

## 决策侧核验

1. **子项 B diff（直接可审）**：6 行改动在卡面文件域内——events 探针 `?limit=200` → `?conversation_id=$ConversationId&limit=200`，新增 `-ConversationId` 参数默认 `g-readonly-smoke-probe`；与 mac 版 `g_runtime_readonly_smoke_mac.sh` 修法逐点对齐（查询构造/probe 默认值/limit=200 保留），契约依据 `agent/internal/chat/events.go` 无 conversation_id 必 400。红绿对照在 commit message 与回执双记录（未修复对真 agent HTTP 400 exit 1 → 修复后 fixture 与真 agent 各 exit 0，同实例对照，agent 取干净 worktree 构建 + 隔离 store）。
2. **子项 A（回执审计口径）**：PC 机本地证据（configure/build/tests 日志在 `Export/pc-verify-1-20260918/`，Mac 侧不可达），按回执记录审计：CMake 4.3.0-rc2 + MSVC 19.50.35726 全新 configure exit 0（A3 `LANGUAGES C CXX` + CMake≥4.0 守卫锚点闭合）；Release build exit 0 产物 30.6MB（A1 SharedMemorySegment 五文件 MSVC 编译通过，非 4819 警告逐条定位在 A1 diff 块外属预存在）；Tests 双配置 5/5 exit 0 且 Debug 口径重跑保断言活性；agent 干净源 `go build ./...` exit 0。命令+退出码+编译器版本+sha1 齐备，内部自洽，接受。**边界声明**：本子项为回执审计，非决策侧复跑（PC 机证据不可从 Mac 复现，符合本卡"PC 侧执行"设计）。
3. **纪律核查**：孤儿改动 `agent/cmd/vitagent/main.go`（PC 主工作树，领取前已存在 10+/3-）全程未触碰未提交——处置正确；该孤儿改动归属待定，**移交下一轮 PC 派卡前核对**（见下）。

## 上报三项裁定

1. **文档同步**：`scripts/SMOKE_TESTS.md`（ps1 裸 limit 差异段）与 `g_runtime_readonly_smoke_mac.sh` 头注对照表两处过时——**随本验收一并同步**（决策侧全仓权限，两处已改为双端 parity 表述），不另开卡。
2. **合并**：`cd734d12` cherry-pick 入 main，随本 ruling 同批提交。
3. **三件栈口径**：**无需补跑**。本卡探针为纯 agent HTTP GET 面（events 契约在 agent 侧，chat/events.go），单进程真 agent + fixture 双验已覆盖契约变更面；内核/Godot 不参与该端点。端测覆盖边界声明（与 webui 渲染面无关，不触发 E2E-WEBUI-1/JOURNEY-1）采认。

## 遗留移交

- PC 主工作树孤儿改动 `agent/cmd/vitagent/main.go`（10+/3-，归属待定）：决策侧已知悉，待 PC 侧下次值班流核对归属（人工改动 or 遗留卡残留），未核对前 PC 侧不得领取触碰 `agent/cmd/` 域的卡。
