# GEN-CTX-KEYFIELD-1：调性/风格工程字段持久化（统一化实施②）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=设计件五 卡2）
- 派发确认：已确认（用户 2026-10-11 同步裁定）
- 验收负责人：GLM 主管决策流
- 池序 76；**规格全文=[设计件五 卡2](../../runs/GEN-UNIFY-CTX-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P1 / 1 天 / **GEN-CTX-DISCLOSE-1 合入后领取**（同文件域 CommandDispatcher.cpp 严格串行）
- 模型分级：L1 / Mac 执行流

## 要点摘录

set_project_key/set_project_style_note 两命令全链（edit state 属性+.vit 容器往返）+catalog 注册（RiskConfirm）；验收=§11 往返三验+四负例 fail-closed+披露联动；停止条件=容器剥离未知属性→上交换存储位。

- 领取：2026-10-12 10:10 +0800 / origin/main f02cf2c8 / GLM-5.3 Mac 执行流（ZCode 会话） / 分支 port/gen-ctx-keyfield-1 / worktree /Users/timozty/Documents/Vit-DAW-ctxdisclose1（刻意复用 DISCLOSE 已清理路径以续接 /tmp/vit-ctxd1-build 增量构建缓存；领取时核对 main 的 CommandDispatcher.cpp 已含 appendMusicContextDisclosure 披露块=亲验在位）/ 领取提交=本提交
- 回执（2026-10-12 18:40 +0800）：
  - **实现 commit**：`13551ab0` @ `port/gen-ctx-keyfield-1`（已推送）。文件域=CommandDispatcher.cpp（+274：keyfield 命名空间辅助+两 lambda handler+披露挂接，**零头文件改动**——lambda 注册形态守卡面单文件域，append_ghost_track 先例）+catalog.go/catalog_test.go（RiskConfirm+argspec+TestMusicKeyfieldToolsAreCataloged）+scripts/music_keyfield_roundtrip_smoke_mac.sh+SMOKE_TESTS.md 登记+run 工件。
  - **权威 run**：`run4-pass-20261012-183348` exit 0 **43/43 门绿**（工件 `coord/runs/GEN-CTX-KEYFIELD-1/run4-pass-20261012-183348/`）。基线内核 `8e445dbb`（DISCLOSE 时代基线，负责造"旧工程"）+候选 `b2f53fcd`（增量重建，警告零增量 56=先例同值）。§11 三验：旧工程（基线内核所造）load→save→load 缺省保真（null/""/无调试键）；set {root:"A",scale:"minor"}→auto-persist→reload **set_at 逐字节四段相同**（.506）；save_as 新路径（.vit 容器）同值往返。四负例 fail-closed（dorian23/root:150/root:"H"/501 字符全 status=error 且**存值不被污染**）+ionian 合法对照落地；清链（key→null 无调试键/style→""/幂等重清）；tempo 族回归守门绿。风格字段含多字节 UTF-8 往返。
  - **agent 面**：全量 `go test ./... -count=1` 93 包 0 FAIL+gofmt 净（最终态复跑 EXIT=0）。
  - **停止条件未触发**：.vit 容器不剥离属性（隔离实验证明整树序列化保真）；实际缺陷是变更注册缺失（详见下条），域内已修。
  - **迭代史（4 轮，含 2 个本卡功能缺陷）**：run1=候选内核段错误（`juce::var(keyValue.get())` 临时 var 引用计数 0→1→0 删对象→UAF；修=release() 一次性移交所有权）；run2=auto-persist 静默跳写（裸 setProperty 不注册变更→EditFileOperations hasChangedSinceSaved 门假→不落盘但回 true；修=`edit->markAsChanged()` 四变异点）；run3=内核修复确认+4 个脚本探针缺陷（kfield eval 无 json）；run4=权威。全记录见 `coord/runs/GEN-CTX-KEYFIELD-1/README.md`。
  - **域外引擎级缺陷上交（AGENTS §11，证据=forensic 目录）**：①set_tempo 在 open 既有工程后同样静默不落盘（set 111→ok→重开回 100；DISCLOSE 的 tempo 往返证明只覆盖 save_as 形态）——锚点 runAutoPersist→saveProjectToCurrentPathOrDefaultXml(:688)→ipcSaveProjectToCurrentPath(:1355)→tracktion EditFileOperations::save hasChanged 门；②auto-persist 任何实际落盘把加密 .vit 重写为明文 XML（系统性暴露，VitHeadlessService.cpp:1218 注释自证防降级但只护显式保存路径）。两者均非本卡引入，需决策侧另开卡。
  - **端测边界声明（AGENTS §5）**：真栈冒测=内核直连 ZMQ 面（脚本断言 exit 0）；agent 面=catalog 单测；无 webui/无用户旅程改动。读侧调试键分支（key_signature_raw/parse_error）防手改态文件，命令面不可达（写侧 fail-closed），本冒测不覆盖、已声明。
- 验收：pass（2026-10-12，Mac 决策侧先行裁定+PC 复核留档）——[rulings/2026-10-12-GEN-CTX-KEYFIELD-1-pass.md](../../rulings/2026-10-12-GEN-CTX-KEYFIELD-1-pass.md)；详裁定文件（KEYFIELD 含双内核独立复跑 PASS 与域外缺陷两件采纳立卡建议；SHELL 含与 genctx 渲染器契约互锁验证）
