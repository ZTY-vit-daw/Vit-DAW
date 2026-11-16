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

## 回执（勘察腿，2026-11-16 晚；权威面=coord/cards/doing/ 本卡 @ main 0fe0d12c）

- **勘察结论一句话**：agent「用音源」链已实证可用，但「选预设调用」的**运行时通道内核与 agent 双侧均为零**（298 实证+内核 ZMQ 零 preset 命令亲读）；预设仅盘面可枚举（Surge XT 637 .fxp/17 分类）+模板 state 固化可落地；**不依赖内核的先行路径**=乐器选择优先（本机 17 VSTi 含 Grand Rhapsody 真钢琴）+盘面预设索引+模板 state 换血；运行时预设腿=**在池卡 291 ARR-PRESET-1 正是此事**（258 报告有命令面草案）→不扩域，建议 291 提序衔接。
- **报告**：`coord/runs/SRC-PRESET-AGENT-1/RECON.md` @ `port/src-preset-agent-1`（`fd74896a` 已推远端）——四节锚点：①预设发现面 ②语义匹配现状 ③322/323 typed 蓝本 ④八步差距表+实现腿 A-E 建议。
- **零栈零 LLM 零计费**；领取=main `5c9f98c5`；生产代码零改动。

## 回执二（腿 A+E，2026-11-16 晚；权威面=doing 卡 @ main ef1e4d31）

- **实现**：`b1e41ba3` @ `port/src-preset-agent-1`（已推）——腿 A=新包 `agent/internal/instruments`（GM 官方 128 表[82=Lead 2 与 manifest 对齐]+打击键位 35-81+中英乐器词规则+专用音源规则+Surge 盘面预设只读索引[624 fxp/16 分类，CcnK/FPCh 校验]+fail-honest 降级四态）；26 测试全 PASS；三门 `go build`/`go test ./...` 全包 exit 0 两轮；真机只读冒烟证据在卷（钢琴→Grand Rhapsody 等全链）。
- **腿 E**：报告 `coord/runs/SRC-PRESET-AGENT-1/LEG-E-STATE-FORENSICS.md`——外层编码铁证（JUCE 字母表 '.'=0，源码+回环双证，纠正勘察腿三候选误读）；tracktion 层零包装；fxp chunk≡Surge save_patch（离线转换=剥头）；**内容实证层一处未解**（309 工件=高熵二进制 vs 现行源码预期 XML 包裹）→腿 D 前须 10 分钟栈时解闸，建议与 doing 卡 378 KERNEL-SAVE-FORENSICS-1 锚点交叉。
- 零栈零 LLM 零计费；worktree 保留候验。

## 回执三（腿 D 闸门，2026-11-16 21:10；权威面=doing 卡 @ main d1ad257b）

- **判定 `binary_309_identical`**：现行内核（a5947361 复用）实测 state 与 309 工件头 32B 逐字相同+init patch 全等（67294B sha f245cce2）=高熵二进制本征形态，版本差假说证伪；**腿 D 走内核亲存 state 路径（309 先例），fxp→state 离线合成不可行**。
- attempt3 PASS（776 参=298 全等；VIT_DUMP_CLEAR_XML sidecar 通道）；attempt1/2=save 形态装置域两轮；栈占用 8 分钟（占用 1e2b5db5/释放 c62aa928）；TEARDOWN_ALL_ZERO+recents 还原 807226F4AF3F 全亲证；LLM 上界 0。
- 附加取证：Global Volume 写后面 state 不变（疑 patch 外参数）——腿 D 预设差分须选 patch 内参数。
- 工件 `coord/runs/SRC-PRESET-AGENT-1/legd_gate_20261116_2115/` @ `port/src-preset-agent-1` `0c354d57`；腿 E 报告 §4 已随判定重写。

## 回执四（腿 D，2026-11-16 22:50；权威面=doing 卡 @ main b36dcc3a）

- **382 上交项收口**：拍数修正双向（builder 秒值换算+coursetemplate 秒→拍解析换算=Go 端新绊线）；真栈 A2 全绿 6.0/6.0/12.0s=8/8/16 拍。
- **音色换血**：旋律/主题 soft-keys（10 patch 内参）+鼓 percussion（11 参）+groove 重写（+GM46 open-hat，21 音）；state 真栈采集固化（互异+异 init 亲证）；manifest 声明=soft_key_approximation_declaration（GM 1 缺省；会响钢琴本机无〔309 Waves RT-SILENT〕如实声明，候 M2 用户复听）。
- **VERIFY_PASS 12/12 exit 0**：三轨离线探针非静音 0.151/0.140/0.156≫0.01；零产品断言失败。
- 栈：占用 f230dc88/释放 48f9d417（两段 boot 一次登记）；TEARDOWN_ALL_ZERO+recents 807226F4AF3F 亲证；LLM 上界 0。三门全绿（build+全量 test exit 0）。
- 实现 `47ccf552` @ `port/src-preset-agent-1`（builder 重写+go 换算+模板/manifest/states+run 工件）。

## 验收：待回填
