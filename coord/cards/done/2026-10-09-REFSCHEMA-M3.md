# REFSCHEMA-M3：A 类四包 stableProjectionID 迁 vit:// 带 sha256 段（dom/fxm/com/rlm）

- 池序 27（[G1 终审 M3 行](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4：A 类四包；M2 已合入 433a734f/06d275f6）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / REFSCHEMA-M2（vit://mom 形态先例+注册表已扩）
- 模型分级：L1 / **flash 可接**（M2 同型先例：构造器改写+对照测试；与 LEDGER-DIR-1/BOM 卡不同文件域，可并行）
- 目标（G1 终审 M3 行规格）：dom/fxm/com/rlm 四包 ProjectionID 从 legacy 形态迁 vit:// 带 sha256 段——内容哈希现成（stableProjectionID 即内容哈希），snapshot 承载按 F6 注记。
- 已核实锚点（领取时回查行号，漂移即修正；`rg -n "stableProjectionID" internal/dom internal/fxm internal/com internal/rlm` 全量清点后列清单入回执）：
  1. dom：`projection.go:530` 定义，:93/:138 两处赋值。
  2. fxm：`projection.go:207` 定义，:79 赋值。
  3. com：`projection.go:467` 定义（:57 赋值）；`change.go` :67/:82（测试 rehash :193）；`paired.go` :119/:155。
  4. rlm：`projection.go:835` 定义，:120 赋值。
  5. 注册表：`agentprotocol/refschema.go`——四 kind（dom/fxm/com/rlm）按 M1/M2 同型核对是否已注册，未注册走注册表数据新增（kind 封闭枚举扩展非文法改动）。
- 语义红线：vit:// 文法（kind/scope/window@snapshot#hash 双段强制）零改动；sha256 段=内容哈希（stableProjectionID 现成语义）；旧记录 legacy refs 必须可解析（legacy 条保留=兼容读）；ProjectionID 消费方（去重/缓存/测试断言）行为预期核对——ID 形态变化若触发消费方断言需如实分记不弱化。
- 验收标准：四包构造器产出 vit:// 形态+ParseRef 全部 RefStateParsed；legacy 兼容读回归（旧 fixture 往返）；四包+全量 `go build ./...`+`go test ./... -count=1` 0 FAIL+gofmt 净；消费面清单（改动处全列）入回执。
- 停止条件：任一族 stableProjectionID 实为非内容哈希（含时间戳/随机量）→ 该族单独上交（升 G1 终审记录缺口，不私改文法）；scope/snapshot 承载争议 → 申报两形态上交裁定。
- 领取：2026-10-09 执行会话 / origin/main 935b15a4 / port/refschema-m3（独立 worktree D:/Vit_DAW_wt_m3，基于 origin/main 新建，树净）
- 回执：port/refschema-m3 `b9d7efc9`（coord/runs/REFSCHEMA-M3/receipt.md 随分支提交）——四包 stableProjectionID 迁 vit:// 带 sha256 段：种子输入集逐字节不动（dom/com 置空 GeneratedAt=内容身份；fxm/rlm 时间戳参与=实例身份，REF_SCHEMA_V1 §7 明文裁定+F6/QUERY_ENGINE §3.3，停止条件未触发），digest 前 16hex 进 #hash 段（F3）；scope=targetScope 物化层先例（TargetRef kind/id 缺省 target/unknown；rlm=project:current 单例），snapshot=observation_id（rlm=GeneratedAt 实例代，canonical 转义往返）；**kind 经 legacy 表 TargetKind 已在 registeredKindSet，注册表零改动**；vit:// 文法零改动（agentprotocol 零触碰）；legacy 条原样保留+旧记录 JSON 往返回归。锚点实核 22 处零漂移。新测试 7 个全绿（四包 refschema_m3_test.go：构造 parsed/段核对/身份族回归/转义往返/legacy 记录往返/change 子 ID 同形态）；**卡外唯一改动=mixboard/persistence_v2_test.go:127 写盘断言两侧同口径小写**（原断言依赖 legacy 全小写 hex 巧合，vit:// snapshot 段嵌时间戳大写 T 后失配；持久化内容本就正确，断言强度不变，如实分记）。验收门全过：build exit 0、**全量 90 包 0 FAIL（exit 0）**、gofmt blob 级净；M2 预算门未触发（ObservationMaxBytes 2MB/mixboard 70000 均未触线），无扩域需求。消费面清单（9 项含 queryengine legacy 路由零触碰/harness 同源断言过/materialize 透传过）详见回执 §3
- 验收：（裁定文件 / 验收 commit）
