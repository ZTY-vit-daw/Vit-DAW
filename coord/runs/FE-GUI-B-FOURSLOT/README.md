# FE-GUI-B-FOURSLOT（卡 91）工件索引

- 领取：2026-10-14 10:52 +0800 / origin/main=dad17d85（领取提交 b2e8f678）/ owner=GLM-5.3 L1 PC 执行流（ZCode）/ 前端仓分支 port/fe-gui-shrink-b-1（自 Phase A 基线 c56e38f 建）/ worktree=D:/Godot/project/vit-daw-frontend-fe-gui-shrink-b
- 前端仓实现 commit：1c61482（已推 origin/port/fe-gui-shrink-b-1）
- 主仓脚本：scripts/run_fe_gui_shrink_phase_b_probe.ps1（红线 git 门+九探针电池，Phase A 同模式）

## 工件目录

| 目录/文件 | 内容 |
|---|---|
| `20261014_112758/` | ps1 电池一次性全量运行日志（红线门通过行+Phase B 119 钉+Phase A 69 钉+vitnote 六探针+FE-RACK 76 钉，全 exit 0）；附 Phase B 探针首轮×2 连跑手动日志（run1/run2_manual） |
| `dev_agent_smoke_20261014/` | 真栈三件套烟测 console_stdout.log（**exit 0**，mix.observe obs_20261014T034250_80770a0c23d5=红线#8 真栈活证）+ godot_windowed_boot.log（D3D12 真渲染、start_page 就绪、零 SCRIPT ERROR） |

## 端测覆盖边界（AGENTS §5）

Godot 前端无浏览器 DOM 断言面，headless probe=端侧门（FE-RACK 先例口径）：探针覆盖=引擎纯逻辑全语义（LRU/顶替/重排/防御）+全树装配+注入钉（F2 键级注入 parse_input_event 原生生效、面板按钮 pressed 信号、chip 拖拽经控制器公共 notify_* 注入口=生产同代码路径、双击=真实轨行信号诚实分支+装配核心直调）。未覆盖=真实视觉观感/指针手感/焦点次序/动效——归用户手测清单；三诊断探针端到端运行需真栈 VSP hub 在线（headless 实测 VSP hello 失败=环境边界），其 slot-grid 等价物以静态钉+stage_apply_probe_layout API 三形态钉收口。
