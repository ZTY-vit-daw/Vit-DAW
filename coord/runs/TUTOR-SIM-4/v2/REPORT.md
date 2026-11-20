# TUTOR-SIM-4 v2 执行证据（tutorprod438b，2026-11-20）

零栈实现卡（v2）：tutor_sim v1 schema（brief/materials）+ MIDI 材料装载 + webui 材料卡。
worktree `D:/Vit_DAW_wt_pc438b`；分支 `port/tutor-sim-4-438b`（基线 `841b9ef4`）；
实现提交 `950f1695`；main 合并 `51eff149`。

## 1. 命令与退出码（全部亲跑）

| # | 命令（cwd） | 退出码 / 结果 |
|---|---|---|
| 1 | `go build ./...`（agent） | 0 |
| 2 | `go test ./internal/chat -count=1`（agent） | ok（124.936s，全程两次：中途回归 + 收尾） |
| 3 | `go test ./internal/tutorsim -count=1`（agent） | ok |
| 4 | `npm run test`（agent/webui） | 66 文件 / 802 用例全过（435/436/437 既有用例零回归） |
| 5 | `npm run build`（agent/webui，tsc --noEmit + vite build） | 0（chunk>500kB 警告为既有） |
| 6 | `powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_tutor_sim_smoke.ps1 -DryRun -ProjectPath "D:/Vit-DAW-experiments/nonexistent.vit" -Answers "42,模进,已装载,完成"` | 0（计划打印含 answers/wrong_answer） |

不启真实栈（卡面约束）：真栈 smoke 腿与手测由 lead 开窗执行，示例命令见
`coord/experiments/tutor-sim-2/README.md` §4。

## 2. 提交与工作树状态

- 领取基线：`841b9ef433a5c32b641775506ea1f8817c5527e9`（worktree 全净）。
- 实现提交：`950f1695`（20 文件，+2968/−85；`git show --stat` 见分支）。
- 合并：main `51eff149`（merge 950f1695，无冲突；main 侧 438/439 coord 提交与本分支文件域不相交）。
- 本卡 coord 收尾（卡面 done/ROSTER/POOL-INDEX/本卷）另行提交。

## 3. 实现锚点

| 面 | 锚点 |
|---|---|
| schema v1 | `agent/internal/tutorsim/tutorsim.go`：`SchemaVersionV1`、`Step.Brief/Materials`、`Material`（kind=midi_clip/audio，audio 留位）、`validateMaterials` fail-closed、`SummarizeMaterials`（LoadFile 时摘要，失败记录不阻塞）、`Store.ReserveMaterial/FinalizeMaterial/CurrentMaterials/LoadedMaterials` |
| MIDI 解析 | `agent/internal/tutorsim/midi.go`：最小 SMF（format 0/1、metrical division、running status、未闭合 fail-closed）；装置摘要亲证 42 音/48 拍/100 BPM（`TestShippedTutorSim2ParsesWithMaterialSummary`） |
| 装载端点 | `agent/internal/chat/tutor_sim.go`：`handleTutorLoadMaterial`（POST /agent/tutor/load_material）；内核链 `tutorSimLoadMidiMaterial` = add_track→rename_track→insert_midi_clip→add_midi_notes（W1 命令面与状态判定口径）+ best-effort rack_add_node（默认 identifier `VST3-Surge XT-1e1bcfb-190e4fbd`=384 腿 D SRC-TEMPLATE provenance；plugin_path/plugin_identifier 可请求覆盖）；already_loaded 幂等、Reserve 防并发重复、失败留证据 id 可重试；路由注册 `server.go`（+1 行，435 先例同型） |
| workflow_data | start/chat 均带 `brief`（当前步）与 `materials`（含 loaded/loading/instrument_attached 等）；v0 实验 brief=""、materials=[]；`schema_version` 按实验文档回显 |
| webui | `TutorWidgets.tsx`：brief 折叠（默认收起，data-tutor-brief/-open/-text）；`MidiMaterialCard`（摘要/装载按钮/已装载置灰/音源态如实）；`AudioMaterialCard` 复用 `widgets/AudioPlayer`（无 src 禁播降级）。`projectData.ts`：`tutorMaterialsFromValue` fail-closed 校验、`tutorMaterialLoaded` 事件、v0 存储视图保持 437 原形状（严格相等面不回归）。`projectService.ts`：`loadTutorMaterial` live 接缝（lib/api 域外零改动，本地 requestJSON 同型 helper + apiPath 复用）、start 回执 workflow_data 局部扩展（v0 输出不含新键）。`projectFixtures.ts`：fixture 装载回执/台账。`ProjectSurface.tsx`：装载 handler（会话寻址、失败 notice、成功只认服务端回执） |
| 装置 | `coord/experiments/tutor-sim-2/`：v1 四步（听主题 42 音→听辨模进→装载材料→完成确认）；brief 从 level.yaml 七节点语义起草，expect=作者模拟问答，不映射生产 done_when；`assets/theme_twinkle_v0.mid` sha256 `276e1ffbeba0357329169aecbb8e060694ac8aace4ad09f2ba889ba97b0c8952` 与冻结源一致（复制后 sha256sum 亲证） |
| smoke | `scripts/run_tutor_sim_smoke.ps1`：-Answers/-WrongAnswer 参数化（默认 7,42,123/0 兼容 v0 装置）；schema/step_total 按 start 回执自适应；v1 断言 workflow_data brief/materials 在卷、v0 断言 materials 空；逐答案循环推进断言；DryRun exit 0 亲证 |

## 4. 测试卫生与纪律

- 运行时状态全部 `t.TempDir()`（writeTutorProject/writeTutorV1Project）；无源码树写入。
- 冻结源零改动：`coord/levels/`、theme MIDI 源、production design schema 未触碰（diff 只含本卡文件域）。
- 435/437 既有测试只加不减；唯一例外如实记录：`tutorsim_test.go` 的 wrong-schema 拒绝例替换值由 `vit.tutor_sim.v1`（本卡合法化）改为 `vit.tutor_sim.v2`，断言强度不变。
- 一次 vitest 运行出现无关 unhandled error（`ScoreWorkbenchPanel.test.tsx` teardown 后 window 未定义，文件域外、复跑即绿）；按 §11 记为一次性观察，非本卡 flaky 认定，若复现应由主管另开修复卡。

## 5. 边界与遗留（lead 验收关注点）

1. **音源预设级**：rack_add_node 只挂默认 Surge XT identifier；Surge 预设/program 选择不在既有内核命令面（298 证：VST3 program change 不暴露参数面；384 腿 D 预设固化走模板工程 state blob 机制）。真栈腿若要求预设级音色，属新裁定/新卡。
2. **音源挂载为 best-effort**：内核拒绝时材料（track+clip+notes）仍算装载成功，`instrument_attached=false` + `instrument_error` 如实回传（真栈腿可据此断言或另行裁定）。
3. **装载态会话内存**：agent 重启丢失（与 tutor 会话同级语义，README 已记）。
4. **装载不保存工程**：内核命令面落轨/clip，工程保存仍由用户/宿主动作触发（与产品行为一致）。
5. **`-Answers` 数与 step_total 一致性**：脚本以断言强制（数量不符 exit 1），lead 换装置时需同步传参。
