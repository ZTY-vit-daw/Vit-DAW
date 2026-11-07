# PLUGIN-CONTROL-EVAL-S1A-1：S1-A 臂执行（裸参数 18 轮，缝开）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[300 P 基线 5/6](../done/2026-11-07-PLUGIN-CONTROL-EVAL-RUNNER-1.md)+[305 四缝合入](../done/2026-11-07-PLUGIN-CONTROL-EVAL-SEAM-1.md)+冻结 plan.json segment_caps——P 臂 go 条件已过，下一分段=A 臂）
- 派发确认：已确认（主管发卡·分段 go/no-go 第 2 段）；验收负责人：GLM 主管决策流
- 池序 307；目标仓库=主仓 scripts（复用 300 runner）+工件；P2 / 0.5 天 / 依赖=305 已合入；L2 / PC；真栈占用（登记协议照旧）

## 目标

1. **A 臂 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1` 缝开（S1-S4 全生效）——模型只见裸参数面（原生 param_id/name/unit/min/max/value/value_text）+通用读写回读工具+原生枚举，无 typed 接管/无语义字段/无语义工具描述/无 typed 路由；
2. **A 臂口径声明项**（305 验收裁定）：quick_controls 选择面+recommended_groups 组名=最小结构残留，结果解读时声明；B 臂检索（web.search/fetch）**本段不开**（A=不联网）；
3. 执行与评价：300 driver 的 round/segment 面逐轮跑（每轮新会话+同初始态+预算计数）；独立评价器判定（readback+非目标 diff）；失败按 plan.json failure_taxonomy 分类（A 臂特有：`seam_leakage` 若观察到语义字段泄露=装置失效优先报告）；
4. 预算：plan.json A 段 caps（≤18 任务轮；per_round_caps 沿用）；§8 跑前成文；单段 go/no-go——**A 结果落地即回报主管**（B/C 段决策输入）；
5. 回执：18 轮成绩表（case/arm= A/repeat/first_action/final_verdict/failure_tags/readback 工件/mutation_count/tokens/cost/stop_reason）+预算实耗+A vs P 逐案对照表+装置验证（缝开亲证：telemetry/payload capture 无语义字段+路由无 typed 接管）。

## 约束

- 文件域=工件目录+卡面；源码零改动（305 已在 main；发现装置缺陷只取证）；agent 源码只读；
- 真栈协议全链；工件新目录；评价器期望不进模型上下文。

## 停止条件

缝泄露（语义字段入模型上下文）→装置失效立即停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 22:14 +0800 / origin/main=d4bd3496bd07e3e949588eb71d6de9d9a82f8817 / owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 307 s1a runner；发卡人置 doing/ 并声明「卡面回填领取信息推协调提交即视领取」）/ 分支=port/plugin-control-eval-s1a-1 / worktree=D:/Vit_DAW_wt_307s1a / 领取提交=本协调提交（coord/307-claim→main，hash 见 git log）。领取基线：fresh worktree 自 origin/main d4bd3496，`git status --short` 与 `git diff --stat` 均空（clean）。冻结面：plan.json 取自 origin/port/plugin-control-eval-prep-1（A_star 段 caps/口径），runner 三件取自 origin/port/plugin-control-eval-runner-1@ccf8c78c（300 验收 pass 但尚未合入 main——按本卡面「复用 300 runner」快照入本卡 worktree 分支，含 P 工件引用）；缝实现=已合入 main 的 1775c10b（305 验收 pass）。执行侧声明：源码零改动（agent/ 与 VitApp/ 只读；300 runner 三件原样快照不加改）；工件根=coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/。
- （执行侧回填回执）/（主管回填验收）
