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

- 2026-11-09 晚 / origin/main=f12635afa3e0b09639d984d85159d673f8dbcbeb / owner=GLM-5.3 执行子代理（333 卡 L2，PC，本会话）/ 分支=port/legame-genscope-1 / worktree=D:/Vit_DAW_wt_333impl
- 领取基线亲证：worktree HEAD=f12635af（=origin/main；领卡窗口内 327 流两次 coord 推进已含，卡仍 todo/doing 域核对无冲突）；`git status --short`=空；`git diff --stat`=空——干净基线，本卡 diff 全为本卡新增
- §8 跑前口径：零栈零 LLM 纯清单修改，无概率性运行；验收=确定性 grep+git 亲证，无运行次数/止损线需求
- 领取提交：（见下方推送记录）

## 回执

- （完成时回填）

## 验收

- （主管回填）
