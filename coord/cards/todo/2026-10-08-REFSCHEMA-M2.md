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
- **首轮回执裁定（2026-10-08 晚窗决策侧）**：实现面/兼容面/测试面验收通过；全量门因卡外 mixboard 预算门饱和未达（§11 上交成立——HEAD 基线亲验通过+余量 16B 探针采信，ref 变长为 G1 身份族语义结构性必然）。**补证返工授权（扩域两行）**：①`agent/internal/mixboard/mixboard_test.go:596` 预算 50000→70000（护栏目的=原始波形泄漏防护，由 forbidden-string 断言独立承载不因调值受损；70000=M2 后实测 58399+~20% 余量）+行内注释注明调值依据；②`docs/REF_SCHEMA_V1.md` §6 词汇表补三个 scope_kind（mix.read/acoustic_package_status/observation——回执 §5 登记同步）；③复跑全量 0 FAIL（chat TempDir 竞态若再现：按 §11 分类记录并隔离复跑，二次再现即上交开修复卡）。原分支追加提交后交付。
- 领取：2026-10-08 晚窗 / origin/main 509e37f9 / port/refschema-m2（独立 worktree D:/Vit_DAW_wt_m2，树净）；锚点实核无漂移（evidence.go:24/31/38；refschema.go:151 observation: legacy 条、:170 mom kind 已在 projectionKindRegistry——kind 无需再注册）
- 回执：port/refschema-m2 `ec0b42f4`（coord/runs/REFSCHEMA-M2/receipt.md 随分支提交）——三构造器+13 处消费面迁 vit://mom（scope=数据键族/snapshot=observation_id），文法零改动、kind 已注册；新测试 8 个全绿（含 legacy 三态兼容回归）；build exit 0、mom/agentprotocol 全绿；**全量 88 包 ok + 2 FAIL**：①mixboard 预算门饱和（HEAD 亲测 49984/50000 余 16B，本卡 165 refs×~51B→58399，泄漏断言全过仅字节上限爆——snapshot=observation_id 强制形态下卡内不可解，预算门在卡外 mixboard_test.go:596，按 §11 上交裁定，卡内停止条件未触发）②chat TempDir 清理竞态（环境中断型，隔离复跑过，与本卡无关）；gofmt blob 级净
- 验收：（裁定文件 / 验收 commit）
