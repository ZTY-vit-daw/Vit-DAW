# YUE-INSTRUMENTAL-PATH-1：接入官方 instrumental 渲染路径验证（381 后续·用户查证指令）

- 发卡：主管 2026-11-16 晚（用户指令「查 YuE 文档确认是否有纯器乐参数」——查证结果推翻 381 的「引擎能力边界」终态：**官方有 instrumental 工作流，差距在本地封装未接入**）
- 池序 390；P1；依赖=372 引擎托管链+381 取证卷（本地引擎六字段歌曲模式通道已锚定）；真栈腿（引擎拉起）候栈位
- 官方依据：[YuE repo](https://github.com/multimodal-art-projection/YuE) `skills/yue2-music/instrumental/SKILL.md`——Vocal→Ins 音符迁移（Vocal 只留休止+和弦）+`scripts/instrumental.py run` 专用脚本+推理配置（m-a-p/YuE2-Vae、默认采样、BF16 AR/NAR、FP32 VAE、32 步）。

## 目标

1. **勘察腿（零栈）**：定位本地 YuE2 引擎封装（agent 托管的 engine supervisor 链，372 C7 域）当前调用的渲染入口与配置；对照官方 instrumental.py 的调用差异（渲染配置/LoRA 面/VAE/是否单声部直入）。
2. **接入腿**：本地封装加 instrumental 渲染路径（最小档：新 dialect/参数分支走官方等价配置；agent 侧 `Instrumental` 请求不再降级为 style 英文词，而是路由到该分支）；复用现有谱面（Vocal 休止+Ins 单旋律=官方形态，381 已证）。
3. **验证腿（真栈，1-2 次生成）**：同一学生谱跑 instrumental 路径生成 → WAV 落 run 工件；听核节点=**用户人耳判零人声**（372 W9 转写检测可作旁证，不作替代）。
4. 产品面结论：接入成功→展演生成恢复纯器乐能力（381 的「能力边界披露」口径升级为「已接通」）；本地 checkpoint 不支持/仍出人声→如实回退「带人声+披露」裁定（用户已预表态接受该 fallback）。

## 边界

- 引擎=本地部署 YuE2（外部模型），不硬改模型本体；接入面=我们的封装/调用层。
- 失败分类照 §8：路径跑通但仍有零星人声≠失败（按 W9 边界口径记录）；成段人声=未接通。
- 真栈腿与 377/378 栈协调（一次一 owner）。

## 文件域

`agent/internal/genapi`（engine supervisor/provider 链，领取时锚定）+引擎启动配置+`coord/runs/YUE-INSTRUMENTAL-PATH-1/` 工件。

## 验收

勘察差异清单+接入 diff+真栈生成 run 工件（WAV+配置留档）+用户听核登记；三门不回退。

## 领取/回执

- **勘察腿（零栈）执行完成**（2026-11-16，量产执行子代理，worktree `D:\Vit_DAW_wt_instr` 分支 `port/yue-instrumental-path-1`，领取基线 HEAD `ebfee4270`，commit `ce2a4f5f` 已 push）。
- **结论一句话**：本地引擎（yue2.cpp `yue-server.exe`）的 /synth 请求面已能表达官方 instrumental 配置的全部可控字段（style 规范化/歌词曲式标签/cot 推导），差距纯在我们封装层的条件构造（yue_prepare.go Instrumental 分支现状=英文词+谱面休止），**本地 checkpoint 不缺器乐件**（官方 instrumental 与歌曲模式共用同一对模型 YuE2-3B+YuE2-Vae，官方 repo 全树零 LoRA——卡面「双 LoRA」假设不成立）；停止条件未触发。不可达面如实登记两条：yue2.cpp 引擎内置采样参数、官方 prefix.npy 逐位断言（以引擎 job 回显+replay JSON 作本地等价证据面）。
- **报告**：`coord/runs/YUE-INSTRUMENTAL-PATH-1/RECON.md`（分支 `port/yue-instrumental-path-1`，主仓 main 尚未合入）——三节（本地封装链锚点 / 官方对照 / 差距清单 S1-S7 步骤拆解+接入腿建议）。
- **接入腿建议**（RECON §3d）：先 S5 干跑（本地引擎 abc+lyrics 组合 + cot=melody 接受度，零生成成本）→ S1/S2/S4 实现（style 负条件规范化、曲式标签歌词通道、cot 推导）→ S6 披露收口 → 真栈 1-2 次生成+用户人耳终审。诚实边界：官方负条件在 yue2.cpp 的抑制效果无本地先验（381 反例：英文词 instrumental 无抑制），接入腿跑通≠人声消失，fallback 口径（带人声+披露）保持待命。
- 验收：勘察报告待决策侧验收；接入腿与验证腿候发卡。

## 领取/回执（接入腿·零栈，2026-11-16）

- **接入腿（S1/S2/S4/S6 + 零生成干跑）执行完成**（主管验收勘察腿 pass 后授权；同 worktree `D:\Vit_DAW_wt_instr` 同分支续 commit `ec596f6c` 已 push；领取基线仍为 `ebfee4270` 工作树续用，无新领取基线冲突）。
- **实现面**（文件域 `agent/internal/genapi` + harness 测试，与卡面一致）：
  - S1：`yue_prepare.go` Instrumental 分支 style 规范化=官方 prepare 语义（`Instrumental, ` 前缀（`(?i)^instrumental\b` 已带则不加）+ `no vocals/no singing/no choir/no spoken words` 逐条件查重追加 + StyleExtra 后句号收尾；空 style 取官方默认文案再前缀化）。
  - S2：`yue_abc.go` 新增 `YueSectionTagLyrics`（`% section` → `[Title Case]` 标签串，Python str.title 等价含连字符词）；Instrumental 时 Lyrics 为空则自动填充（非空保留+披露），无 section 则保持空=官方空歌词等价。
  - S4：`yue_abc.go` 新增 `YueVocalHasChords`（Vocal 行引号和弦检测）；官方推导规则落 `GenerationRequest.COT`（新字段）→ builder 消费（默认仍 full，extra_params 仍可覆盖）；转换器形态（和弦骑 Ins）→ melody，官方形态（和弦挂 Vocal 休止行）→ full。
  - S6：`dialects.go` Limitations instrumental 文案 + prepare actions 全部按新行为改写；381 防超卖关键短语断言（`engine-side muting not guaranteed` 在场、`validated instrumental` 禁止）保留为回归防线。
- **零生成干跑**（`TestYueInstrumentalDispatchShapeDryRun`）：完整条件化请求经 `yueRequestBuilder` 构造后断言——style/lyrics/abc/cot 四面齐全、 dispatched keys 全部落在引擎已证接受集（deploy smoke1 + 381 echo + replay 旋钮证据锚点）内、无发明参数；摘要面 `cot/lyrics_chars` 同步。**边界如实**：本腿只证请求构造面；引擎侧 cot=melody 与 abc+lyrics 组合接受度候真栈腿。
- **测试**：`go build ./...` exit 0；`go test ./...` 全量 exit 0（genapi+harness 定向先行全绿）；新增 6 测试（style 边界×6 case、S2/S4 组合、干跑形态、非器乐 full 基线、section 标签、Vocal 和弦检测），更新 2 处授权内期望（prepare instrumental style 官方形态、harness gen_jobs style 断言改大小写不敏感）。
- **忠实性修正 2 项**（实现过程中亲证）：官方空 style 语义=默认文案**仍加** `Instrumental, ` 前缀（`Instrumental, Expressive instrumental music, …`）；官方歌词串带尾换行——builder 由 TrimSpace 发送改为原样发送（摘要同步逐字计数）。
- **诚实边界**：接入跑通≠人声消失（381 反例：英文词 instrumental 无抑制；官方负条件在 yue2.cpp 上效果无本地先验）；fallback 口径（带人声+披露）待命；真栈生成腿+用户听核候栈位。

## 验收：待决策侧

## 2026-11-17 状态归位
依既有POOL-INDEX与gate，已完成零栈两腿、候真栈与用户听核，故todo归位doing。保留原执行owner与分支，本次不自动接管。真栈续跑前另记显式交接。

## 真栈验证腿第一轮（2026-11-20 18:32，lead 亲跑，候用户听核）

- 装置=372 冻结 `run_yue_wire_smoke.ps1`，被测=接入腿分支 `port/yue-instrumental-path-1` @ ec596f6c 自建 agent；RunRoot=`coord/runs/YUE-INSTRUMENTAL-PATH-1/real-stack/run_20261120_1832`（worktree wt_instr）。
- **新配方真栈证成（v1，60.0s 完整渲染）**：dispatched style=`Instrumental, 欢快的钢琴独奏，突出学生旋律, no vocals, no singing, no choir, no spoken words.`；cot=melody；lyrics_chars=0（fixture 无 % section=官方空歌词等价）；W3 对账 ins_note=16/vocal_rest_only=true；W4 双版 style 各含用户词原文且互异；零 LLM。
- v2 job done 但 WAV 14.9s（vs 60s 请求，比例窗 [0.6,1.5] 外）→ driver exit 1=legW8_short_output；v1/v2 条件仅 style 文本异 → 引擎 NAR 长度求解概率行为（381 W9 同概率类），非条件链缺陷；生成预算（2 次）已用尽，不追加重跑。
- W9 转写旁证未达（driver 在 W5 停）；**用户人耳听核=v1 WAV**：`run_20261120_1832/downloads/yue-local-1526daa82c0c8478.wav`（11.5MB, 60s）——判「零人声」即接入成功；成段人声=未接通走 fallback 口径。
- 装置债亲证：recents 被 save_as prepend+default_project 被内核改写（RECENTS-HYGIENE-1 泄漏面/425 债），berth 快照还原哈希亲证；vram 采样器峰值 0 缺陷登记。
- 栈占用/释放：PC-RUNTIME-STACK 2026-11-20 18:32/18:34 两条在卷。
