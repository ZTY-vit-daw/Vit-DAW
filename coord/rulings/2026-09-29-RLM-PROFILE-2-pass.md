# Ruling：RLM-PROFILE-2 pass（2026-09-29，决策侧）

- **裁定：pass**。合入 main=cherry-pick 19a2093（见下提交链）。
- **决策侧独立复验（非转述）**：temp worktree @origin/port/rlm-profile-2 全量 `go build`+`go test ./... -count=1` → **87 包 ok、0 FAIL**（与回执及领取基线一致）；blob gofmt **11/11 清**；diff 亲核=11 文件 +631/−7 全在 agent/internal/{chat,harness,rlm,tools}，**capabilitycontext/queryengine 零触碰**（卡面避让承诺兑现，CCB-PARAM/L1-3-IMPL-B 避让区完好）。
- **采信要点**：①挂点实证充分——render.start job_id 生命周期+终态缓存，仅 ready 可绑、未知/failed fail-closed 拒；②一 render 一 profile 由 RenderBindingIndex.Bind 强制（重绑显式拒 :302，同绑幂等）；③披露走 rlm 自带投影面，**nil 绑定时 JSON 逐字节不变有测试锚定**（WithoutRenderBindingsKeepsLegacyJSONShape）——旧行为零变化达标；④持久化 §11 合规（旧态无字段=nil=零绑定，restore 逐条 fail-closed+非法丢弃计数）；⑤红先行证据归档（vet exit 1→GREEN_EXIT=0）。
- **端测边界裁定**：纯 agent 侧接线+默认路径逐字节不变——按 L2-1-RLM-1/CCB-PARAM 先例**免真栈验收**；新本地工具（render.profile.bind/list）的 /agent/invoke 面烟测并入 IMPL-C 端测收口场景。
- **上交事项采纳**：计量实勘结论（内核 loudness=导入期近似 approximate_rms_lufs_v1、无 BS.1770 K-weighting/gating、无 true peak 计量器、render_done 零计量挂接）成立——**交付监督卡的落点裁定（A=agent 侧真 BS.1770 vs B=内核 L3 扩展+挂 render_done）列为 L2 线待裁项**，近似算法不作交付级断言依据的判断采信。
- 执行侧评价：Mac 首卡即达 PC 侧同质量水位（诚实边界+自核避让区+证据归档）；领取时运行时文件隔离申报合规。
