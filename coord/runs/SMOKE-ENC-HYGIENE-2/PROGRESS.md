# SMOKE-ENC-HYGIENE-2 PROGRESS

任务：run_free_state_d1_smoke.ps1 编码读取卫生残留两处（:630 NOT_EXERCISED reason 读取、:445 LW manifest 读取各加 -Encoding UTF8）+ 构造验证最小两腿。
工件目录=D:/Vit_DAW/coord/runs/SMOKE-ENC-HYGIENE-2/；终产物=coord/cards 回执+construct/ 工件。

## 步骤

- [x] S1 git fetch origin + 领取基线：主仓 HEAD=8732bd21=origin/main；已有改动均非本卡域（VitApp 两 xml/UX-AB 工件/untracked），不碰
- [x] S2 worktree D:/Vit_DAW_wt_270impl @ port/smoke-enc-hygiene-2 建立成功（基线 8732bd21）
- [x] S3 卡面领取回填+提交 f7204a25 推 main 成功（远端已核对 f7204a25）；行号核对：:445/:630 未漂移，全文件仅这两处 Get-Content 无编码，268 修复零回退，无第三处同族
- [x] S5 构造验证完成：probe_270 attempt3（20261107_001930）exit 0，5/5 ALL PASS——leg1/leg3 pre 行乱码缺陷实证、leg2/leg4 post 行 clean_parse、leg5 静态扫描（post 0 处无编码=268 四处+本卡两处零回退）；attempt1（pattern 歧义）/attempt2（桩漏 }）失败留档，RESULTS.md 已写
- [ ] S6 diff+commit 推送 port/smoke-enc-hygiene-2；卡面回执回填（diff+两腿退出码+commit）+ 工件随回执提交推 main

## 长命令记录

- git push origin HEAD:main（领取提交 f7204a25）— 已完成 EXIT=0
- pre/post 快照已导出：construct/pre_baseline_run_free_state_d1_smoke.ps1（git show 8732bd21，:445/:630 无编码残留确认）+ construct/postfix_run_free_state_d1_smoke.ps1（工作树复制）
- 正在跑：powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW/coord/runs/SMOKE-ENC-HYGIENE-2/construct/probe_270.ps1 -OutDir <construct 下时间戳目录> -PreScript <pre 基线> -PostScript <post 快照>（离线零栈，5 腿：2 读取点 × pre/post 对照 + 静态零回退扫描）

## 断点备注

（无）
