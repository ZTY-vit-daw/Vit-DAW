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

## 回执（勘察腿，2026-11-16 晚）

- **勘察结论一句话**：agent「用音源」链（发现→装载→参数读写）已实证可用，但「选预设调用」的**运行时通道内核与 agent 双侧均为零**（298 实证 VST3 program change 不在参数面+内核 ZMQ 零 preset 命令亲读）；预设仅盘面可枚举（Surge XT 出厂 637 .fxp/17 分类）+模板 state 固化路径可落地；**不依赖内核的先行满足路径存在**——乐器选择优先（本机 17 件 VSTi 含 Grand Rhapsody 真钢琴/Electric88，「用钢琴」=直接装载即得）+盘面预设索引+模板 state 换血。运行时预设腿确需内核命令=**在池卡 291 ARR-PRESET-1 正是此事**（258 报告有完整命令面草案）→本卡不扩域，建议主管将 291 提序衔接。
- **报告路径**：`coord/runs/SRC-PRESET-AGENT-1/RECON.md` @ 分支 `port/src-preset-agent-1`（commit `fd74896a`，已推远端）——四节全锚点：①预设发现面（pluginprobe 观察专用/内核 op 清单/298 工件 0/776 复核/盘面清单/state 编码可逆性实验）②语义匹配现状（无 GM 表/无匹配链/装载意图门不认乐器语义）③322/323 typed 蓝本摘要 ④八步差距表+实现腿拆分建议 A-E。
- **实现腿建议**：腿 A（零栈 agent 域）=GM 名表+乐器→候选规则+Surge 盘面预设索引读取器+fail-honest 降级；腿 B（=291 卡，内核域，不在本卡）=`plugin_load_preset`；腿 C（真栈，依赖 A+B 或 A+state 固化路径）=乐器语义意图路由→枚举/匹配日志→typed 写入落轨（治理确认卡+读回核对，310 红线）→出声探针非静音；腿 D=模板重做（旋律乐器语义装配+鼓 groove 真打击音色，GM 鼓采样音源本机无=诚实边界候裁定）；腿 E（可选零栈取证）=state 字母表唯一性+fxp↔state 转换可行性（成功则腿 D 免栈采集）。
- **零栈零 LLM 零计费亲证**：未登记栈位、未启动进程；只读勘察（代码+卡卷+盘面只读文件）；生产代码零改动（diff=本报告+卡面）；无 run ID。
- 领取协调提交=main `5c9f98c5`；勘察工件分支=`port/src-preset-agent-1` @ `fd74896a`。

## 回执二（腿 A+E，2026-11-16 晚窗；主管验收 pass 后授权，范围=授权四项）

- **实现 commit**：`b1e41ba3` @ `port/src-preset-agent-1`（已推远端）；worktree 续用保留候验。
- **腿 A（范围 1/2/4）**：新包 `agent/internal/instruments`（=主管授权「等价语义面」，按勘察报告腿 A 建议形态；processorintent 未动）——
  ① GM Level 1 官方 128 音色表（Wikipedia/MMA GM 1 Sound Set 逐条核对；1-128 显示口径，**82=Lead 2 (sawtooth) 与 373 manifest 配对亲证**；含 ProgramChange() 换算）+打击键位表 35-81（36/38/42 与模板 percussion_keys 对齐亲证）；② 中英乐器词→乐器类有序规则（bass drum→drums 先于 bass、电钢琴→epiano 先于钢琴等序敏感 case 有单测钉死）+专用音源规则（钢琴→Grand Rhapsody、电钢→Electric88/200/Grand80、Clavinet、Bass Fingers/Slapper；Surge XT 为 synth_preset_library 候选且显式排除 Surge XT Effects）；③ Surge 盘面预设只读索引（一级分类 .fxp 枚举+CcnK/FPCh 头校验+无效文件如实标记；分类→乐器语义，Keys 按名细分 EP/organ/harpsi/suitcase/keys_misc；嵌套目录=教程内容如实排除）；④ fail-honest 降级四态码（no_instrument_class/no_installed_candidate/surge_factory_index_unavailable/installed_instrument_list_empty），打击/铜管/木管预设=Approximate 标记+诚实注记（GM 键位语义不成立等），无候选不硬造。
- **腿 A 真机证据（只读零栈）**：env 门控冒烟 `VIT_INSTRUMENTS_REAL_SMOKE=1`（默认 skip）——出厂库 **624 个 .fxp/16 分类全枚举 invalid=0**（勘察腿 637=含 Tutorials/Formula Modulator 嵌套 13 文件的总文件数，本腿如实修正）；17 件 VSTi 语义索引入参；「这曲用钢琴→Grand Rhapsody（dedicated）」「电钢琴→3 专用+Surge Keys(EP)」「贝斯→2 专用+Basses:59」「鼓→Surge Percussion:9 近似」「弦乐→诚实降级」「gm:82→Leads:131」全链日志在卷。
- **测试与三门**：新包 **26 测试全 PASS**（GM 表完整性/manifest 配对/词序/降级分支/索引 fixture=t.TempDir 不依赖真机）；`go build ./...` exit 0；`go test ./...` **全包 exit 0 两轮**（第二轮=终代码）；webui 未触碰（npm 面不适用，如实声明边界）。
- **腿 E（范围 3）**：报告 `coord/runs/SRC-PRESET-AGENT-1/LEG-E-STATE-FORENSICS.md`（同 commit）——**外层编码已解（铁证）**：state=`<size>.<JUCE 自定义 base64>`，字母表 `.ABC…z0-9+`（**'.'=值 0**；内核自身 JUCE 子模块源码+真实工件回环逐字相同双重证明；**纠正勘察腿「三候选替换」误读**——任意双射都保长度，本腿以源码定论）；**tracktion 层已解**：state=getStateInformation 原文逐字 base64、还原直喂 setStateInformation（fork @60ab1a49 源码亲读，零包装零加密）；**插件契约层已解**：fxp chunk 体≡Surge `save_patch` 序列化（上游源码 loadPatchByPath 同一 loadRaw，'sub3' 格式）——fxp→state=剥头即得（机械定义）；**内容实证层一处未解**：309 工件（Nov-09 内核保存）解码=高熵二进制（`58 d9 0c` 三字节跨插件同头+长度区+~69B 共享段+载荷），与现行内核源码预期（XML 包裹明文）不符——疑 11-09 构建与现行子模块源码版本差；**离线合成管道逐段定义但端到端未证，腿 D 前须 10 分钟栈时解闸**（现行内核一次 save 对比归层；若仍二进制则与内核域相关，**建议与 doing 卡 378 KERNEL-SAVE-FORENSICS-1 的 D1-D3 锚点交叉**——其领取提交 3bfaad42 恰在本回执分支建点前）。
- **零栈零 LLM 零计费**：腿 A/E 全程未登记栈位、未启动 VitApp/Godot/agent 进程；取证=只读源码/仓内工件/盘面只读文件；无 run ID。生产域外零改动（diff=新包+腿 E 报告，11 文件 +1618 行）。
- 领取协调提交=main `5c9f98c5`；勘察=`fd74896a`；腿 A+E=`b1e41ba3`（均 @ `port/src-preset-agent-1`）。

## 验收：待回填
