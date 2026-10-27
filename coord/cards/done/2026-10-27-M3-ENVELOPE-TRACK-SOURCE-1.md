# M3-ENVELOPE-TRACK-SOURCE-1：轨级参数包络在场性数据源探测（190 验收裁定遗留）

- 发卡：GLM 主管决策流 / 2026-10-27 早窗（依据=190 验收裁定「轨级在场性缺口留主管另裁数据源，候选=轨级 automation.get_curve 探测或内核轨参数面小扩」+gate 2026-10-26 §5 首排②）
- 派发确认：已确认（190 验收裁定链+gate 2026-10-26 首排）
- 验收负责人：GLM 主管决策流
- 池序 196；目标仓库=**主仓 D:\Vit_DAW**（scripts/ 探测域+run 工件；**VitApp/ 生产源零改动**）；P2 / 0.5 天 / 依赖=无（190 已合入 ✓；M3-KERNEL-1 automation 三命令已合入 ✓）
- 模型分级：L1 / PC 执行会话
- 并行与资源：**真栈腿与用户手测节错峰**（§2.2 增补③——用户手测占用主检出 Godot 栈期间不启栈；领取/登记前 fetch 重读 PC-RUNTIME-STACK.md 与手测节占用状态互查）；纯文档/工件腿不受限。

## 已核事实（发卡侧亲读回执，执行侧领取后复核）

1. **轨级曲线面大概率可用**：189 真栈腿（envelope_lane_gui_20261025_2048）在内置 volume 插件上 set_points 三点+车道 get_curve 回读逐点一致已实证（探针侧直调 client，C1 AutomationClient 真管线）。
2. **描述符面不可用=根因**：get_plugin_parameters 对内置轨级插件回 error「Plugin Parameters are only available for external plugins」（189 L1/190 卡面停止条件逐字在卷）→ envelope_point_count 不覆盖轨级=192 自动档对轨级参数（volume/pan）不触发的已知缺口。
3. **未知面**：轨级参数寻址全貌（volume 已见实证；pan 及其他轨参数 id 形态未证）、多轨下寻址方案、get_curve 只读白名单对象面是否含轨级参数。

## 目标

1. **探测腿（真栈，ps1 冒测扩展=190/M3-KERNEL-1 模式）**：add_track×2→对轨级 volume/pan（及在案其他轨参数）逐参：get_curve 空披露→set_points 三点→get_curve 回读逐点一致→clear_region→回空——逐参数记录命令可用性/回执形态/错误文案（若有，逐字）。参数寻址从 M3-RECON-1 §2.3+189 run 工件锚定；锚点不符即上交。
2. **数据源结论（只裁证据不实施）**：两案对比——A=前端侧轨级 get_curve 探测接入 192 扫描腿（零内核改动）；B=内核轨参数面小扩（envelope_point_count 覆盖轨级）——每案成本/兼容/发现时机（刷新链 vs 写回执重扫）各两行；交主管择一后另开消费实施卡，本卡不实施。
3. **边界如实**：轨级参数族若有不可寻址/无曲线挂载成员→如实记录不硬造（AGENTS §10）。

## 文件域

`scripts/`（探测脚本，M3-KERNEL-1/190 冒测模式，**优先复用既有脚本加参数**不新增文件）+`coord/runs/M3-ENVELOPE-TRACK-SOURCE-1/`（工件）。**VitApp/ agent/ webui/ 前端仓零改动**。越域即停。

## 验收标准

探测脚本 exit 0（异常类逐字留证）+工件入 run 目录（完整命令/run ID/HEAD/关键日志，AGENTS §9 工件契约）+RESULTS.md 含数据源两案对比；端测边界声明。

## 停止条件

- 轨级参数寻址与 189/M3-RECON 锚点不符（内核已重构）→锚点上交；
- set_points 对轨级参数回 error（189 L2 前提失效）→如实留证，本卡转 B 案证据卡上交，**不擅自改内核**。

## 领取

- 时间：2026-10-27 早窗
- origin/main hash：46400263c45a06c08d7a4ba66b78a566f2f79bd5（领取核对时两次基线前移 2fa99bd3→c1c6a63c→46400263，均为他卡状态提交，按 §2.1.3 重放后核对无人领取）
- owner：PC 执行会话（GLM-5.3，ZCode，196 卡会话）
- 分支：port/m3-envelope-track-source-1（实现，自领取后 main 建立）
- worktree：D:/Vit_DAW_wt_etsrc1（领取核对基线：git rev-parse HEAD=46400263、status 仅本卡 R 一行）
- 领取提交：本提交（coord/m3-envelope-track-source-1-claim → main）

## 回执

- 实现 commit：port/m3-envelope-track-source-1 @ **cafbc11f**（scripts/automation_curve_smoke.py 探测模式增量+dev_agent_smoke.ps1 -TrackSourceProbe 腿；默认冒测路径零改动，同栈 190 腿 13 checks 全绿即回归证据；VitApp/ agent/ webui/ 前端仓零改动）
- 真栈 run：**coord/runs/M3-ENVELOPE-TRACK-SOURCE-1/tracksrc_20261027_1232 ps1 exit 0**（登记 83adfd32/释放 8ae4ea2b；内核=190 二进制只读复用 sha 85B4E8DF；VIT_PROJECT_XML=run 副本 90AFB63C 隔离亲证；占用期即时复核/无 already listening/登记先于启动三纪律在卷）
- 探测结论：**volume/pan 双参数轨级全周期可用**（双轨×逐参 空披露→set_points 三点→逐点回读一致→跨轨 plugin_id 隔离→clear_region→回空；pan 本卡新实证）；master volume/master pan/trim/mute/gain 与 Level Meter 全参数不可寻址（"Parameter not found for param_id: X" 逐字）；descriptor 面 error 与 189 逐字一致（190 内核文案未变）
- 两案对比+端测边界+工件清单：RESULTS.md（run 目录内）；执行侧意见倾向 A（零内核改动），裁定权归主管
- 停止条件：未触发（锚点相符/set_points 轨级零 error）
