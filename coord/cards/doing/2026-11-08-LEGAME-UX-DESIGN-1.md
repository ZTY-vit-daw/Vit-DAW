# LEGAME-UX-DESIGN-1：乐局产品式体验设计（自动开场+onboarding+personal-agent 形态分界）

- 发卡：GLM 主管决策流 / 2026-11-08 晚窗（依据=[用户裁定 2026-11-08](../../decisions/2026-11-08-product-style-handtest-ruling.md)——产品式手测标准+乐局产品形态方向+「进入即自动开场」+命名/人格环节追问）
- 派发确认：**待用户**（设计卡先出分层方案与分界裁定请求，用户拍板后拆实现卡）；验收负责人：GLM 主管决策流
- 池序 313；目标仓库=设计文档（coord/）为主，实现拆卡另发；P1 / 设计 0.5 天；L1-L2 / PC；零栈

## 目标（设计先行，零实现）

1. **L1 即修件设计**（产品式手测 v2 的硬前置，逐项给方案+改动面+验收口径）：
   - a. **乐局目录产品级解析**：去 `VIT_EDU_LEVELS_DIR` env 依赖——config 文件键/安装布局解析/与前端启动链路对齐（前端拉起 agent 即可达）；
   - b. **进入工程自动开场**：工程挂接乐局卡→agent 会话就绪即自动发开场叙事（无需「开始乐局」命令词）——与横切状态机挂载点（281）的接线形态、幂等/重入语义；
   - c. **fresh 工程保证**：手测工程每次交付前重铺干净态（布位器+预演副本制）；
   - d. **生成引擎产品化前置**：YuE2 生命周期（自动启动/健康检查/失败引导）或 v2 手测先裁出范围。
2. **L2 最小 onboarding 设计**（命名/人格制定环节的最小形）：首次进入→命名你的搭档（{model_name}/{user_name} 落库+persona 注线消费方）+人格预设 2-3 选（prompt_block 级）——GUI 形态（启动页卡片/首次进入对话框）+与 persona_slot/ruleset v3 注线（282 后续小卡合流）；
3. **L3 personal-agent 完整壳形态**（Grok-bot 式对话壳/人格包系统/人格共塑）：按 rulings §五-19 归产品线的既定分界复核，给出「12 月展演最小形 vs 产品线完整形」对照表；
4. **产出**：design doc（coord/reports/ 或 levels 目录）+裁定请求清单（L2 是否进 12 月窗口/L1 落地时点/d 裁出范围）交用户。

## 约束

- 零实现零栈纯设计；不触在飞实验线（312 占栈中）任何域；
- L1 各项须给「改动面文件域+验收口径（含真栈腿）」为拆卡做准备。

## 停止条件

设计发现自动开场与既有语义入口/横切拦截语义冲突→取证列冲突面交裁定。

## 领取/回执/验收

### 领取

- 2026-11-08 深夜段 / origin/main `2dbb2cbd` / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 313legame）/ 分支 `port/legame-ux-design-1` / worktree `D:/Vit_DAW_wt_313legame`
- 领取基线：HEAD=`2dbb2cbd`（=origin/main，净树建枝）；`git status --short` 空；`git diff --stat` 空
- 派发依据：卡面「待用户」已由 [decisions/2026-11-08 §六](../../decisions/2026-11-08-product-style-handtest-ruling.md) 收口——「乐局产品体验设计回到上一版定稿……313 设计腿据此派发出正式稿」；用户已与主管完成三轮设计讨论（§一/§五/§六），任务指令=按定稿方向出正式稿+拆卡清单
- 在飞域核对：doing 五卡（EDU-ASK-ANCHOR/ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-OFFRENDER-FIX/PLUGIN-CONTROL-EVAL-S1B）文件域均不在 coord/reports/ 与本卡面——不相交；312 占栈中，本卡零栈零冲突
- 文件域：`coord/reports/2026-11-08-legame-ux-design-v1.md`（新建）+ 本卡面；零代码零栈

- （回执执行侧回填）/（验收主管回填）
