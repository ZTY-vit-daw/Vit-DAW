# SMOKE-SCEN-RANGE-1：范围线两卡烟测场景——note 载荷时间维度注入+意图层框选拆分提案（dev_agent_smoke 扩展）

- 池序 3（REGION-TIME-1/INTENT-WIRE-1 转正判据卡）；目标仓库=D:\Vit_DAW（agent+scripts）；依据=AGENTS §5 端侧烟测门槛（新场景=现有冒测脚本加参数）+2026-10-04 晚用户质询处置（手测兜底→脚本化）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（两卡实现已合 main：4c98c643/2f5e57d1/c2edb0d6）
- 模型分级：L1 / flash 可接（先例=dev_agent_smoke.ps1 场景模式+g_runtime_readonly_smoke 白名单 GET 模式）
- 目标：
  1. **场景 A（REGION-TIME-1 转正判据）**：`dev_agent_smoke.ps1` 新增参数场景——起真实三件套后，POST `/agent/chat` 携带 note 载荷（faces 含 timeline 面 domain.range_time_span+条目 range_clip_start/end+顶层 range_time_span，fixture 构造合法 v3.1 形态）；断言打**组装/投影面**（非 LLM 文本——AGENTS §8 确定性口径）：note 会话落库+遥测/状态面可见 time_digest 注入（领取时实锚断言点：note_sessions 投影/section_stats/等效确定性面）；旧载荷（v3 无时间键）fail-open 零报错同场景断言。
  2. **场景 B（INTENT-WIRE-1 转正判据）**：同栈 POST `/agent/chat` 携带 requestContext.selected_clip_ranges（合法结构 fixture：单 clip 单 range，界在 clip 内）+userText「把这段拆出来」；断言响应提案=**两条 clip.split（先 end 后 start）**；反例组：无 ranges 同话术→零提案（回落既有行为）。
  3. 脚本以 **exit 0** 收口（两场景各成组，-Scenario 参数选择）；run 工件落 coord/runs/SMOKE-SCEN-RANGE-1/。
- 文件域：scripts/dev_agent_smoke.ps1（加参数/场景）+必要 fixture 文件（scripts/fixtures/ 下）；agent 代码零改动（发现断言点缺失属停止条件）。
- 约束：真栈泊位（E2E 7897 即起即拆模式，不占用户栈）；不碰 webui/Godot。
- 验收标准：脚本 exit 0+两场景断言全过+run 工件可回指；决策侧复跑一轮 exit 0。
- 停止条件：断言点实锚发现组装面无确定性暴露（time_digest 只活在 LLM prompt 内部）→ 锚点清单上交，决策侧定断言面或降级口径。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 断言点实锚 / run 工件路径 / 泊位声明）
- 验收：（裁定文件 / 验收 commit）
