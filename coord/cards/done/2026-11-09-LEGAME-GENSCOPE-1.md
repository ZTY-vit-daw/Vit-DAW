# LEGAME-GENSCOPE-1：手测 v2 清单裁出生成旅程（L1-d 案 B，生成线裁定收尾件）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=[生成线裁定](../../decisions/2026-11-09-gen-capability-line-ruling.md)：用户定向引擎产品化归生成能力线，手测 v2 按案 B 执行；313 报告 §2.4/卡 4 原案）
- 派发确认：已确认（裁定落档即生效）
- 验收负责人：GLM 主管决策流
- 池序 333；P2 / 0.2 天 / 依赖=零；L1 / PC；零栈零 LLM 纯清单修改

## 目标

1. 手测 v2 步骤清单（coord/transfer/2026-11-08-M2-handtest-steps.md）：删除「YuE2 引擎+providers 手动前置」整段；
2. 增补边界声明段：生成能力 12 月展演以「局外成品」口径展示（录播素材里放生成结果，不现场跑引擎）；生成体验旅程候 AI-first 启动面（332 设计定稿后的手测 v2.5/v3 入口）；
3. 与 AI-first 启动面轨道衔接一句话（新启动面 feature-freeze 11 月底，就绪则录播入口改走新面——见裁定 2026-11-09-ai-first-launcher-ruling 纪律）。

## 文件域

coord/transfer/2026-11-08-M2-handtest-steps.md+本卡面；零代码。

## 验收标准

清单评审：无手动服务前置残留（grep 亲证 YuE2/providers/env 前置段清零）+边界声明在卷+commit hash。

## 停止条件

清单结构发现与 L1 三件实际布位冲突 → 上交候裁定（不擅改布位段）。

## 领取

- 2026-11-09 晚 / 领取成功时 origin/main=73e1b166→fd31c879（首推基线 f12635af，被拒后按 §2.1.3 重应用，详见回执补记）/ owner=GLM-5.3 执行子代理（333 卡 L2，PC，本会话）/ 分支=port/legame-genscope-1 / worktree=D:/Vit_DAW_wt_333impl
- 领取基线亲证：worktree HEAD=f12635af（干净基线：`git status --short`=空、`git diff --stat`=空；领卡窗口内 327 流两次 coord 推进+332 领取已含，卡仍 todo/doing 域核对无冲突）——本卡 diff 全为本卡新增
- §8 跑前口径：零栈零 LLM 纯清单修改，无概率性运行；验收=确定性 grep+git 亲证，无运行次数/止损线需求
- 领取提交=fd31c879（已推 main，远端 doing/ 卡面 owner 复核✓）

## 回执

- **diff 文件清单**（全在卡面文件域，impl commit=**296c80ba**，branch=port/legame-genscope-1 已推 origin）：
  - `coord/transfer/2026-11-08-M2-handtest-steps.md`（+10/-10），四处：①标题口径改「乐局一局（生成一首已裁出候 v2.5/v3）」；②〇节「旅程 B 前置」生成引擎手动服务前置整段删除，换一行删除记录指向第三节；③三节旅程 B 整段（B1-B3 表）替换为「生成旅程边界声明」四条——裁出依据（生成线裁定链接）+12 月展演「局外成品」口径（录播放生成结果、不现场跑引擎）+生成体验候 332 设计定稿后 v2.5/v3 入口+AI-first 衔接（feature-freeze 11 月底/12 月初录播/新面就绪录播入口改走新面，裁定纪律三条全文引用）；④四节诚实边界原第 4 条「生成质量随机性」随旅程 B 裁出移除（留一行移除说明）——该条系旅程 B 附属悬空条款，属裁出直接后果，非 L1 布位段（布位段零触碰）。
- **grep 亲证**（验收①②）：`grep -n "YuE2" / "providers" / "genapi" / "VIT_GENAPI" / "8087" / "yue-server"` 对清单文件全部 0 hits（grep exit 1=无匹配）；边界声明在卷：`grep -n "局外成品|AI-first|feature-freeze|v2.5/v3|裁出"` 命中标题+三节四条+四节移除说明。残留 "env" 仅 2 处且均非生成服务前置：line 11=LEGAME-DIR-1「环境前置=零」布位记录（L1 布位段，未触碰），line 14=重铺命令 `$env:APPDATA` PowerShell 自动变量语法——「无手动服务前置残留」成立。
- **命令与退出码**：上述 grep 全部亲跑；`git push origin port/legame-genscope-1` exit 0；worktree `git status --short`=空。
- **提交路线**：清单文件属文档改动，按 PROTOCOL §3（执行侧直推 main 仅限 coord/cards/）impl commit 296c80ba 留 port 分支候验收，主管验收后 cherry-pick/合入 main；本卡面状态变更（doing→done+回执）单独走 main。
- **领取过程补记**：首推 449fd3a8（base=f12635af）被拒 non-ff（同刻 332 领取提交 73e1b166 落 main）→fetch 重读远端（333 仍 todo；332 域=coord/reports+其卡面，无交；doing 三卡域核对无冲突）→重应用为 fd31c879 推送成功并复核远端 owner——§2.1.3 全程在案。
- **零栈零 LLM**：无真栈 run、无 run ID、无计费、无工件目录需求（§6.1 不适用；本卡无 runs 工件）。

## 验收

- （主管回填）
