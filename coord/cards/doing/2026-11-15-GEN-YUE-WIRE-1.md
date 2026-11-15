# GEN-YUE-WIRE-1：YuE2 成曲链真实接线——H6 六项（学生谱→方言→成曲→版本）

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=审计 H6 六项+§9 用户裁定（YuE2 依谱生成纯器乐、改提示词出版本）+[进度驱动裁定]；无工期估算）
- 派发确认：已确认（主管发卡）
- 验收负责人：主管决策流 + 用户听核节点（成曲版本人耳）
- 池序 372；P0；依赖=无（烟测腿候栈位）；PC

## 目标（审计 H6 逐项）

1. **MIDI→方言转换**：学生工程谱（旋律轨 clip）→`yue_local` ABC 方言的确定性转换（程序转换，不走 LLM 转述）；单元往返测试。
2. **prompt→条件映射**：用户改提示词→实际 `style`/结构条件映射，显示实发摘要（harness prompt 非空约束满足）。
3. **纯器乐条件**：`instrumental` 沿已验证谱面声部/休止路径真实下发（§9 用户已手测 YuE2 谱面跟随准确）。
4. **请求面卫生**：ExtraParams 固定配置不得覆盖学生 ABC/style；两版本=两次 job（count 不支持多候选）。
5. **后台 job+状态 UI**：最薄后台任务（非同步轮询阻塞对话）、进度/失败状态公布、版本列表（原 MIDI+源谱+请求+replay 留档）。
6. **最小引擎托管**：按需启动/就绪/失败提示，只管理自身进程，零手工服务（用户不敲命令/不设环境变量）——沿 Godot/VitShell 用户入口裁定。
7. 单测+定向集成测；真实栈+真实生成烟测腿（DG1 口径候 371 定义）**候栈位排队**（当前 367 占栈）。

## 非目标

- 生成 VST3 产品线复活；多引擎方言扩展；防作弊证据（后置）。

## 文件域

`agent/internal/`（gen/genapi/harness 接线，领取时锚定精确清单）+ webui 生成状态面（如需）+ 烟测脚本扩展 + coord 工件。禁入：chat/launcher/project（367 域）、webui/styles/launcher.css、谱面/音源域（369/360 域）。

### 领取时锚定的精确文件清单（2026-11-15 勘察）

- 新增 `agent/internal/genapi/yue_abc.go`：MIDI→YuE2 ABC 方言确定性转换（notes+拍号/速度/调号→dual voices+段注释方言谱）+ ABC 解析回读（往返测试基底）
- 新增 `agent/internal/genapi/yue_abc_test.go`：往返单测
- 新增 `agent/internal/genapi/yue_prepare.go`：yue_local 请求准备面（prompt→style 确定性映射、instrumental 谱面声部休止化展开、ExtraParams 核心字段卫生保护、实发摘要构建）
- 新增 `agent/internal/genapi/yue_prepare_test.go`
- 新增 `agent/internal/genapi/engine.go` + `engine_test.go`：最小引擎托管（按需启动/就绪探测/失败分类/只管理自身进程）
- 修改 `agent/internal/genapi/dialects.go`：yueRequestBuilder 卫生段（核心字段保护，cot/steps 等引擎旋钮仍可 extra_params 覆写）+ Limitations 同步
- 修改 `agent/internal/genapi/config.go`：ProviderConfig 增 engine 托管配置块（engine_command/args/ready_timeout/env）
- 修改 `agent/internal/harness/gen_generate.go`：yue_local 路径接 yue_prepare（映射+纯器乐+摘要）+ 引擎 ensure
- 新增 `agent/internal/harness/gen_jobs.go` + `gen_jobs_test.go`：最薄后台 job store（gen.jobs.start/status/list；版本留档=原 MIDI+源谱+请求+replay+音频路径）
- 修改 `agent/internal/tools/catalog.go`：gen.jobs.* 三命令行级注册
- 烟测脚本扩展（候栈腿定义，不跑）：`scripts/` 现有脚本体系下新增或扩展（真栈腿前按 371 DG 口径冻结）
- coord 工件：`coord/runs/GEN-YUE-WIRE-1/`（如产生）
- webui 生成状态面：**本卡不做 webui 改动**——状态公布走 gen.jobs.* 工具回执（agent 对话面），渲染面边界在回执声明（卡面"如需"按最小面裁定；App.tsx 大文件+367 在飞不引入 webui 风险）

## 验收标准

六项逐项证据（单测/集成/真实生成 run 工件）；三门不回退；用户听核节点如实登记（人耳）。

## 停止条件

YuE2 引擎本地托管与本机环境冲突且不可按需管理→取证上交（引擎侧人工腿候用户定向）。

## 领取

- 2026-11-15 深夜 / origin/main=3997e11b（领取 fetch 后亲核；卡在 todo/、派发已确认、依赖=无）
- owner=GLM-5.3 L2 执行子代理（ZCode 执行会话，GEN-YUE-WIRE-1 专项；PC）
- 分支 port/gen-yue-wire-1 / worktree D:/Vit_DAW_wt_yue1；协调分支 coord/yue1-claim（领取提交见 main log coord(372)）
- 领取基线：worktree 自 3997e11b 新建，HEAD=3997e11b，status 全净（mv 前），diff --stat 空
- 文件域核对：doing 卡 369（wt_369coord，谱库 spike）/370（legame 契约）/371（单栈口径）/367（launcher 零摩擦，chat 域候 271）/273/309/ARRLOOP——与本卡 gen 域（genapi/harness gen 面/tools catalog 行级）不相交；chat/ 禁入已确认（GEN-CHAT-ROUTING-1 域外锚点不触）
- 栈位核对：PC-RUNTIME-STACK 顶部=SHELL-5 22:40 释放后无占用行（origin/main 亲读）；主管派发口径=367 占栈/候栈排队——本卡零栈段先行（单测+定向集成测），真栈+真实生成烟测腿**候栈第 2 位**（第 1 位=367 烟测腿；YuE2 引擎进程按 278 先例自管启停不入栈记录）
- LLM/计费：本卡零 LLM 腿（确定性转换+死端点 override 面）；真实生成腿为本地 GPU 零计费（278 先例）

## 回执

（待回填）

## 验收

（待回填）
