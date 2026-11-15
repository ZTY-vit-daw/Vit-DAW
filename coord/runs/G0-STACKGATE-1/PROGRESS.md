# G0-STACKGATE-1 执行进度（PROGRESS）

- 卡：coord/cards/（池序 371，P1）单栈验收口径冻结 DG1-DG3
- 执行 owner：GLM-5.3-Flash 执行流（ZCode 子代理）/ PC
- 领取时间：2026-11-15
- 领取时 origin/main：9cc3048ccdda11c3f889476ebfd85eea3cd46c25
- 协调 worktree：D:/Vit_DAW_wt_371coord（分支 coord/371-claim，基于 origin/main 9cc3048c）
- 主工作树状态（领取基线核对）：main 工作树存在既有脏改动（CURRENT-STATE.md / VitApp Workspace / coord/decisions/2026-11-11 等），本卡不触碰，全部工作在独立 worktree 进行

## 步骤日志

| # | 做了什么 | 产物 | 下一步 |
|---|---|---|---|
| 1 | 建 worktree D:/Vit_DAW_wt_371coord（coord/371-claim ← origin/main 9cc3048c）；本 PROGRESS 落盘 | 本文件 | 卡 todo→doing 领取提交推 main |
| 2 | 领取提交：卡 todo→doing+领取节+POOL-INDEX 371 行同步 | coord/cards/doing/2026-11-15-G0-STACKGATE-1.md | 读参照系（烟测脚本体系/兄弟卡/CURRENT-STATE） |
| 3 | 推送两次被顶：并入 369/370 领取提交（POOL-INDEX 冲突按两侧 doing 行合并解决）；远端核对 owner 在场 | 领取基线 a1db8cf7 | 读参照系 |
| 4 | 参照系读完：vitshell_full_smoke.ps1（L0-L6/F1-F3/S10/占用六步/退出码 0-1）+vitshell_full_smoke_cdp.mjs（CDP phase 机制）+journey1（退出码 0/1/2+隔离契约）+run_webui_rendered_dom_smoke（E2E-WEBUI-1 门槛）+run_yue_dialect_smoke（leg0/1/2+VRAM ceiling+引擎所有权）+edu_jam/legame_intro（0/2/4+零 LLM 遥测）+g_runtime_readonly（白名单 GET）+审计 README/SCHEDULE/WORK-PACKAGES+裁定+占用记录+兄弟卡 369/370/372 | 无（读） | 写关卡定义文档 |
| 5 | 出声机械断言锚点核实：内核 l2_render_probe 命令面（scripts/dad_probe.py 驱动）；YuE2 WAV 时长合理窗+SheetSage2 转写回读（yue_dialect_smoke.py） | 无（读） | 写关卡定义文档 |
| 6 | 关卡定义文档落 docs/ 候审：DG1-A/B/C 断言面+DG2 乐局段 J1-J10+DG3 双门槛+手测节点 M1-M4+引用模板+映射表 12 行 | docs/STACKGATE_DG1_DG3_V1.md | 收口提交（卡转 done+回执+POOL-INDEX） |

