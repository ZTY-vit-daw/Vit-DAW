# GEN-CTX-DISCLOSE-1：工程音乐上下文披露键（统一化实施①）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=GEN-UNIFY-CTX-DESIGN-1 验收 pass 的实施拆卡规格件五 卡1；用户三线同步+生成≠MIDI 口径）
- 派发确认：已确认（用户 2026-10-11 同步裁定）
- 验收负责人：GLM 主管决策流
- 池序 75；目标仓库=D:\Vit_DAW；**规格全文=[设计件五 卡1](../../runs/GEN-UNIFY-CTX-DESIGN-1/05-impl-card-specs.md)（权威，本卡为池入口）**
- 归属：Mac 生成线（统一化脊柱落地）；P1 / 0.5 天 / 无依赖
- 模型分级：L1 / Mac 执行流（内核 C++ 披露块+冒测脚本）

## 要点摘录（以规格件为准）

get_project_state 新增 music_context 六键（tempo/拍号族，key_signature/style_note 占位 null）；文件域=CommandDispatcher.cpp 披露块（:3389-3420 区域，领取时实测）+scripts 冒测；验收=真栈只读冒测（set_tempo 联动断言）+既有键零变更快照 diff+agent 侧零 diff。

## 并行与资源

CommandDispatcher.cpp 与任何在飞卡（如 PC 混音线内核腿）叠域时按 PROTOCOL §3 核对；Mac 栈按 MAC-RUNTIME-STACK 登记。

- 领取：2026-10-11 21:42 +0800 / origin/main=dba5c516 / GLM-5.3 Mac 执行流（ZCode 会话）/ port/gen-ctx-disclose-1 / worktree /Users/timozty/Documents/Vit-DAW-ctxdisclose1（coord 面 /Users/timozty/Documents/Vit-DAW-ctxd1-coord）/ 领取提交=dba5c516（worktree 基线即领取提交，领取时 git status --short 空、diff --stat 空）
- 回执（2026-10-12 09:5x +0800 自验完成，待主管验收）：
  - **实现 commit**：`dd08da4a` @ `port/gen-ctx-disclose-1`（已推远端）。diff=CommandDispatcher.cpp +32 行（匿名 namespace 内 `appendMusicContextDisclosure` 前向声明 :82+定义（namespace 闭合 :2100 前）+handleGetProjectState `scope` 行后调用）+`scripts/music_context_disclosure_smoke_mac.sh`（新冒测）+`scripts/SMOKE_TESTS.md`（登记条目）+`coord/runs/GEN-CTX-DISCLOSE-1/`（run 工件+探针迭代史）。**零越域**：agent/ 零触碰。
  - **锚点亲验**：领取时（worktree=dba5c516）实测 `handleGetProjectState` 函数头=:3291、披露块 `status=ok`=:3389、`tracks`=:3420、`appendGraphRevisionProperties` 调用=:3447——与设计件二卡面锚点记录逐条一致，停止条件不触发。
  - **披露形态**（权威 run `zmq/B_open.json`）：`music_context={tempo_bpm:128.0, tempo_setting_count:1, time_signature:"4/4", time_signature_numerator:4, time_signature_denominator:4, time_sig_setting_count:1, key_signature:null, style_note:""}`——与设计件二 §1.1 逐键一致（juce double 序列化保留 .0；key_signature null=juce void var）。
  - **真栈冒测**：`scripts/music_context_disclosure_smoke_mac.sh`（**内核直连 ZMQ REQ 5555 两阶段**；探针面裁定：agent 用户可见 get_project_state 工具是策划投影会过滤内核键（catalog.go project.state spec），本卡零 agent diff，断言面=内核回复本体——件二 §2.5 消费方（kernel.SendCommand）同面）。权威轮 **run1-pass-20261012-094027 exit 0，24/24 门绿**（schema `music.context.disclosure.mac.v1`）。验收三条对应：①六键+联动=phaseA set_tempo 128→auto-persist→phaseB open 披露 128.0；phaseB set 96→回显 96.0→再查 96.0（读写同 getTempo(0) 引擎面）；②既有键零变更=两内核同文件同路径 A_open/B_open 快照 diff 剥 music_context 后**逐键相等 diff_count=0**，键集 delta=∅（keyset_delta_exact），唯一规范化易变路径 `$.plugin_list_hygiene.completed_at`（内核启动墙钟戳，TIM-KERNEL-HYGIENE-1 语义，verdict 内记录）；③agent 侧零 diff=go build ./... exit 0+gofmt 净+`go test ./... -count=1` 92 包 0 FAIL+git diff 范围核对（仅上述三文件域）。
  - **构建**：候选内核=worktree 增量构建 sha256 `8e445dbb…`（改前纯态构建 0 error 1063 警告行→改后 0 error、CommandDispatcher.cpp 警告 46=基线，零警告增量）；基线内核=GEN-MAC-BASELINE-1 二进制 `9a70f064…`（其源 32fcab06 与领取基线 dba5c516 的 VitApp/Source 零 diff 已 git diff 核证）。
  - **迭代史**（AGENTS §8 分类，全脚本缺陷/环境中断非栈失败，工件 probe-iteration-history/）：run1=内核 Tracktion 设备管理器初始化挂起（环境中断，PORT-SMOKE-MAC-4 先例，重试过）；run2=探针面选错（agent 投影层过滤→改直连 ZMQ）；run3=三处门断言 bash 3.2 兼容缺陷+易变时间戳未规范化。
  - **栈登记**：排队→占用 b48e4549（与 GEN-API-ADAPTER-1 竞争登记，按协议排队，其释放后连续三轮端口空闲复核才占用）→释放 e40786d9（用毕复核四端口监听=0、进程=0）。
  - **端测边界声明（AGENTS §5）**：本卡纯内核披露面+脚本，零 webui/零用户旅程改动；真栈冒测=内核 ZMQ 面读断言（exit 0）。生成端到端归卡 4/AIGC-DRUM-GEN；agent 用户可见面接入归卡 3。
- 验收：pass（2026-10-12）——[rulings/2026-10-12-GEN-CTX-DISCLOSE-1-pass.md](../../rulings/2026-10-12-GEN-CTX-DISCLOSE-1-pass.md)；决策侧复验=内核 diff 亲读/独立复跑 exit 0/全量 0 FAIL（详裁定文件）；合入=cherry-pick 实现提交；验收提交随 accept 链合 main
