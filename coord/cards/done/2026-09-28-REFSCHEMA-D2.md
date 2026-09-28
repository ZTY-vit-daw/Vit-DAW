# REFSCHEMA-D2：COM paired 消费方升级+D2 三处切换迁移（G1 ruling D 类收官腿）

- 优先级 / 预估 / 依赖：P2 / 0.3 天 / REFSCHEMA-D1 验收（rulings/2026-09-28-REFSCHEMA-D1-pass.md）：D2 三处生成点挂起，消费方硬编码等值断言待升级
- 模型分级：L1 / flash 可接（锚点明确的双端小卡）
- 目标：
  1. **消费方升级（红先行）**：`agent/internal/com/evidence.go:150`（ready 回执校验）与 `paired.go:168`（artifact 校验）的 `dad.compressor_dual_tap:` 前缀等值断言 → 改 `agentprotocol.ParseRef` 后校验（kind==dad.compressor_dual_tap 且 legacy 形态 snapshot==PairID、或 vit:// 形态 scope/snapshot 承载 PairID——两种形态同帧兼容，宽限期语义 §11）。测试：legacy 旧格式与 vit:// 新格式双形态均过、错 kind/错 PairID 仍拒。
  2. **内核三处切换**：`VitProductionCoordinator.cpp:526/:953` + `CompressorDualTapEvidence.cpp:238` 由镜像常量 legacy 前缀切 `makeCompressorDualTapRef`（D1 已落地的 builder 家族；D2 builder 语义已被 VitRefSchemaTests T5 黄金样例锁定：`vit://dad.compressor_dual_tap/track:trk_9/t=0..44100@pair_2f3e#-`）。
  3. **门**：ctest 双配置全绿（`VitApp/cmake-build-*` 多配置生成器，-C Release 与 -C Debug）+ `cd agent && go test ./... -count=1` 全量 0 FAIL。
  4. 回执：消费方双形态测试+三处切换样例对照（旧→新各一行）+双端门+HEAD。
- 文件域：`agent/internal/com`（两文件+测试）、VitApp 三生成点、VitApp/Tests（若需样例更新）；越域即停。
- 约束：宽限期内 legacy 解析路径保留；agentprotocol 零改动（ParseRef 现成）；提交显式列文件+推 port/refschema-d2+coord 直推 main+切回 main。
- 停止条件：ParseRef 对该 kind 的 legacy 形态不满足 snapshot==PairID 校验所需信息 → 实证上交
- 领取：2026-09-28 20:56 / a1df2d29681c61d85d370973cbf158ade8db811d（HEAD==origin/main，无漂移） / port/refschema-d2
  - 领取时 `git status --short`：仅 `M VitApp/Workspace/default_project.xml`（领取前已有运行时工程状态改动，非本卡产物，不提交、不丢弃）
  - 实勘行号漂移：evidence.go:150 / paired.go:168 无漂移；内核三处 526→534、953→964、238→243
  - 实勘发现：`makeCompressorDualTapRef` builder 在 D1 未落地（T5 黄金样例经 formatEvidenceRef 直测锁定语义）——本卡在 RefSchema.h/.cpp 按 D1 同族 pattern 补该 builder
- 回执（2026-09-28 22:1x 执行侧自验）：
  - **实现 commit**：`0b595c8a`（分支 `port/refschema-d2`，已推 origin；领取 commit `45081d05`）。显式文件清单 9 个：`agent/internal/com/evidence.go`、`evidence_test.go`、`paired.go`、`paired_test.go` + `VitApp/Source/Service/RefSchema.h`、`RefSchema.cpp`、`VitProductionCoordinator.cpp`、`CompressorDualTapEvidence.cpp` + `VitApp/Tests/RefSchemaTests.cpp`。agentprotocol 零改动（红线遵守）。
  - **消费方双形态测试**（红先行）：`TestValidatePairedEvidenceReceiptAcceptsBothEvidenceRefShapes`（evidence_test.go）与 `TestValidatePairedArtifactAcceptsBothEvidenceRefShapes`（paired_test.go）。红形态实测（实现前运行）：vit:// 接受用例双 FAIL——`evidence_ref_invalid` / `pair_identity_invalid`（旧等值断言拒新格式）；实现后全绿。覆盖：legacy 旧格式过、vit:// 新格式过（`vit://dad.compressor_dual_tap/track:track_1/t=48000..240000@com2_pair_1#-`）、错 kind（vit://dad.l3/...）、错 PairID（双形态各一）、malformed（缺 #- 段，ruling #3）、opaque 未注册字面量均拒。校验实现：`validCompressorDualTapEvidenceRef`（evidence.go）——legacy 态要求 `Legacy.TargetKind==dad.compressor_dual_tap && Legacy.Value==PairID`（注册表 slot=snapshot 语义，即 snapshot==PairID）；parsed 态要求 `Ref.Kind==dad.compressor_dual_tap && Ref.Snapshot==PairID`（PairID 在 snapshot 段、track 在 scope 段，与 T5 黄金样例一致）。停止条件未触发：ParseRef 对该 kind 双形态均给足校验信息。
  - **三处切换样例对照**（行号=漂移后实锚；三处输出同构，例值 track=trk_9/t=0..44100/pair_2f3e）：
    1. `VitProductionCoordinator.cpp:534`（stampCompressorDualTapIdentity）：旧 `juce::String(refschema::kLegacyPrefixCompressorDualTap) + request.pairId` → `dad.compressor_dual_tap:pair_2f3e`；新 `refschema::makeCompressorDualTapRef(request.trackId, request.startSample, request.endSample, request.pairId)` → `vit://dad.compressor_dual_tap/track:trk_9/t=0..44100@pair_2f3e#-`
    2. `VitProductionCoordinator.cpp:964`（start reply）：旧同上（`evidence.pairId`）→ 新 `makeCompressorDualTapRef(evidence.trackId, evidence.startSample, evidence.endSample, evidence.pairId)`，输出同构
    3. `CompressorDualTapEvidence.cpp:243`（artifact evidenceRef）：旧同上 → 新 `makeCompressorDualTapRef(request.trackId, request.startSample, request.endSample, request.pairId)`，输出同构
  - **builder 前提漂移**（已在领取栏记录）：D1 未落 `makeCompressorDualTapRef`（T5 经 formatEvidenceRef 直测锁语义），本卡按 D1 同族 pattern 在 RefSchema.h/.cpp 补齐；T5b 黄金样例（builder 输出==T5 串）+T6 改宽限期语义（legacy literal 与 agentprotocol 注册表逐字节锁）。
  - **双端门**：ctest 双配置（主树 `cmake-build-pcverify1-tests`，VS 18 2026 多配置，重配置后 VitRefSchemaTests 入列）：`-C Release` **10/10**（exit 0）+ `-C Debug` **10/10**（exit 0）；`cd agent && go build ./...` OK、`go test ./... -count=1` **全量 0 FAIL**（exit 0）、`go vet ./internal/com` 清（全仓 vet 余 3 处为领取前既有基线：vsphub/harness/chat，`git diff HEAD` 零 diff 佐证非本卡引入）；gofmt 工作树+blob 级（`git show HEAD:<file> | gofmt -d` 空）双清。
  - **泊位声明**：本卡未启动任何真栈进程（无 VitApp 内核/Godot 前端/agent 运行栈、无 5555/7878 等栈端口监听、无 ctest/go 之外的进程），无需拆栈。申报一项：验收时发现常驻 `Godot_v4.6.1-stable_win64`（PID 30936，无栈端口监听）——**非本卡启动**（本卡全程未拉起任何 GUI/前端），疑为用户自有会话，未处置。端测边界：本卡改动未过真实栈烟测（卡面泊位条款）；注意 **`scripts/compressor_dual_tap_smoke.py:133` 仍是 legacy 硬等值断言**（`evidence_ref == f"dad.compressor_dual_tap:{pair_id}"`）——内核切换后该烟测脚本将 FAIL，属本卡文件域外（§11），上交决策侧裁定（扩域修订或并入真栈烟测腿；同 D1 先例：主 VitApp 全链接构建验证亦归真栈腿）。
  - HEAD at completion: `0b595c8a`（port/refschema-d2）；main `45081d05`。
- 验收：**pass（G1 D 类 5/5 收官）（2026-09-28 决策侧，rulings/2026-09-28-REFSCHEMA-D2-pass.md）**——ctest 双配置决策侧复跑 10/10+10/10（二进制时间戳与提交时间线吻合）；go 门隔离复跑 87 包 0 FAIL+双形态测试 PASS+vet 清+blob gofmt 四文件全清（"双级清"属实）；红先行红形态实测采信；builder 前提漂移实勘与处置正确；泊位声明合规。上交的 compressor_dual_tap_smoke.py:133 活破坏点由决策侧验收代修（双形态助手+六形态自检，随验收 commit 入库）。实现 cherry-pick 0b595c8a→fbe58802。L1-1 段完全闭合。
