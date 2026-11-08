# PLUGIN-CONTROL-EVAL-BR1-1：B-R1 显式薄提示变体（裸面+检索+「告知可查」——主动性问题对照）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[312 终验裁定①](../done/2026-11-08-PLUGIN-CONTROL-EVAL-S1B-1.md)——B 零检索执行=知识转化未测及，首要缺口=主动性；[评估计划薄提示词策略](../../reports/2026-11-07-plugin-control-model-evaluation-plan.md)在案待验证方向；用户裁定实验线最高优先级连续推进）
- 派发确认：已确认（主管发卡·变体对照段）；验收负责人：GLM 主管决策流
- 池序 317；目标仓库=主仓 scripts/工件（复用 312 装置）±（如需）最小提示注入缝；P1 / 0.5 天 / 依赖=312 工件在库（已合入 main f84d6a62）；L2 / PC；真栈占用（领取即占——当前空闲）

## 目标

1. **B-R1 18 轮**：B 臂同款装置（S1-S5+S2b 裸面+检索直执缝）**+ 显式薄提示一段**注入模型系统提示——内容=「你有 web.search/web.fetch 可查插件手册与参数文档；遇到不确定的参数编码（param_id/量纲/取值范围）先查再写，查过的不重复查」。提示文本进 RUNBOOK 冻结；其余口径（cases/判据/T4 标注/caps）与 312 逐字同；
2. **注入机制勘察**：312 的 arm_b_driver 包装器可加提示 preamble 则零源码；若需 agent 侧注入，授权最小缝（同 312 检索缝模式：env 键+默认 unset 字节不变+配对单测）；
3. **装置门**：312 七面复刻+**R1 亲证**（提示段在模型首请求 system/prompt 面在场——payload capture 断言）；检索执行记录逐轮在卷（本轮核心观测量）；
4. **回报（实验线核心问题收口）**：
   - 若 R1 检索被发起且写入类转化提升 → **知识缺口真实且可检索补**（产品结论=薄提示词策略可行，typed 适配器之外存在轻量路径）；
   - 若 R1 检索被发起但写入仍败 → 知识非唯一缺口（编码/推理层问题），typed 承重结论加固；
   - 若 R1 仍零发起 → 主动性问题非提示可解（模型倾向问题，产品须结构性引导）。
   C 段 go/no-go 随本结果定（312 终验裁定①）。

## 约束

- 文件域=工件目录+卡面+编排/驱动包装器±（如需）提示注入缝+测试；源码零改动红线（除授权缝）；真栈协议全链；检索 attempts/URLs/hashes 逐轮记录。

## 停止条件

缝泄露任何形态→装置失效停段；同型环境失败两轮→止损；断言失败取证不重跑。

## 预算

≈¥2-4 上界（312 同量级）；§8 跑前成文。

## 领取/回执/验收

- 领取：2026-11-08 21:02 / origin/main=3a0a3ab0（≥f84d6a62 含 312 装置与缝；worktree 基线干净：status 零行、diff --stat 空）/ owner=GLM-5.3（ZCode）L2 执行子代理·PC·本会话 / 分支=port/plugin-control-eval-br1-1 / worktree=D:/Vit_DAW_wt_317br1 / 领取提交=coord/317-claim（卡面随领取首次入库 doing/，与 312 先例同）
- 回执：commit 31169e6a @ port/plugin-control-eval-br1-1（工件+缝+测试全量在分支；报告=coord/runs/PLUGIN-CONTROL-EVAL-BR1-1/RUNBOOK.md+arm_br1_20261108_211559/arm_br1_summary.json）
  - **成绩**：18/18 评估，exit 0；逐案 B-R1=T2 3/3、T5 3/3、T6 3/3、T1 0/3（value_encoding_error）、T3 0/3（model_task_failure）、T4 0/3（value_encoding_error）=3/6 案例级，与 B(312)/A-pure(310) 逐案同形；
  - **三态对号=态三（仍零发起）**：显式薄提示段注入系统提示（R1P 亲证：语料 63/63 message_loop 请求段在 system 面+probe 轮 9/9+semantic_entry/repair 零污染=分类器输入与 B 逐字节一致）后，18 轮零检索发起——6 个全目录可见轮（段+web 对目录行俱在）全未查（no_query_initiated_despite_disclosure_and_prompt），12 包路由轮同零；写入类失败标签与 B 完全一致（T1/T4 value_encoding_error、T3 model_task_failure）→**主动性问题非提示可解；产品结论=结构性引导（路由披露/装载时注入/适配器形态），C 段按 312 裁定①随本结果定（R1 仍败→C 降级或改判候主管）**；
  - **五方对照**（arm_br1_summary.json five_way_comparison）：B-R1 3/6 = B 3/6 = A-pure 3/6 < A307 5/6 ≈ P 5/6（P 单 repeat）；检索行为=五方全零发起（A307/A-pure/B/B-R1）；
  - **检索账本**：18 轮 attempts=0/executed=0/browser 族=0；可见性分层 6 full-visible+12 pack-routed 全零；检索污染观察表空；无 cap 超限（T2_r3 段内 llm_calls_over_cap=预算停车非检索面）；
  - **装置门**：活体检索 probe PASS（web.search 3 结果+web.fetch HTTP 200+零确认停车）；probe 轮+语料级 seam_verify_br1 八面全 PASS（S2/S2b/S3/S4/S5/CS/S1+typed 零+R1/R1c+R1P）；selftest 17/17；312 B 语料交叉验证=七面零失败仅 R1P 命中（增量恰为段）；typed 执行零；装置零失效；
  - **预算**：95 任务+13 probe LLM 调用/1,193,705+233,819 in/52,850+8,991 out tokens——caps（216/8.64M/1.728M）内；cost=missing（价目未核）；估 ≈¥2-3 区间（312 同量级 118 调用/1.78M in）；
  - **栈协议**：占用 b40049b1（净空亲核 21:15:11 全零+≤60s 复核 21:15:44）→启栈→run→owner 亲拆 TEARDOWN_ALL_ZERO 21:28:14→释放 62e430d0；登记链接=coord/resources/PC-RUNTIME-STACK.md 头两条；
  - **端测边界声明**：本卡为实验腿（agent HTTP/ZMQ 真栈 18 任务轮+活体 probe），不涉渲染面与用户旅程——无 E2E-WEBUI/JOURNEY 覆盖主张；agent 侧源码改动=授权薄提示缝单点（message_loop.go+配对单测），默认 unset 字节不变（四包回归绿），生产启动路径永不设置；
  - **注入机制勘察结论**（RUNBOOK 冻结节）：wrapper 层系统提示注入无外部通道=不可行；用户消息 prepend 会污染语义入口分类器输入（312 裁定：路由决定可见性）破坏单变量设计故否决；采最小缝（312 模式）。
- 验收：（主管回填）

### 终验（主管，2026-11-08 深夜）

- **pass 整卡收口+缝合入 main**——五方对照亲证（BR1=B=A-pure 逐案同形 3/6；写入失败标签与 B 逐字一致）；retrieval_analysis 亲读（0/18 发起+0/18 尝试；6 全目录可见轮+提示段俱在仍零查）；R1P 第八面亲证链合格（63/63 system 面在+semantic_entry 零段=分类器输入与 B 逐字节一致=单变量设计成立）；注入机制否决链合理（wrapper 无通道/用户消息 prepend 污染分类器→最小缝正解）；装置门八面+活体 probe PASS；栈协议全链（b40049b1→62e430d0）；预算 108 调用在 caps 内。
- **三态裁定=态三确认**：主动性问题非提示可解（可见+可用+被告知三层递进仍零发起）。
- **C 段改判=不跑原案**（312 裁定①兑现）：信息量已被 P≈A（语义面等值 5/6）+B/B-R1（检索死腿 0/18×2）覆盖，预期 C≈P≈A；实验线 S1 段就此收口，结论落 [评估线结论档](../../rulings/2026-11-08-plugin-control-eval-line-conclusion.md)。
