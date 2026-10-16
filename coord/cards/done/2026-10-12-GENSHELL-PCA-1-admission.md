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
- 领取（复工·修订版）：2026-10-16 深夜 / origin/main=ca86b226 / owner=GLM 执行会话（ZCode Mac 执行流，GENSHELL-PCA-1 修订复工专会话）/ 分支 port/genshell-pca-1（延续，前腿 impl=4d779236 在顶）/ worktree=/Users/timozty/Documents/Vit-DAW-pca1（延续）/ 领取提交=（本提交）
- 领取时核对（复工）：远端 ca86b226 亲核卡在 todo+修订节（方案 B）在场；GenShell.vst3 在位；origin/port/genshell-pca-1=4d779236 与本地分支顶一致；唯一 doing 卡 LOUDNESS-B-TRIGGER 文件域（loudnesswatch/*+chat/free_state_conclusion_bridge.go+chat/free_state_reasoning_loop.go+harness/harness.go+各自测试+fs_largeproject_smoke.ps1，impl=64b3ccfd 回执亲读）与本卡修订域（processorintent/processorattestation/processorregistry 三包+启动接线一处+scripts/gen_shell_pca_smoke_mac.sh）零交集——seed 接线按 LOUDNESS-A-CONFIG 先例走 chat/server.go 装配模式（server.go:164/198/7245/7256 锚点族），不触 harness.go；真栈烟测走 Mac 栈，LOUDNESS-B 重跑腿同栈但串行按 §2.2 资源记录协调

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

---

## 修订（2026-10-16 深夜，决策侧）：方案 B 裁定落地——native 永久准入，blocked→todo 复工

- **用户裁定**（[decisions/2026-10-16-pca-b-native-and-gen-source.md](../../decisions/2026-10-16-pca-b-native-and-gen-source.md)）：GenShell=原生打包插件，非第三方效果器，直接永久认证（方案 B 原则）。
- **落地设计（决策侧裁决，实现上述原则）**：不走 token 介质，改由 **registry 注册事实驱动的 native 永久准入**——agent 启动时对 processorregistry 声明的 native load_only 家族（generation_shell）**幂等 seed 一条永久 v2 attestation**（绑定 GenShell.vst3 bundle 指纹，复用本卡前腿已落地的零 coverage 签发→晋升→admission 语义）；装载门/admission/audit 全走现有查询路径。**三道现行门零改动**：chat 认证入口不动、token 通道（harness.go:2192）保留一次性授权语义不动、typed coverage 硬要求不弱化。
- **复工范围**：①前腿词表（impl=4d779236 @ port/genshell-pca-1 已推）延续同分支续做；②新增 seed 函数（processorattestation 包内，native 家族幂等 seed+bundle 指纹绑定+revoke 尊重——运行时 revoke 生效即拒，重启 re-seed=native 家族会话级语义，回执如实声明）；③agent 启动接线一处（按 LOUDNESS-A-CONFIG 锚点模式定位装配点）；④补 `scripts/gen_shell_pca_smoke_mac.sh`。
- **文件域（修订后）**：原三包（processorintent/processorattestation/processorregistry）+agent 启动接线一处+smoke 脚本。**不触** chat 认证入口/local_processor_certification 工具对映射/receipts_v2 typed 控制门（三处按裁定保持原样）。
- **验收标准（修订）**：①真栈烟测 exit 0：agent 起栈→（seed 自动发生）→agent 路径 rack_add_node 装载 GenShell 成功（job/audit 留痕）→运行时 revoke seed 记录→同进程再装载被拒（pca_load_gate 错误串断言）→UI 路径（直接内核命令）装载不受影响回归→重启 agent→re-seed 后装载恢复；②三处词表一致性单测+native seed 幂等/revoke 单测；③全量 `go test ./... -count=1` 0 FAIL+gofmt 净。
- **停止条件（修订）**：seed 与现有 store 生命周期冲突（promoted 淘汰策略覆盖 seed 记录）→ 锚点上交换持久层决策；装载门 admission 查询路径与 seed 记录形态不符（eligibility.go:130-166 之外的分叉）→ 上交。
- **端测边界**：agent HTTP 面断言；无 webui/前端改动。

## 回执（2026-10-16 深夜·修订版复工，方案 B 落地）——自验全绿，交验收

- **impl commit=a198e1c3**（port/genshell-pca-1，已推 origin；基线=前腿 4d779236，本腿 4 改+4 新，全部落修订卡文件域）：
  - `processorattestation/native_seed.go`（新）：`SeedNativeLoadOnly`——load-only 家族幂等 seed：先查 store 同 subjectKey+family+**当前 bundle 指纹**的 promoted 记录，命中即返回零写入（重启幂等，Revision 不变有单测断言）；未命中→零 coverage 签发+PromoteCurrent（bundle 升级=新指纹 supersede 旧 promoted 转 stale，走 store 既有语义）；**revoke 尊重**=运行时 revoke 后本进程内不再 seed（装载即拒），下轮 seed 新签发新记录（re-seed=会话级语义，与裁定一致）；typed 家族（词表非空）fail-closed 拒；
  - `processorattestation/native_seed_test.go`（新）5 测：签发→promoted→admission eligible / 幂等零写入（Revision+条目数不变+同 ID）/ revoke→admission 拒→re-seed 新 ID→恢复 / bundle 指纹变化→supersede（旧 stale+新 promoted+新指纹 admission）/ typed 家族+未知家族+缺 bundle+缺 identifier 全拒；
  - `processorregistry/registry.go`：`Definition.NativeSeed` 声明字段+`NativeSeeds()` 遍历 API+generation_shell 行登记部署事实（`~/Library/Audio/Plug-Ins/VST3/GenShell.vst3`、identifier=`VST3-GenShell-15b5622d-b818da1c`、Vit Audio/VST3）；Register 校验：native seed 仅限 PCA 词表空集家族+字段完整（typed 家族声明即注册失败）；registry_test 补声明完整性+typed 拒两测；
  - `chat/server.go`：`seedNativeProcessorAdmissions()`（遍历 registry native 声明→~ 展开→seed→Info/Warn 留痕；失败 WARN 不阻断启动）+ `chat/continuation_scheduler.go` Start() 接线一行——**进程启动一次**，测试/嵌入式 Server 构造不调 Start 不触发（注释既有语义），不污染测试 store；
  - `scripts/gen_shell_pca_smoke_mac.sh`（新）：真栈烟测五腿。
- **验收①真栈烟测 exit 0**：run=**genshell_pca_mac_20261016-231931**（内核 8624fe9e=CPP-1 验收同二进制零改动+agent 本卡改动新构建；**12 ok/0 red**，工件 `coord/runs/GENSHELL-PCA-1/genshell_pca_mac_20261016-231931/`）：LEG A seed 断言（store promoted+指纹=对部署 bundle 现算 sha256+identifier 匹配+log `[pca-native-seed] seeded=true`）→ LEG B agent 路径装载放行（authority full_project_access→invoke rack_add_node status=ok，agent_action_id=act_9490d52968602203 留痕）→ LEG C 运行时 revoke seed 记录→**同进程**再装载被拒（`pca_load_gate: autonomous full-access load rejected: no_promoted_current_pca_admission_for_identifier`）→ LEG D UI 路径（直接内核命令，revoke 态下）装载 ok（plugin_id=1053——UI/C++ 面不经 Harness 门回归）→ LEG E 杀 agent 重启→re-seed 新记录（pca2_14cab…≠revoked pca2_2ee3…）+log seeded=true+装载恢复（act_5b6accc5a4346fba error= 空）。隔离（AGENTS §10）：store/语义索引/工程文件/VSP 注册全部指 run 目录，不碰 ~/.vit 真实面。
- **验收②单测**：native seed 5 测新增全绿+前腿三处词表一致性保持全绿；**验收③全量** `go test ./... -count=1`=**95 包 0 FAIL**+触碰文件 gofmt 净。
- **三道现行门零改动亲核**：chat 认证入口（processor_certification_entry.go）未触；token 通道（harness.go:2192 authorizeCertificationTokenLoad）未触——烟测 LEG C 的拒路径中 token fallback 因隔离空语义索引无法解析身份而拒（通道语义本身未改，真实 token 面未动）；typed coverage 硬要求未弱化（typed 家族 seed 入口 fail-closed+validateIssueSpecV2 不变量保持）。
- **停止条件（修订）未触发**：store 无淘汰策略（grep 无 evict/prune）→seed 记录无被覆盖风险；装载门 admission 查询路径与 seed 记录形态吻合（promotedProcessorAuthorizationRecords 按 Identifier 匹配+QueryCurrentAdmission 指纹比对，seed Subject 即按内核枚举事实构造）。
- **端测边界声明**：全部断言=agent HTTP 面+内核 ZMQ 面+store/log 文件；无 webui/前端改动（渲染面零涉）；UI 路径回归=直接内核命令（与 CPP-1 K2 先例同形态）。如实边界：①PC 未部署 GenShell.vst3 时 seed WARN 跳过（registry 声明的是 Mac 部署事实，PC 部署后同声明自动生效）——PC 全量 95 包测试不受影响（seed 不在测试路径）；②revoke=会话级语义（重启 re-seed 恢复）为裁定既定行为非缺陷；③identifier 由 GenShell 固定 FUID 派生（CPP-1 两轮+本烟测共三轮实证恒定），未来若变更 FUID 需同步 registry 声明行。
