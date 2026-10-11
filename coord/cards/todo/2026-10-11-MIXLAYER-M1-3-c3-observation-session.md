# MIXLAYER-M1-3：C3 空间与深度·观察侧收口（只读能力 Session）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；MIX-CAP-RECON-1 §3 C3 行+M1 表）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 66；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 无硬依赖（可与 M1-1 并行）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- **与 M1-4 文件域高度重叠（canary seam/routing）——建议同 owner 串行或先后合入，不并行**

## 目标

`fine_mix.space_depth.v0` 以**只读能力 Session** 形态接入：canary seam 增 C3 分发（`capability_runtime_canary.go:109-126` 现六分支无 C3）；capabilityIDForStage 映射已有（`capability_routing.go:619`）。观察载体现有面：

- FXM 差分手动通道（fxm/types.go 结构齐备、`mixboard.go:3992` req.Args 反序列化消费点——**本卡不建自动差分生产者**）；
- 通用插件参数快照（plugin_param_guard 依赖面）；
- MOM static_level_relationship。

产出=空间/深度相关观察报告 Session（无写控制）。

## 边界（显式排除）

- 语义 reverb/delay 控制面不在本卡（内核 plugin_grabber 无 reverb/delay inspector，随 C3 控制侧另卡）；
- FXM 自动差分生产者不在本卡（时域线 M2 范畴）。

## 文件域

chat/ capability seam（capability_runtime_canary.go+capability_routing.go）+ 新 C3 只读 runtime 文件 + 单测。

## 并行与资源

见卡头：与 M1-4 串行；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 单测：Session 分发+只读性（无写命令注册）。
- 「C3 观察在自由态循环内可达」冒测断言（模式=fastpath 注册清单完整性契约，MIX-CAP §3 建议）。
- 真栈烟测 exit 0；`go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

canary seam 结构与锚点不符（六分支已变）→ 锚点上交。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
