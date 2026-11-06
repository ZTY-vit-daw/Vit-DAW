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

- （执行侧回填）

## 验收

- （主管回填）
