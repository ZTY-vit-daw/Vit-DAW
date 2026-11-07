# Ruling 2026-11-07 D1D3-DIGEST-WIRING-1：pass

- 验收：GLM 主管决策流 / 2026-11-07 早窗
- 实现 commit：4c437f68 @ port/d1d3-digest-wiring-1（合入 main 见回执）

## 验收结论

1. **接线修 pass**：d1d3AssemblePreMasterReport 三面全量改读 d1d3DigestView（`out["digest"]` 子图主源+`out["acoustic_digest"]` 拍平面归一兜底，stereo_relation_status→stereo_status 键名归一落在浅拷贝视图不污染响应本体）；observation_id 顶层优先/digest 兜底——与 272 取证对照矩阵逐面对齐。
2. **单测反钉 pass**：夹具蓝本=265 实盘 obs_20261106T152825（数值/行形状实盘原值）+266v1 settle 形态（waveform missing+rankings 在场）+**顶层旧假形状防回退组**（喂顶层形状必须投影 missing——杜绝双形状自洽假绿的教训已入测试面）；主管亲读 diff 确认。
3. **交叉复跑 pass**：主管在 worktree 亲跑 `go test ./internal/chat -count=1`（128.1s ok）+`./internal/mixboard ./internal/mom`（ok）+`TestD1D3` 专项（ok）。
4. **相邻两处勘察=取证交付合格**：①D1 waveform settle 翻转驱动定位到 currentProjectFeatureRowOrMissing 的 project_material_identity_mismatch（mixboard.go:2534/:2544-2559，266v1 WE 槽被 _1055 行替换且材质失配 vs 265 _1011 匹配——两 run 观察件亲核）；②逐轨 stereo 7/7 missing 定位到 readObservationKey 键路由取观察包 TargetRef.ID（full_project="current"，catalog.go:469/:570）恒 miss。两处均跨域未扩修、锚点+双证在卷——转卡 293（WE 失配取证）/292（mix.read 键路由修）。
5. **端测边界（AGENTS §5）如实**：本卡零真栈=单测+build 级；真栈回归门=266 既有 D1 门 A+D2/D3 门断言，搭后续 run（记 backlog，不阻断本卡收口——接线面由实盘形状反钉+全包回归背书）。

## 账务

- 零 LLM 零真栈，无计费；kernel/harness 响应结构本体零改动红线亲证（diff 仅读取侧）。
- 本裁定入 L1 台账，48h 翻案窗口。
