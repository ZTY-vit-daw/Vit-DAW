# TUTOR-SIM-4 实验工程说明（tutor-sim-2，池序 438）

本目录是 TUTOR-SIM-4 的服务端导师循环实验装置 v1：first-run-twinkle 单关卡、
节点粒度的一课（听主题 → 听辨模进 → 装载材料 → 小结确认）。目标是在 435 三步
算术实验之上，验证 **brief 教学简报 + MIDI clip 材料装载** 的最小闭环：

打开实验工程 → 自动开场（带 brief/材料卡）→ 用户按当前步骤输入 →
**服务端 trim 后精确匹配** → 错不推进、对推进；材料卡「装载到工程」走
agent 既有内核命令面（add_track / rename_track / insert_midi_clip /
add_midi_notes + best-effort rack_add_node，锚点 `scripts/yue_wire_smoke.py` W1）。

本装置**不映射、不判定**生产乐局 done_when；不改冻结 level.yaml 与 theme MIDI
源（`assets/theme_twinkle_v0.mid` 为原样复制，sha256=
`276e1ffbeba0357329169aecbb8e060694ac8aace4ad09f2ba889ba97b0c8952`，与
`coord/levels/first-run-twinkle/assets/theme_twinkle_v0.mid` 一致）。

## 1. 实验 schema（vit.tutor_sim.v1）

在 v0（`coord/experiments/tutor-sim-1/`）基础上，每步新增：

| 字段 | 必填 | 说明 |
|---|---|---|
| brief | 否 | 节点教学简报（多行文本），进 start/chat 回执 workflow_data；UI 折叠展示 |
| materials[] | 否 | 材料列表：`{kind, title, path}`；kind=`midi_clip`（path 必填）或 `audio`（本轮 schema 位，无内容） |

v0 文档仍按原样解析（brief/materials 缺省空，UI 不渲染卡）；未知字段与多文档
依旧 fail-closed。midi_clip 材料在实验加载时即解析 .mid 摘要（音符数/时长/速度，
解析失败记录在材料上、不阻塞实验），路径相对本 YAML 所在目录解析。

## 2. 准备实验工程

1. 用真实产品入口新建并保存一个空白实验工程（如
   `D:/Vit-DAW-experiments/tutor2-blank.vit`）。
2. 把本装置整体复制进工程目录的 sidecar（**保持 YAML 与 assets 的相对布局**）：

   ```powershell
   New-Item -ItemType Directory -Force "<工程目录>/.vit_agent" | Out-Null
   Copy-Item "D:/Vit_DAW/coord/experiments/tutor-sim-2/tutor_sim.yaml" "<工程目录>/.vit_agent/tutor_sim.yaml" -Force
   Copy-Item "D:/Vit_DAW/coord/experiments/tutor-sim-2/assets" "<工程目录>/.vit_agent/assets" -Recurse -Force
   ```

3. 通过正常入口打开该工程，让 agent 的当前工程指向它。

## 3. 与 v0 装置的差异

- 装载面：新增 `POST /agent/tutor/load_material`
  `{conversation_id, material_title, plugin_path?, plugin_identifier?}`。
  成功回执 `load={loaded, track_id, clip_id, plugin_id, instrument_attached, …}`；
  材料已在工程内 → `already_loaded=true`（幂等，不重复建轨）；音源挂载被内核
  拒绝 → 材料仍装载成功、`instrument_attached=false` + `instrument_error` 如实
  记录（track+clip+notes 即材料本体）。
- start/chat 回执 `workflow_data` 新增 `brief`（当前步简报，可为空）与
  `materials`（当前步材料列表，含 loaded/loading 装载态）；v0 实验两字段为空
  列表/空串，UI 不渲染卡。
- `schema_version` 按实验文档回显（v0 装置回 `vit.tutor_sim.v0`，本装置回
  `vit.tutor_sim.v1`）。

## 4. Headless smoke（lead 执行）

`scripts/run_tutor_sim_smoke.ps1` 在 435 断言基础上扩展：

- `schema_version` 按 start 回执自适应（v0/v1 都接受，step_total 也按回执
  自适应，不再硬编码 3）；
- v1 装置：断言 start/chat 的 workflow_data 含 `brief` 与 `materials` 字段；
  v0 装置：断言 `materials` 为空（不渲染卡的契约面）；
- 答案序列可参数化：`-Answers "42,模进,已装载,完成"`（默认仍为
  `"7,42,123"` 兼容 tutor-sim-1 装置）；`-WrongAnswer` 默认 `0`。

```powershell
# 默认仅打印 runbook（exit 3）；dry-run 不发 HTTP（exit 0）
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 -DryRun -ProjectPath "<工程.vit>"

# v1 装置真实栈腿（需主管登记栈位并授权；脚本不启停栈）
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_tutor_sim_smoke.ps1 `
  -StackAuthorized -RuntimeOwner "<登记 owner>" -ProjectPath "<工程.vit>" `
  -Answers "42,模进,已装载,完成"
```

## 5. 边界与已知限制

- 会话与装载状态只存内存：agent 重启后丢失，需重新 start 与装载。
- 音源挂载是 best-effort：内核拒绝时材料装载仍算成功并如实记录原因；Surge
  预设（program/state）选择不在既有内核命令面内（298 证：VST3 program change
  不暴露在参数面；384 腿 D 预设固化走模板工程 state blob 机制），本卡不发明新
  命令——预设级音色属真栈腿/后续卡边界。
- 装载不写工程文件：内核命令面落地轨道/clip，保存由用户/宿主负责（与产品
  行为一致）。
- 真实 LLM 概率性：smoke 只断言服务端状态机、契约字段与装载面，不对开场/引导
  措辞做语义断言。
