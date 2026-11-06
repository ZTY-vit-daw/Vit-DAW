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

- （执行侧回填：会话 / worktree / 分支 / 领取基线 HEAD + status + diff --stat）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
