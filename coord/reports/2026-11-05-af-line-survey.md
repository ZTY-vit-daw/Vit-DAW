# A-F 混音能力主线开线勘察（2026-11-05）

- 编制：GLM 主管决策流 / 2026-11-05 晚窗（依据=用户三线计划 2026-10-24「混音 A-F」+2026-11-05「先勘察，发卡」）
- 结论先行：A-F 目录 23 节点中 **C5/D（除观察面）/E/F1 无 runtime**；首批两卡打 **D1 全局响度与峰值**（地基最近）与**轨道清单权威缺陷复验**（观察完整性地基）；C5/E/F 列 backlog。

## 一、A-F 定义与目录（权威源=agent/internal/chat/mix_workflow_queue_view.go:80-104）

A-F=混音能力**层**（capability layers，非线性门禁——static_mix_capability_contract.go 明文）。23 节点六族：

| 族 | 节点 | runtime 现状 |
|---|---|---|
| A 工程准备 | A1 接收整理/A2 技术完整性/A3 轨道整理/A4 片段清理/A5 段落地图 | capability ID 在册（A2/A3 有 TIM/TOM 投影底座；journey 腿曾 GREEN） |
| B 粗混静态 | B1 增益分级/B2 静态平衡/B3 声像/B4 低频关系 | 已建模有 contract（B5 已退役并入 B2） |
| C 细混 | C1 频段清理/C2 动态控制/C3 空间深度/C4 段落自动化/C5 总线处理 | C1/C2/C3 有 schema 族；**C4=M3 链（今日收口）**；C5 无 runtime |
| D 母带前检查 | D1 全局响度与峰值/D2 翻译检查/D3 参考对比/D4 版本输出 | **D1-D3 观察统一收口 pre_master.observation.v0**（capability_routing.go:658）；无执行 runtime；loudnesswatch 包在库（trigger/compare/config ~1300 行含测试） |
| E 母带 | E1-E4 | **无 runtime** |
| F1 混音审查 | — | **无 runtime** |

## 二、四个保留分支归属裁定（250 收尾遗留，本次勘察定性）

| 分支 | 内容 | 裁定 |
|---|---|---|
| port/timefx-bus-writegate-1（ahead2） | 155 写门实现+烟测腿 | **已入 main**（4bbb211a/f427e88d 在 main 史+烟测腿在 main 脚本亲证 8 处命中）——分支=cherry-pick 残余，**关线**；引用保留零成本，卫生卡可清 |
| port/timefx-bus-topoplan-1（ahead12） | 154 九 run 证据+bus 装载授权修正（c9708940） | **证据件已在 main**（coord/runs/TIMEFX-BUS-TOPOPLAN-1 在册）；实现修正被 main 的 pca-fullaccess-1（8a415ef7，authorizeFullProjectAccessLoad 服务端权威目录解析=超集修复）**覆盖**——分支=superseded，关线；引用保留 |
| port/track-list-gap-1（ahead5） | 139 诊断（tool face 返回 shadow Summary 忽略 kernel reply，delta 轨孤儿化）+fix 草稿+GO run PASS | **缺陷复验未做**——诊断件在 main，修未合入；且后续 fix-tracklist-shadow-freshness-1/VSP-SHADOW-REFRESH 系是否已覆盖未核 → **开卡 255 复验**（在→修；不在→证据关线） |
| port/loudness-watch-trigger-1-d1（dirty72MB） | 90 修订腿：D1 settle 重锚 LW 真栈腿脚本+两 run；**gate4 被产品缺陷阻断上交** | **开卡 254 承接**（D1 主线卡以该脚本与缺陷记录为输入） |

## 三、首批卡（池序 254/255，文件域不相交）

1. **254 AF-D1-LOUDNESS-SETTLE-1**（P1/1 天/L1-L2/真栈腿）：D1 全局响度与峰值能力落地——loudnesswatch 包为底+pre_master.observation.v0 收口面已有；承接 90 腿脚本与 gate4 缺陷；目标=D1 节点 capability 点亮（观察+阈值判断+确认后动作面）。
2. **255 TRACKLIST-AUTHORITY-VERIFY-1**（P1/0.5-1 天/L1/复验优先）：内核回复权威缺陷复验——当前 main 上复测 139 诊断形态；在→按分支草稿实现修（文件域 harness/shadow 面）；不在→证据收口。

## 四、后续 backlog（下轮排程输入，未开卡）

- C5 总线处理 runtime（设计与实现两段）；
- D2/D3/D4（翻译/参考对比/版本输出——D3 与 RLM 参考电平投影天然衔接）；
- E1-E4 母带族、F1 审查（大面，建议答辩后或按 demo 需求裁剪）；
- 既有欠账：E2E-WEBUI-1（渲染面烟测建设）、JOURNEY-1（旅程烟测建设）——§5 两层层级债务，涉 webui/旅程的改动前应补位；
- 卫生项：154/155 残余分支引用清理（随下轮卫生卡）。

## 五、里程碑口径

- M3 链=C4 段落自动化（今日收口，见 2026-11-05-m3-chain-closeout.md）——**A-F 主线的 C4 已点亮**；
- A-F 主线开线首里程碑=**D1 点亮**（254），其后按 backlog 排；
- 生成线（AI 鼓组/艺人试音/人声合成/Suno）与编曲基座线另排，不与本批混域。
