# EDU-LEVEL-RUNTIME-A：关卡运行时·状态机——jam_state + 判定器注册表 + lesson_card 校验门（乐局 3a）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 报告](../../reports/2026-11-05-edu-runtime-recon-1.md) 卡 A 原案 + rulings §五-16/17）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 280；前置=279 定稿；目标=`agent/internal/edu/` 纯数据层；P1 / 0.5-1 天；L1 / PC；零栈零 LLM

## 目标（257 报告 edu 包三件套之状态层）

1. **`edu/jam_state.go`**：乐局状态类型——schema_version、conversation_id+project_uuid 双绑定、节点数组（node_id/state/attempt/entered_at）、事件追加表；**纯数据零 controller 引用**（§五-16 接口不变式：不进 orchestrationcontroller 枚举、不进路由白名单）；status 枚举含 suspended 值（12 月不用，仅枚举占位+旧值缺省语义随卡声明，§11）。
2. **`edu/judge.go`**：判定器注册表+首版 3 个（note_sequence_match/pitch_set_match/rhythm_match）——输入=读取命令返回的 notes 数组（纯数据），确定性零 LLM；推进=纯函数 `(state, observation) → state'`。
3. **`edu/lesson_card.go`**：乐局卡加载+校验门——小节存在、检查方式有判定器（**未知判定器名直接拒绝**，专家写不出不存在的检查方式，257 修订①）；旧状态缺省语义+restore fail-closed（mix_workflow_queue_view.go:670 模式）。
4. **单测**：状态迁移正反例、判定器三件、校验门拒绝路径、旧 JSON 反序列化往返（§11）。

## 约束

- 文件域=`agent/internal/edu/` 新文件+测试（与 276 同包不同文件，领取时确认 276 已提交避免叠加）；零注册面改动；挂起接口=零实现零字段预留（§五-16）。

## 验收标准

- `go build ./...` + `go test ./internal/edu -count=1` 全绿；279 的乐局卡 v0 能被校验门通过（互测）；
- 回执附状态机迁移图文本+校验门拒绝用例清单。

## 停止条件

- 279 格式与状态机设计假设不符→停，双方上交决策侧对齐。

## 领取

- 2026-11-07 / owner=GLM 主会话 L2-worker 子代理（ZCode / PC）；验收负责人=GLM 主管决策流
- worktree=`D:/Vit_DAW_wt_280impl` @ 分支 `port/edu-level-runtime-a`；卡面协调提交走 `coord/280-edu-level-runtime-a`（worktree `D:/Vit_DAW_wt_280coord`，PROTOCOL §2.1-2）
- 领取基线：origin/main `a8be057e`（279 定稿终验 pass 后 HEAD）；impl worktree `git status --short` 空、`git diff --stat` 空（净树领取）
- 文件域=`agent/internal/edu/` 新文件+测试（jam_state / 判定器注册表 / lesson_card 校验门；276 合入的 judge.go 等既有文件不修改，注册表落新文件）+ `agent/go.mod`/`go.sum`（yaml 解析依赖 gopkg.in/yaml.v3 v3.0.1，模块缓存已有，离线可加）+本卡面；零栈零 LLM；零注册面改动（不碰 orchestrationcontroller/路由白名单）；capability_pack 不触（282 前）

## 回执

- 2026-11-07 / GLM 主会话 L2-worker 子代理（ZCode / PC）；impl commit `b63917ae` @ `port/edu-level-runtime-a`（已推 origin）；8 文件 +2878/-1
- **产物**（`agent/internal/edu/` 新文件×3+测试×3；276 既有文件零修改）：
  - `jam_state.go`（539 行）——`vit.jam_state.v0`：schema_version+conversation_id/project_uuid 双绑定（NewJamState 空值拒绝）+节点数组（node_id/state/attempt/entered_at）+事件追加表（seq 递增只增不改）；AdvanceJam=纯函数 `(state, observation) → state'`（时间戳由 Observation.At 注入，零时钟零 IO）；restore fail-closed（mix_workflow_queue_view.go:670 模式：未知 node_id/状态值 drop+计数 never execute；未知 schema_version/level 绑定不符整体拒绝；未知事件种类审计保留原文不驱动迁移）；**零 controller 引用实证：包 imports=stdlib+yaml.v3，无任何仓内依赖**
  - `judge_registry.go`（184 行）——首版 3 判定器闭集注册表：note_sequence_match 复用 276 `JudgeAgainstPrototype`（accept_transposed=卡面裁决落点：transposed_match 仅真值时算过）；pitch_set_match=音高多重集（missing/extra 对称披露，双侧非空才可过）+set_mode（pitch_class 默认容八度错位/absolute 严格）；rhythm_match 复用 `rhythmEqual`（音数不等即 fail）；LookupJudge 未知名 false=拒绝路径 1 运行时同型
  - `lesson_card.go`（1173 行）——`vit.level_card.v0` 加载+校验门：yaml.v3 严格解析（KnownFields 顶层未知字段拒绝）+叶子未知参数字段拒绝（Keys 捕获）+12 拒绝路径+6 新增对齐项（清单见下）；`evidence_hooks.params` **整包复用 276 `parseAnalyzeArgs`**（路径 5/6/7/8 与命令面零语义漂移）；`AssetNotes` 合成 id `asset:<id>#<k>`+derived 链 deviations 应用（新增项⑤：内嵌表=判定输入源，零栈可判）；LoadLevelCard 文件级核对（资产 sha256 对账+persona 源存在+id↔目录名收口）
  - 测试×3（976 行）：**夹具=279 定稿 level.yaml 唯一**（mutation harness 无突变对照自证）；276 包内测试零回退同跑全绿
- **状态机迁移图**（验收标准要求项）：

```
pending ──enter──▶ active ──done_when ✓──▶ done（终态）
                      │
                      └─judge attempt ✗──▶ active（attempt+1，事件 attempt_failed）
事件/结构观察（无 judge 输入）──▶ 原状态（不计 attempt；轨槽未观察=数据缺口非错误）
全部节点 done ──▶ Advance 返回 ErrJamCompleted（幂等哨兵）

枚举占位（12 月不用，§五-16 挂起接口=零实现零字段预留）：
  active→suspended / suspended→active 已声明合法迁移但 v0 无入口 API
  旧状态含 suspended：restore 原值保留（§11 缺省语义）+Advance fail-closed（ErrSuspendedNode）
```

- **校验门拒绝用例清单**（验收标准要求项；每项独立突变定稿夹具断言拒绝原因，测试名 TestGateReject*）：
  1. done_when 未知 judge 名（n1→"chord_match"）｜2. 未知 event（human_replied→"user_typed"）｜3. 未知 structure（clip_exists→"clip_playing"）｜4. bar 越界三态（range_bars 超出 bar_span [12,14]／bar_span 扩后窗口不被 notes 表覆盖 [12,13]@span13／range_beats 超 span [0,52)）｜5. 区间形态三态（逆序 [48,0]／range_bars 换算与 range_beats 不一致 [2,12]vs[0,48)／非二元组）｜6. min_match_notes=1（复用 parseAnalyzeArgs 同错误语义）｜7. key.tonic=12｜8. transformations=["canon"]｜9. node.id 重复（n6→n5）｜10. 资产引用四态（reference.asset 未知／删 sha256／preload 未知资产／notes_ref 未知表）｜11. schema_version="vit.level_card.v1"｜12. objectives[].check 未知 judge（verbal_rubric 豁免由夹具 obj2 阳性实证）
  新增①=2/3 枚举闭集；②set_mode 未知值/仅 pitch_set_match 可带（合法形态阳性另证）；③accept_transposed 仅 note_sequence_match 可带（夹具 n1/n3 两处 false 落位实证）；④组合子嵌套超一层；⑤AssetNotes id 形态+deviation 应用（corrupted#33=68 vs theme#33=69）；⑥runtime_slots 必填+track_index 重复。附加：顶层未知字段/叶子未知参数/hook 未知参数 fail-closed。
- **互测基线**（279 附录 A 对账）：judge(themes vs corrupted 全曲)恰 1 处 mismatch idx33/actual 68/expected 69/prototype 69/rhythm_matched=true——与 279 机械复算基线逐值一致
- **测试与退出码**：`go build ./...` exit 0（全量）｜`go vet ./internal/edu` exit 0｜`go test ./internal/edu -count=1` ok（含 276 既有测试=91 个测试函数，0.5s）｜`go test ./internal/chat -run 'Test.*(Event|Message|Lifecycle)' -count=1` ok（12.9s，§6 健康子集零回退）
- **runtime_slots 裁定建议（280 内裁定项，交主管）**：**建议升格必检**——卡内所有 clip 寻址（judge target.clip/structure 谓词/hook params.clip_id/finale no_placeholder_blanks）以轨槽名为坐标系，无声明块则这些引用退化为悬空字符串、拒绝路径 10 出现系统性缺口；279 唯一实例已带块且自检通过=升格零返工；281（3b 挂载）需轨槽→实际 track 绑定表，卡内声明是挂载静态前提。**不硬编码轨数**（=3 是 first-run-twinkle 实例约束，留内容侧 lint check_level_yaml.py）——未来单轨教学卡不被通用门误杀。已按此实现（块必在+track_index 唯一+preload 可解），主管可翻为实例惯例（届时去掉必填断言一条即可）
- **判定器语义定案提案（开放⑤，交主管盖章）**：pitch_set_match=音高**多重集**相等（重复音计数保留——参照两次学生一次即 fail；音乐上缺音不可容认）+set_mode 二值闭集（默认 pitch_class 容八度错位=初学者宽容；absolute 严格）；accept_transposed 仅 note_sequence_match 可带（其他判定器带=校验门拒绝，参数纪律 fail-closed）；组合子深度限一层（实例全单层，专家可写性）
- **诚实边界与有意不检项**：①write_boundary.allow 不对命令注册面核对（edu 包禁 import 注册面=分层约束；279 定稿轮已对 @main 注册面九命令实名核对，内容侧 lint 持续覆盖）②MANIFEST 三方对账留内容侧（gate 只做卡↔实际文件 sha256）③notes 表单调性不强制（运行时 sortedNotes 排序，专家可乱序手写）④confirm_downgrade v0 fail-closed 只认 none（裁定 2026-11-07；新值须先扩门）⑤文件域边界：go.mod/go.sum 属机械依赖追加（yaml.v3 v3.0.1，领取记录已声明），无其他域外触碰
- **端测覆盖边界声明（AGENTS §5）**：本卡纯数据层零栈零 LLM——渲染面与用户旅程门槛不适用；真栈校准（乐局运行时挂载+烟测）=281（3b）卡职责，本卡交付物已被 281 消费的接口面=LoadLevelCard/AdvanceJam/RestoreJamState/LookupJudge/EvaluateNodeConditions
- **状态**：自验通过，卡转 done 待主管验收；281 就绪（3b 挂载可直接以本包 API 落地；真栈腿须按 PROTOCOL §2.2 登记占用）

## 验收

- （主管回填）
