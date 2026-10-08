# REFSCHEMA-M2：mom C 类构造器迁 vit://mom 形态（G1 迁移计划 M2 行）

- 池序 22（[G1 终审 M2 行](../../runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md)：痛点②③④根源；M1 已合入 00f96200）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / REFSCHEMA-M1（注册表 15 条含 observation: legacy 条）
- 模型分级：L1 / **flash 可接**（M1 先例同型：构造器改写+注册+对照测试）
- 目标（G1 终审 M2 行规格）：mom C 类构造器从 legacy 前缀迁 vit://mom 形态——`mix.read:`/`acoustic_package_status:`/`observation:` 三族（scope 承载数据键族；snapshot=observation_id）。
- 已核实锚点（领取时回查行号）：
  1. 构造器：`agent/internal/mom/evidence.go:24`（mix.read:track.*）/`:31`（acoustic_package_status:*）/`:38`（observation:<id>）。
  2. 消费面散点（构造调用处，全在 mom 包）：projection.go:80/:287/:313、masking_relationship.go:87、project_relation.go:37 等——`grep -rn "mix\.read:\|acoustic_package_status:\|observation:" internal/mom/` 全量清点后列清单入回执。
  3. 注册表：`agentprotocol/refschema.go:151` observation: 已为 legacy 条（M1 补入，Anchor=mom/evidence.go:34-39）；vit://mom kind 若未注册须按 M1 同型走注册（kind 封闭枚举扩展=注册表数据新增，非文法改动）。
  4. 语义红线：vit:// 文法（kind/scope/window@snapshot#hash 双段强制）不动；snapshot=observation_id 身份族语义（非内容哈希——沿 M1 的 observation: 头裁定）；旧记录里已落盘的 legacy refs 必须可解析（legacy 条保留=兼容读，新构造器产出 vit:// 形态）。
- 验收标准：三构造器产出 vit://mom 形态+ParseRef 全部 RefStateParsed；legacy 条兼容读回归（旧 fixture 往返）；mom 全包测试+全量 `go build ./...`+`go test ./... -count=1` 0 FAIL+gofmt 净；消费面清单（改动处全列）入回执。
- 停止条件：scope 段承载力不足（数据键族塞不进 scope 文法）→ 实证上交（升 G1 终审记录缺口，不私改文法）。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 构造器与消费面改动清单 / 新测试名 / 全量退出码）
- 验收：（裁定文件 / 验收 commit）
