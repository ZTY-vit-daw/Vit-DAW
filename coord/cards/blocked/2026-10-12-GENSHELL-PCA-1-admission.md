# GENSHELL-PCA-1：PCA 准入面（实施⑤）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡5）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 99；**规格全文=[件五 卡5](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P2 / 0.5-1 天 / GENSHELL-CPP-1（二进制可指纹即领）（✓ 2026-10-16 pass 合入 7745719a——GenShell.vst3 在位，依赖满足，可领；与 HOST-1 文件域零交集可并行）
- 模型分级：L1 / GLM 或 flash 执行会话

## 要点摘录（以规格为准）

generation_shell processor family 三处词表（processorintent/processorattestation v2/processorregistry，coverage v1=空集）+本地认证路径（FingerprintPath 对 GenShell.vst3）+装载门双向探针（promoted 放行/缺席 pca_load_gate 拒/UI 路径不受影响）。验收=真栈烟测+三处词表一致性单测+全量 0 FAIL。停止条件=CertifyProcessor 硬性要求非空工具对（local_processor_certification.go:234-247）→上交换方案不擅自造 typed apply。

- 领取：2026-10-16 21:34 CST / origin/main=ba820dcf / owner=GLM 执行会话（ZCode Mac 执行流，GENSHELL-PCA-1 专会话）/ 分支 port/genshell-pca-1 / worktree=/Users/timozty/Documents/Vit-DAW-pca1 / 领取提交=**6ed8824d**（fast-forward 推 main 亲核，远端卡面 owner 已核对）
- 领取时核对：卡在 todo（远端 ba820dcf 亲核）、依赖行已勾 ✓（CPP-1 pass=7745719a）；GenShell.vst3 在位（~/Library/Audio/Plug-Ins/VST3/）；唯一 doing 卡 LOUDNESS-B-TRIGGER（loudnesswatch/chat/harness 腿）与本卡三包+scripts/gen_shell_pca_smoke_mac.sh 零交集；领取基线=ba820dcf（含 CPP-1 合入）

## 回执（2026-10-16 执行侧）——停止条件触发，部分交付后移 blocked

- **impl commit=4d779236**（port/genshell-pca-1，已推 origin；基线 ba820dcf，6 文件 +273/-4，全部落卡内文件域）：
  - `processorintent/intent.go`：FamilyGenerationShell 家族枚举+supportedFamilies（+intent_test 载入词表/inspect_only 边界拒/未知家族拒）；
  - `processorattestation/v2.go`：v2 家族表空集条目（**coverage v1=空集**——IsV2Family 准入路由、V2CoverageAxes 空、ValidateV2Coverage 拒一切 typed 轴、V2CoverageForRoles 零映射）+validateIssueSpecV2 空集家族**零 coverage 签发**语义（typed 家族仍硬要求非空 coverage，不弱化）；新增 generation_shell_test.go（bundle 指纹可重复/零 coverage 签发→PromoteCurrent→admission eligible→revoke→不 admitted 往返/action query fail-closed/typed 家族不弱化/未知家族 fail-closed）；
  - `processorregistry/registry.go`：generation_shell 注册行（executable、own PCA family、词表 [load_only]、**零 CoverageProofs**——PCARequiredCoverage fail-closed，typed 路由不可达）；registry_test 计数红线 11→12+行测试+**三处词表一致性**测试。
- **验收达成情况**：验收 ② 单测腿全绿（三处一致性+未知家族 fail-closed）+ 全量 `go test ./... -count=1`=**95 包 0 FAIL**+gofmt 净；验收 ① 真栈烟测**未执行**——被下述停止条件阻断（无认证签发则烟测第一步不存在，promoted 放行探针无法构造）。
- **停止条件触发（卡面预设分支，亲勘三层钉死，均为本卡文件域外）**：
  1. `agent/internal/chat/processor_certification_entry.go:435`——HTTP 认证入口 `entry.IsInstrument` 直接拒 instrument（GenShell=instrument 壳，件四 §2 明确分类走 synth/instrument 既有规则）；
  2. `agent/internal/processorauthority/local_processor_certification.go:234-252`——`validateV2CertificationTools` 硬映射表无 generation_shell 工具对（卡面锚点 234-247 本体），CertifyProcessor 入口即拒（本机 pcactl 实证：语义索引先行查找挡在更早层——本机 719 条索引无 GenShell，需起栈扫描重建后可达该墙）；
  3. `agent/internal/processorauthority/receipts_v2.go:74-88`——ImportReceiptsV2 候选硬要求 typed 控制门（apply/restore passed+write_count>0+roles→非空 axes），零 coverage 家族证据结构上不可通过。
- **装载腿已就位（零 harness 改动）**：装载门对 promoted 记录家族无关（harness.go:2285-2327 扫两 store；admission=QueryCurrentAdmission→QueryLibraryAdmissionV2，eligibility.go:130-166）——词表就位后 generation_shell 即可被准入路由；单测已证 store 级签发→晋升→admission 往返。
- **上交换方案（卡面预授权两条+执行侧补充一条，决策侧裁定）**：
  - **A**（卡面原案）：定义 `gen_shell.inspect` 只读工具对——需 processorauthority（接受 inspect-only 对）+chat 入口（放行 instrument+家族）+typed 工具面+receipt 零 coverage 分支，全部域外；
  - **B**（卡面原案）：认证 token 通道——现行 `authorizeCertificationTokenLoad`（harness.go:2192）硬编码 FamilyStaticEQ 且只授权装载不签发；扩展为 load-only 签发=harness+签发入口改造，域外（本机 pca_certauth.token 在位已 armed）；
  - **C**（执行侧补充）：load-only 签发入口小卡——store 级语义（零 coverage 签发/晋升/admission）本卡已落地并单测覆盖，缺的只是可执行入口（pcactl 子命令或认证入口放行），涉及 cmd/pcactl 或 chat，均为本卡域外。
- **烟测脚本 scripts/gen_shell_pca_smoke_mac.sh 未写**：签发步形态取决于方案选择，先写必然返工；方案裁定后修订卡补（脚本域在本卡内，随修订卡走）。
- **端测边界声明**：本回执全部证据=代码锚点亲读+单测（95 包 0 FAIL exit 0）+pcactl 本机探针；**无真栈 run**（烟测被停止条件阻断，无 run ID 可报）。
- 验收：（裁定文件 / 验收 commit）
