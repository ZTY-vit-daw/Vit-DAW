# SRC-PRESET-AGENT-1：音源换血——agent 动态「用音源→选预设→调用」能力+模板重做

- 发卡：主管 2026-11-16 晚（373 M2 用户听核音色否决→用户裁定换血；**底线=用户原话：「你得确定 agent 是否会『使用音源，然后找到合适的预设调用进来』，这是使用的底限」**）
- 池序 384；P1；依赖=373 模板已合入（结构复用）、322/323 认证链模式在卷；真栈腿候栈位
- 性质：能力线（agent 动态音源使用）+内容线（模板音色重做）双腿——**不是人肉固定配音色**。

## 目标

1. **勘察腿（零栈）**：现有音源预设发现面——Surge XT（及备选 GM 兼容音源）经 `pluginprobe`/`processorregistry` 能否枚举预设库（名称/分类/Program 语义）；产出现有面差距清单。
2. **匹配链腿**：乐器语义→预设选择规则（GM Program 标准名/教材谱例乐器标注 → 音源预设映射；复用 processorintent/semanticeffect 语义面；匹配不到时 fail-honest 降级文案，不硬造）。
3. **agent 动态调用腿**：轨级对话/乐局语境「这曲用钢琴」→ agent 选定 piano 预设 → **typed 写入链落轨**（复用 322 Pro-Q 3 认证/323 vendor-scope 模式；写入类必须 typed 等价面——310 结论红线）；出声探针非静音。
4. **模板重做腿**：melody-drum-v1 音色换血——旋律轨按谱例乐器语义装配（替换单音 saw stand-in）、鼓轨 groove 重写+真打击音色（GM 键位语义正确化）；manifest 的固化 blob 改为（或叠加）语义音色声明+动态装配路径。
5. **用户复听节点**：模板重听（M2 重测）+「对话点音色→听到换音色」演示。

## 边界

- 内核零改动预期（音源/VST 写入走既有插件链）；若勘察发现预设枚举需内核面→上交锚点不扩域。
- 372 生成线 YuE 引擎音色不在本卡（那是 381 域）；本卡=DAW 轨内音源。
- 「单音固定旋律」内容升级（教材曲例替换）衔接 380 转写产物，若 380 未完则本卡先用现旋律只换音色。

## 文件域

`agent/internal/processorregistry`/`pluginprobe`/`processorintent`/`capabilityadapters`（领取时锚定）+`agent/internal/coursetemplate/`+`coord/lesson-designs/.../templates/`+`coord/runs/SRC-PRESET-AGENT-1/`。

## 验收

底线断言（真栈）：agent 收到乐器语义指令→预设枚举/匹配日志在卷→typed 写入落轨→出声探针非静音→**用户听到音色变化**；模板重听 M2 重测用户 PASS；三门不回退。

## 领取（勘察腿，2026-11-16）

- 领取时间：2026-11-16 19:10（PC）
- 领取时 origin/main：`d5c20765c8edf462416d93a938e4303c3e0c0e5f`
- owner：GLM-5.3（ZCode L2 实现子代理，SRC-PRESET-AGENT-1 执行流）／PC（Windows）
- 分支：`port/src-preset-agent-1`（领取协调提交走 `coord/src-preset-agent-1-claim`）；worktree：`D:/Vit_DAW_wt_presetagent`
- worktree 基线：HEAD=`d5c20765`，`git status --short` 空、`git diff --stat` 空（干净基线，无领取前已有 diff）
- 依赖核对：322 PROQ3-CERTIFY-AND-T7-1（done 已合入）/323 EQ-RECEIPT-VENDOR-EXPAND-1（done 已合入）/373 SRC-TEMPLATE-1（done 已合入）——POOL-INDEX 48-56/172 行亲核
- 本轮范围：勘察腿（零栈，不改生产代码）；真栈腿未登记栈位（PC-RUNTIME-STACK 未占用）

## 回执：待勘察腿完成回填

## 验收：待回填
