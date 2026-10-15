# FE-CEF-FOCUSFIX-1 run 工件索引

- 卡：coord/cards/doing/2026-10-14-FE-CEF-FOCUSFIX-1.md（池序 114）→ done
- 执行：Mac 执行侧（GLM 会话），2026-10-14
- 主仓 worktree：/Users/timozty/Documents/Vit-DAW-fecef2（分支 port/fe-cef-focusfix-1，
  领取基线 origin/main f50279d2→b320069a）；前端仓分支 port/fe-cef-focusfix-1
  （基线 6e5a459=FE-CEF-KEYINPUT-RECON-1 交付态，领取时工作树仅未跟踪运行时目录）

## 工件清单

| 文件 | 内容 |
|---|---|
| UPSTREAM.md | 上游线结论：dsh0416/godot-cef 定位、我方 v1.15.4、#183/#228/#236 修复家族比对、v1.16.2 升级路径+7 项回归清单 |
| VALIDATION.md | 驱动实证：合成卡死态补偿有效性（ON 自主恢复×5 vs OFF 卡死×5）、第二层缺陷（IME 代理文本重放）证据与机制、验收结论对照、复现命令 |
| autodrive_off_control.log | 补偿关 5 轮（最终常量：0.16s/键）：合成卡死 5/5 保持、round 3 起代理重放错乱、FINAL fail 2/5（.log 受 gitignore 留盘本 worktree run 目录，可回指） |
| autodrive_on.log | 补偿开 5 轮（同常量）：compensation.fire×5+page.focus_recovered×5、round 1-2 过、round 3 起同错乱（与 OFF 逐字节同）、FINAL fail 2/5（同上留盘） |
| frontend_compensation.patch | 前端仓 commit 34072f0 全量补丁（GH001 推送限制归档模式；VitDebugFlags 补偿开关+host 补偿态机+autodrive 驱动场景） |

## 一句话结论

上游根修=升级 gdcef v1.16.2（含 macOS 验证过的焦点统一修复 #228 与代理修复 #236）；
宿主补偿（默认关）在其目标面（R2 卡死签名）机制证明成立——ON 自主恢复 vs OFF 卡死
各 5/5；四步序列 5 轮全对被**补偿范围外的 IME 代理文本重放缺陷**阻断（gdcef 内部，
升级可期），如实上交决策侧。
