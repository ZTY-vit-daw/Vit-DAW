# EDU-SYMBOL-CORE-1：符号分析引擎——变换族检测核 + analyze_midi_transformations 命令（乐局 1a / 注册链①）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-9/§六](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [256 报告](../../reports/2026-11-05-edu-cap-recon-1.md) 点①草案 + [执行排序](../../reports/2026-11-05-nov-three-line-execution-plan.md)）
- 派发确认：已确认（用户 2026-11-05 批准入池，明日按序执行）
- 验收负责人：GLM 主管决策流
- 池序 276；**注册链第 ① 位**（动 catalog.go/policy.go/capability_pack.go；本卡提交前 277 不得领取）；目标=`agent/internal/edu/`（新建包）+三处注册；P1 / 2.5-4 天 / 依赖=无；L1-L2 / PC 执行会话；真栈烟测腿一次

## 目标

1. **变换族检测核**（`edu/transform.go` 或同包）：音程序列提取（pitch→半音差）+ 四变换统一检测（repeat/sequence/inversion/retrograde=音程序列在变换下的相等性）+ 节奏比对；确定性纯函数，同输入同输出。
2. **命令面**：`analyze_midi_transformations`（工具名 `midi.analyze_transformations`，RiskDirect 只读）——输入/返回 schema 按 256 报告点①草案（clip_id / range_beats / transformations / min_match_notes；返回 bar 锚定 findings + music_context_echo + llm_context 字段族借 EPM 惯例）；**bar 守门**：`music_context.tempo_setting_count==1` 才出 bar 字段，否则 null+limitations。
3. **判定器**：学生片段 vs 原型比较（错音定位=逐音程序列 diff），同核复用。
4. **三处注册**：catalog spec 行 + policy directCommands + capability_pack（midi pack Tools 补一行；详细引导归 282，本卡只注册不加引导段）。
5. **黄金用例单测**：小星星主题（A 段后半↔B 段调内模进+B 段两遍原样重复）——检出两关系、区分 repeat/sequence、shift 值正确；负例（无关系→空 findings）；边界例（同节奏异轮廓不误报）。fixture 新建测试资产，不碰样例工程（§10）。

## 约束

- 文件域=`agent/internal/edu/`（新建）+ catalog.go/policy.go/capability_pack.go 各一处行级注册 + 测试；**零内核改动**；不触 semantic_entry/automation（M3 域）。
- JSON 返回字段名与 256 报告草案一致（消费方=乐局卡/编曲 SYMLINK）。

## 验收标准

- `go build ./...` + `go test ./internal/edu ./internal/tools ./internal/policy -count=1` 全绿；既有测试零回退；
- **真栈烟测腿**（§5）：chat 驱动调命令→返回断言（小星星 fixture 导入后分析，findings 与单测一致），扩现有冒测脚本，**脚本 exit 0**；
- 回执附：注册三处 diff 摘要、单测清单、烟测 run ID 与工件路径、commit hash。

## 停止条件

- 检测核对黄金用例反复不过→保存失败输出上交，不硬编码曲目特例（§10）；栈/环境中断按 §9 取证。

## 领取

- 2026-11-06 / **段 1/N 领取**（变换族检测核+判定器+黄金用例单测，全新包 `agent/internal/edu/`；段 1 不做三处注册/命令面/真栈烟测）
- origin/main hash：`84f86f9e01b9cef764e5ae8fce6060fe0a8345b3`
- owner：GLM 主会话 L2-worker 子代理（GLM-5.3 / PC）
- worktree：`D:/Vit_DAW_wt_276impl`；分支：`port/edu-symbol-core-1`（自 origin/main 新建）
- 领取基线：HEAD=`84f86f9e01b9cef764e5ae8fce6060fe0a8345b3`；`git status --short`=空；`git diff --stat`=空（worktree 全净，无领取前已有 diff）

## 回执

- **段 1/N 回执（2026-11-06）**：变换族检测核+判定器+黄金用例单测完成，全绿。
- 实现 commit：`20d4115d`（分支 `port/edu-symbol-core-1`，worktree `D:/Vit_DAW_wt_276impl`）；领取提交 `d4acae60`（已推共享 main）。
- **diff 文件清单**（全部为新建，零既有文件改动）：
  - `agent/internal/edu/edu.go`（139 行）——包契约+Note/Key 类型+音程序列提取/节奏比对/大调音级映射
  - `agent/internal/edu/transform.go`（356 行）——检测核 DetectTransformations：四变换统一检测（repeat / sequence[chromatic|diatonic] / inversion / retrograde=音程序列在变换下的相等性）+节奏比对（identity 逐位相等、retrograde 镜像）+极大匹配过滤+退化优先级链（identity>inversion>retrograde）+确定性排序
  - `agent/internal/edu/judge.go`（102 行）——判定器 JudgeAgainstPrototype：学生片段 vs 原型逐音程序列 diff 错音定位（移位假设换算期望值、恢复点抑制），同核复用
  - `agent/internal/edu/transform_test.go`（408 行）/ `agent/internal/edu/judge_test.go`（159 行）——19 用例
- **黄金用例**（fixture=测试内联 42 音小星星，C 大调 4/4，未触碰样例工程）：乐句2↔乐句3 调内模进检出（DiatonicSteps=+1、ShiftSemitones=2）且与乐句3↔乐句4 原样重复（Kind=repeat、shift=0）严格区分；A 段整体再现（14 音 repeat）附带检出；负例（无关系→空 findings）、边界例（同节奏异轮廓纯调内序列→空）、chromatic/inversion/retrograde 正例、同音退化只报 repeat、MinMatchNotes 阈值、乱序输入/重复调用确定性、无 Key 时 diatonic 不报、变换白名单、非法选项 fail-closed 均覆盖。
- **测试命令与退出码**（`agent/` 下）：`go build ./...`=0；`go test ./internal/edu -count=1`=0（19/19 PASS）；全量 `go test ./...`=0（零回退）；`go vet ./internal/edu`+`gofmt` 干净。
- 设计决策存档：sequence 双语义（chromatic=半音恒定移调 / diatonic=大调音级恒定模进，后者需 Key——小星星 A 段后半↔B 段为调内模进，半音差 [2,1,2,2] 非恒定，纯 chromatic 检不出，故引入 diatonic 档）；ShiftSemitones=首音半音差（段 2 命令面 label 用）；段对约定=不相交且匹配段在后；O(N²·L) 复杂度，长 clip 由段 2 range_beats 分窗。
- 段 1 未做（按派发指令留守段 2/3）：catalog.go/policy.go/capability_pack.go 三处注册、analyze_midi_transformations 命令面、真栈烟测；零内核改动、零栈占用（无 PC-RUNTIME-STACK 登记需求）。

## 验收

- （主管回填）
