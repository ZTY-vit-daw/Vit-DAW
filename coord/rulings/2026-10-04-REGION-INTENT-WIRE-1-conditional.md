# Ruling：REGION-INTENT-WIRE-1 —— conditional pass（2026-10-04 晚，PC 决策侧）

## 裁定

**conditional pass**。机制面三件套全过；转正挂**用户真栈手测复验**（见下）。

## 验收依据

1. **diff 直读**（`bd60a799`，intent.go +122/-1、intent_test.go +219 新文件，无越域）：
   - 路由形态：不新增意图分支——split case 条件扩 `isClipSplitText || len(rangeSplitCommands)>0`，范围形态全部守卫在 `clipRangeSplitCommands`（话语含"这段/这个范围/框选"+split 动词（含扩入的拆出/拆开/拆分）+无口述秒+无播放头指涉+ranges 可解析）；无 ranges 时零变化。
   - **两次 split 先终点后起点的顺序论证我方核实成立**：内核 split 后原 clip_id 保留为左半（ClipService.cpp:1566-1576）——若先切起点，第二次切点将落在切短后的原 id 之外被内核拒绝；先切终点则两次切点对拆分前几何各自有效，框选段独立成 clip。贴 clip 起点/终点退化单切、整 clip 框选不切、`clipMinSurvivingSeconds=0.01` 镜像内核常量守卫，逻辑正确。
   - `selectedClipRangeRows` 镜像 server 白名单口径（clip_id 必在、键白名单、异形 fail-open）；selectedClipArgs 注记 count+首行入 args+clip_id 兜底。
   - 形态决定四项（播放头守卫/动词门不含"拆掉"/兜底注记/mentionsClip 门未扩）均在卡面授权内，自查申报如实。
2. **我方独立复跑**（[verify_pc](../runs/REGION-INTENT-WIRE-1/verify_pc/)）：`go build ./...`+`go test ./internal/conversation ./internal/chat -count=1` 全绿（conversation ok / chat 全包 ok）；gofmt blob 口径两文件 `gofmt -d` 0 行（**我方首轮检查路径写错作废，按仓根路径补跑核实**——留档为验收过程记录）。
3. 8 条新用例覆盖与卡面目标一一对应（切序/零变化/异形×6/多 range 取首/口述优先/贴界×2/播放头保持/args 消费）。

## 归档与配套

合 main=merge commit（2026-10-04，bd60a799 --no-ff）；主树 binary 18:12:21 收口重建（纯 agent 侧改动，webui dist 无涉）。执行侧未 push 分支，验收后随裁定推送 `port/region-intent-wire-1` 簿记。

## 转正条件（用户真栈手测，Godot 拉起途径）

1. 前端 range 工具框选 clip 局部 → 主对话说"把这段拆出来" → 确认卡出现**两条 clip.split** → 批准后拆分发生在框选边界，框选段独立成 clip；
2. 口述秒数切分（"在 5 秒处切开"）仍按口述值；
3. 无框选时旧话术行为不变。

任一不过 → 取证卡。
