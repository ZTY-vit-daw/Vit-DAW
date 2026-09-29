# PAPER-EXP-ADAPT-1 执行报告（blocked 上交）

- 执行侧：PC 执行会话（GLM-5.3，L1 真栈操作）
- 领取：2026-09-29 19:30 / origin/main 92d53b07（卡面记录值；提交时主树已被决策侧推进至 0b86e56d——GUARD-1 done+手测节点报告，coord-only，与代码无关）/ 分支 `port/paper-exp-adapt-1`（worktree `D:/Vit_DAW_worktrees/paper-exp-adapt-1`，基于 tag `experiment-baseline-2026-09-29`=4b695354）
- 结论先行：**目标 1/2/3 完成；目标 4（PILOT 2 例 exit 0）被 runner 材料合格门阻断——七例中六例（含 p01+p07 两张 PILOT 例）被拒，真栈两轮 exit 1 实证在案**。证据推翻卡面前提"材料侧适配清单即可过 runner"，上交决策侧处置。runner 断言零改动、内核零改动、素材零改动。

## 0. 真栈泊位声明（AGENTS §9）

- 本会话在 2026-09-29 19:31–19:45 独占真实栈：VitApp 内核（ZMQ 5555/5556，构建后 pid 7192）+ Go agent（HTTP 7878）+ Godot 前端（`D:\Godot\project\vit-daw-frontend`，pid 33368）。起栈前核查：5555/7878 空闲、无 VitApp/Godot/VitAgent 残留进程。
- 在飞卡 L1-3-GUARD-1（领取时在飞、本卡执行中已被决策侧验收 done）为纯单测卡，全程零占栈——冲突面为零，与开工指令预判一致。
- 测试时 HEAD=e8955ed7（main，代码与 tag `experiment-baseline-2026-09-29` 等价，其间 coord-only 提交）；工作树仅既有运行时残留 `M VitApp/Workspace/default_project.xml`（历史卡已记录为非卡域）。
- VitApp.exe：2026-09-29 19:37:42 构建（cmake Release，PILOT 两轮均未用 -SkipBuild，ps1 内置重建为增量 no-op），SHA256 `78DC4889921204472DDCA365FFA1915287EF8C5D69BD7BA8C91C68463AD0631F`，30922752 B。
- 收栈状态：栈仍在本会话泊位（PILOT p07 轮后未再动）；authority 已实证恢复 manual_confirmation。

## 1. 目标 1：pv1 七例 → .vit 工程（✅ 完成）

- 隔离工作区：`C:\Users\timoz\Documents\毕业设计\experiments\out\suite_v1_lm_runner\`（材料仓 commit `258b1ba`）。cases/ 为源树 42 stems 的字节级只读副本（sha256 逐轨核对并记录）；源树与 sealed/ 零写入（§10）。
- 构建链（spv1 同款、runner 消费面一致）：`scripts/semantic_processor_project_smoke_projects.py` → `project.new → project.clear → project.save_as → project.import_folder_as_stems → DAD 分析等待 → project.save → project.state 校验`，经 agent `/agent/invoke` 驱动内核。**卡面锚点 `project.import_audio_files` 是同族文件清单形态；实际采用 folder 形态 `project.import_folder_as_stems`**（同在内核 canonical 命令表，spv1 工程即此链建成，`project_build_report.json` 在案）——语义等价（内核导入面建 stems 工程），偏离仅命令表面形式，特此声明。
- 结果：七例全 passed——各 6 轨（bass/drums/guitar/other/piano/vocals）、DAD ready 6/6、waveform envelopes 6/6、product fine evidence（noise_floor/frequency_time/transient/band_dynamics 四字段）passed。工程 ~23KB/例。报告：`fixture-build/20260929T1939/project_build_report.json`。

## 2. 目标 2：适配版公开清单（✅ 完成，已入库）

- 位置：材料仓 `experiments/out/suite_v1_lm_runner/fixture_manifest.json`（commit `258b1ba`）。
- 形态：schema_version=`semantic_processor_agent_project_smoke_public_manifest.v1`；六键盲法契约全真；七例（p08 按用户 2026-09-18 裁定未收录）各带绝对 `project_path`、`stem_files` 为 `{track,file,sha256,size_bytes}` 对象数组；`adaptation_notes` 记录源清单 sha256（`75e54939…`）与四项适配内容。
- 验证：runner 自身 `load_public_case` 对七例全过（schema/盲法/单例匹配/project_path 存在性/无 sealed 路径段）。PILOT 两轮真栈实跑进一步证明清单门与工程物化（copytree）链路全通——**唯一失败点在材料合格门**（见 §4）。

## 3. 目标 3：full access 脚本化路径（✅ 确认成立，含一处语义约束）

真栈六步实证（`full-access-probe/20260929T1941/full_access_probe_log.json`，journey 冒测同款驱动）：

| 步 | 结果 |
|---|---|
| a. GET /agent/authority 基线 | manual_confirmation（默认） |
| b. POST authority_mode=full_project_access | 回显 full_project_access ✓ |
| b2. 同身份 invoke 后 GET | 保持 full ✓ |
| c. runner 形态 project.open（副本 copytree 新路径）后 GET | **重置回 manual_confirmation**——AUTHORITY-LOST-1 显式闩跨工程身份边界丢弃（`agent/internal/chat/server.go` ~6826："project A's full access cannot leak into project B"） |
| d. 开工程后（同身份）POST full | 切换成功且跨 invoke 持久 ✓ |
| e. POST manual_confirmation 恢复 | ✓（栈交还默认态） |

**正式轮（EXP-VIT-MAIN）脚本化序列**（现行代码即可，零开发）：①物化案例副本（copytree，runner 同款）→ ②`project.open` 该副本 → ③`POST /agent/authority {"authority_mode":"full_project_access"}`（同身份、无在飞 turn；turn 运行中切换会被 409 拒）→ ④`python scripts/free_state_d1_smoke.py --public-manifest <清单> --public-case-id <id> --project-workdir <①目录> --reuse-existing-project ...`——runner 见工程已绑定即跳过重开（`free_state_d1_smoke.py` prepare_project 的 reuse_live_binding 分支），权限保持 full access 跑完整轮。注意 ps1 包装器每次 `-RestartAgent` 会换栈实例，正式轮须按上述 py 直跑形态或 wrapper 化（决策侧在 EXP-VIT-MAIN 卡定夺）。切换会持久化进该工程工作区，跨 agent 重启由 restore 采用盘上态（`server.go` ~7147）。

## 4. 目标 4：PILOT（❌ 阻断——材料合格门拒 pv1，证据上交）

### 4.1 失败面（离线复算=runner 自身函数，权威口径；`offline-qualification/20260929T2010/all7_qualify_results.json`）

| 案例 | 判定 | 被拒门与明细 |
|---|---|---|
| pv1_p01 | **FAIL** | RMS 地板：guitar −52.2 dBFS（≤−45） |
| pv1_p02 | FAIL | RMS：guitar −90.7、other −49.1 |
| pv1_p03 | FAIL | RMS：other −89.6、piano −89.8；crest：bass 8.6 dB（<10） |
| pv1_p04 | FAIL | RMS：guitar −50.7、other −51.9、piano −53.5 |
| pv1_p05 | PASS | best crest 24.8 dB |
| pv1_p06 | FAIL | 瞬态对比门：vocals weak contrast（`weak_contrast_tracks`） |
| pv1_p07 | **FAIL** | RMS：guitar −65.1 dBFS |

（p08 已剔除不计；其 bass crest 9.4 同样不过。）

### 4.2 真栈实证（AGENTS §8/§9 口径）

| run | 命令 | 退出码 | 结果 |
|---|---|---|---|
| 20260929_194221 | `powershell -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot D:/Vit_DAW -PublicManifest <适配清单> -PublicCaseId pv1_p01 -PromptFlavor neutral` | **1** | `D1-S1 FAIL: public material failed the compression-fixture qualification gates: {"weak_rms_tracks":["guitar"],...,"best_crest_db":25.355}`；responses=[]（LLM 前即断，零概率性成分，属确定性门拒绝） |
| 20260929_194244 | 同上，`pv1_p07` | **1** | 同门同因（guitar；best_crest 27.192） |

工件：`vit-pilot/20260929T1942_{p01,p07}/`（d1_smoke_report.json + agent_last.log + stdout_tail + 本表命令/退出码）。原始目录 `D:\Vit_DAW\artifacts\free_state_d1_s1\{20260929_194221,20260929_194244}\` 未覆盖。

### 4.3 归因（不改断言、不改素材的前提下穷尽核对过的路径）

- 门本体：`free_state_d1_smoke.py` `qualify_material`（~L234-259）**对所有 flavor 无条件执行**（RMS −45 地板/逐轨 crest 10/最好轨 14 + sibilance/transient 子门），为 D2-1.5-S2f-2 压缩正向 fixture 设立，注释自述按 **spv1 合成材料**实测校准（crest 14.5–21.0 dB）。
- pv1 是**真实音乐 stems**（MUSDB18 型六轨拆分、20s 节选）：guitar/other/piano 在节选窗内本就是安静轨甚至近静音轨（−50～−90 dBFS），这是真实混音形态而非素材缺陷；等响度版（BS.1770 LUFS Δ=0）由设计决定，不能靠抬增益"修"素材（改素材=改实验输入包，56b §6 纪律 2 定稿后不改）。
- 无绕行面：无 skip 旗标；门读 stems 本体，清单侧无合法表达可避（裁掉安静轨=破坏六轨对等与盲法，不采）；ps1/py 参数族穷举核对无门相关开关。
- 结论：**"不动 runner 即可 PILOT exit 0"的卡面前提被证伪**。卡面停止条件（内核导入面缺口）未触发——导入面完好；阻断在 runner 材料门与真实素材的结构性失配。

### 4.4 上交决策侧的处置选项（不代决策）

1. **开 runner 门实验适配卡**（建议方向）：门是 D2-1.5-S2f-2 为压缩 fixture 设的**素材合格前置**，非 D1 契约本体；对 pv1 真实材料需重校准地板（按 pv1 实测分布）或将该门改为 fixture-set 作用域（如 manifest 声明 qualification profile）。属 runner 断言变更，本卡纪律禁止，须决策侧立卡。
2. PILOT 例替换为 p05（唯一过门例）——不满足卡面 p01（问题例）+p07（控制例）配对设计，且 p05 单例无控制对照，仅作记录。
3. 改素材口径——与 56b 定稿纪律冲突，仅作记录。

## 5. 版本与可回指锚

- 代码：tag `experiment-baseline-2026-09-29`=4b695354（worktree）；运行树 main e8955ed7（代码等价）；VitApp.exe sha256 见 §0。
- 材料仓：`experiments/out/suite_v1_lm_runner/` commit `258b1ba`（清单+七工程+构建报告+README+stem_copy_log；wav 副本被 ignore 规则覆盖不入库，sha256 可校验重制）。
- 探针副本工程：`D:\Vit_DAW\temp\paper-exp-adapt-1-probe\`（full access 跨身份重置实证用）。
- 本目录：fixture-build / full-access-probe / offline-qualification / vit-pilot 四组工件。
