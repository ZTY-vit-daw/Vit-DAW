# CAPABILITYPACK-KEYADVERT-1：能力包提示词 catalog 键宣传与键面真实性对账（292 勘察①转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[292 回执勘察①](../done/2026-11-07-MIXBOARD-TRACKKEY-1.md)+[292 裁定](../../rulings/2026-11-07-MIXBOARD-TRACKKEY-1-pass.md)⑤——`tools/capability_pack.go:475` 提示词把 `track.{id}.slow.time_energy.summary` 宣传为稳定 catalog 键，但 full_project 观察上真实 track id 恒 miss 且逐轨 time_energy 材质边界受限（current_metrics 仅单目标；track WE 行 time_segments 被内联投影剥离，仅证据 blob 有）——**宣传键≠可读键=提示词诚实性缺陷**，与 272/274「消费面与真实形状错位」同族但方向相反（这次是宣传面超前于键面）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 295；目标仓库=主仓 agent（tools 提示词面）；P2 / 0.5 天 / 依赖=292 已验收合入；L2 / PC；零栈

## 目标

1. **对账处置（执行侧在两案中择一，须证据钉死）**：A=宣传面修正（提示词只宣传真实可读键——time_energy 若材质边界受限则改宣传其真实形态或移除该键宣传）；B=键面扩展（若 time_energy 逐轨材质实际可读（如经证据 blob 路径），按 292 同型路由扩展 track-keyed time_energy——材质边界论证须在卷）。默认倾向 A（诚实性优先，与 272 裁定「缺失如实保留不升级」同哲学）；B 须先证材质可用。
2. 顺检：capability_pack.go 其余提示词键宣传逐键对账（只记录同型项）。
3. 回归：`go test ./internal/tools ./internal/chat -count=1` 全绿+build 0。

## 约束

- 文件域=agent/internal/tools/capability_pack.go±同包测试（如走 B 案可触 mixboard/catalog.go 键路由——292 已合入该域现为公共域，触之须在回执声明）；不触在飞卡域。
- 禁止 git reset --hard / git clean；不动主工作树；零真栈。

## 停止条件

对账发现宣传键背后是更大的键面设计问题（如整个 catalog 键命名体系与观察包结构系统性错位）→取证上交转设计裁定，不做大改。

## 领取/回执/验收

- 领取：2026-11-07 18:26 +0800 ｜ origin/main=f5b30165599acd9b3c3f68e9cbb0412bda3bff44（晚于发车核对点 a93dd97c 一笔=297 领取，纯 coord/，`git diff --stat a93dd97c..f5b30165 -- agent/ scripts/ docs/` 为空=文件域等价）｜ owner=GLM 主会话 L2-worker 子代理（ZCode/PC）｜ 分支=port/capabilitypack-keyadvert-1 ｜ worktree=D:/Vit_DAW_wt_295ad ｜ 领取基线：HEAD=f5b30165、status 全净、diff --stat 空（fresh worktree）｜ 前置核对：卡在 todo/ ✓ 派发已确认 ✓ 依赖 292 ruling pass+5c8712a6 已在 main ✓ doing 四卡（ARRLOOP-AUTHBRIDGE-1/B3-ABLATION-1/CLOSURE-ROUTE-ADAPT-1/PLUGIN-CONTROL-EVAL-PREP-1）文件域与本卡（tools/capability_pack.go±同包测试）不相交 ✓
- 回执：2026-11-07 19:10 +0800 ｜ **A/B 择一=A 案（宣传面修正，改宣传真实形态）**，证据三点：①恒 miss 机制——mixboard `readObservationKey` 的 time_energy case 臂（catalog.go:553）只按观察自身 `TargetRef.ID` 键控，full_project 包 TargetRef={id:"current"}（catalog.go:603 亲核注释）→ 真实 track id 键走 default；292 路由（parseTrackKeyedStereoKey:612-615）只解析 stereo 双面（catalog.go:605-606 注释明示 time_energy 不在其域）→ `catalog_key_not_found` 恒 miss；②B 案材质边界在卷但不采纳——逐轨 time_segments 确在证据 blob State A（track WE 行携带，292 勘察"仅证据 blob 有"），但可读有条件（v2 持久化激活+blob 未被预算清理+该轨有 WE 行且含有效 segments；265 实盘 6/7 轨，1039 无行），且以 time_energy 键名披露 WE 行 time_segments 属语义替换、当前无消费方请求 track-keyed time_energy——按 272「缺失如实保留不升级」哲学与卡面默认倾向不扩键面；③该键真实形态——`track.<target>.slow.time_energy.summary` 是每观察包真实 catalog 条目（BuildCatalog:270），按观察自身目标 id 键控（per-track 观察时即真实 track id 可读，full_project 时=track.current.*），故修法=改宣传此形态而非移除。**修改**：capability_pack.go mix 包 ResultHints 原"stable keys … and track.{id}.slow.time_energy.summary"改为"stable keys …；track.<id>.slow.time_energy.summary is keyed by the observation's own target id (track.current.* on full_project observations), so another track's time_energy needs its own per-track observation instead of a full_project catalog key"。**diff**：2 文件 +66/-1——capability_pack.go 1 行改写；capability_pack_test.go 新增（同包测试：TestMixPackCatalogKeyAdvertStatesOwnTargetKeying 钉诚实形态三要素+四个真实稳定键不丢失；TestCapabilityPackProseAvoidsPerTrackIdKeyTemplate 全包 track.{id} 模板反钉），**红先行验证**：旧文本两测试 FAIL（exit 1，实测）→ 修后 PASS。**commit**：impl `3ff051b0` @ `port/capabilitypack-keyadvert-1`（已推远端；mixboard/catalog.go 零触碰=B 案未走，无域声明项）。**测试**：`go build ./...` exit 0；`go test ./internal/tools ./internal/chat -count=1` exit 0（tools ok 0.403s + chat ok 158.371s 全绿）；新增测试显式 -run 复跑 PASS。§8 跑前口径已成文（1 次 build+1 次测试，确定性无 LLM，止损线=同版本同断点两连败停）。**顺检同型项清单（只记录不扩修）**：①B1 包 StateSlices（:321）mix.read 四键 project.tracks.summary/project.risks.headroom/project.rankings.peak/project.acoustic.tracks 逐键对账全部真实（catalog.go:256/265/263/257）✓；②mix 包 ：474 digest/available_detail ✓、:476 relationship 五型 ✓（catalog.go:348-356）；③ccb.observation_catalog/ccb.observation_request 工具真实（tools/catalog.go:665-666）✓；④Tools 数组经 allowed 交集自滤（capability_pack.go:85），不存在的工具名自动不展示，不构成宣传≠可读同型项；⑤域外同型（mixboard 键面非提示词面，292 回执已记录）：relationshipEvidenceRefs 的 track.<id>.fast.levels ref、band_energy/static.identity 休眠面、BuildCatalog 只广告自身目标 track 键（292 记为良性，与本卡修正后宣传口径一致）——未发现 capability_pack.go 内其他同型项。**停止条件未触发**（未发现 catalog 键命名体系与观察包结构性错位；缺陷限于单条宣传句）。**端测边界声明（AGENTS §5）**：本卡零真栈——验证边界=Go 单测（tools 含新增反钉+chat 全包）+全量 build，未覆盖真实运行栈端侧行为与提示词实际消费面（LLM 会话级效果）；零 LLM 零真栈，无计费，无 run ID。
- 验收（2026-11-07 晚窗，主管）：**pass**——A 案择一证据充分（恒 miss 机制 catalog.go:553 键控亲核+B 案材质边界如实评估不采纳=272 哲学一致）；1 行改写语义准确（按观察自身目标 id 键控+跨轨走 per-track 观察的诚实形态）；红先行测试（旧文本 FAIL→修后 PASS）+主管交叉复跑 tools 包绿；顺检同型项清单合格（域外项 292 已记录不重复）。实现 3ff051b0 合入 main。S1-P 基线前披露面已净（299 PREP 前置衔接达成）。L1 台账 48h 窗口。
