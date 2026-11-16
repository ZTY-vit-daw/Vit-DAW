# KERNEL-SAVE-FORENSICS-1：D1/D2 内核保存语义取证修复 + D3 agent 消毒面（374 上交锚点）

- 发卡：主管 2026-11-16 晨（承接 374 域外锚点三件，未扩修纪律正确）
- 池序 378；P1；依赖=无；真栈候栈位
- 目标：①D1：XML-boot 会话加轨 save 丢轨（取证→修复）；②D2：save 后 clip_id 披露降级直至重开（取证→修复或明示语义）；③D3：agent 消毒面剥 project_path 对谱面面板的影响（锚定消毒位→最小披露方案）；④各配回归测+真栈断言。
- 文件域：VitApp/Source 保存/披露链+agent 消毒位+测试+coord 工件；领取时锚定。
- 验收：三锚点各自的取证-修复-回归链+真栈腿。

## 领取

- 时间：2026-11-16 晚 +0800（领取提交时间以 git 为准）
- 基线：origin/main `919e9650`（fetch 亲核；领取窗口内 377 done 提交刚落，worktree 在最新 tip 上建立）
- owner：GLM-5.3 执行子代理（ZCode L2 会话）/ PC
- 分支：`port/kernel-save-forensics-1`；worktree `D:/Vit_DAW_wt_378save`；协调 worktree `C:/Users/timoz/wt_378coord`
- 领取时基线核对：HEAD=919e9650，status 干净，diff=0（新 worktree 即净）
- 文件域锚定（领取时核对）：`VitApp/Source` 保存/披露链（save_project/open_project/get_project_state 域）+agent get_project_state 消毒位（领取时锚定具体文件）+内核 C++ 测试（VitApp tests）+`scripts/` 烟测扩展+`coord/runs/KERNEL-SAVE-FORENSICS-1/**`+卡面/栈登记。doing 卡核对：WAVESHELL（只读取证）/LAUNCHER-ZEROFRICTION（只读）/SRC-PRESET-AGENT（processorregistry 域）/377 done 挂账（scoreworkbench 域）——与本卡无交集。
- 三锚点口径（卡面为权威）：D1=XML-boot 会话加轨 save_project 后新轨被丢（.vit-boot 不复现）；D2=save_project 后 get_project_state clips[].clip_id 降级 null 直至 open_project 重开；D3=agent get_project_state 消毒面剥 project_path（374 腿经 save_as 目标+--project-xml 双源绕开）。

## 回执

（执行侧回填）

## 验收

（决策侧回填）
