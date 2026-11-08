# EDU-ASK-ANCHOR-1：问询锚定——transport_set_loop 注册 + 参照物清单引导 + A/B 兜底编排（乐局 4 / 注册链④）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-8/15](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [256 报告](../../reports/2026-11-05-edu-cap-recon-1.md) 点②/④）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 282；**注册链第 ④ 位（前置=278 已验收提交）**；P1 / ~1 天；L1 / PC；烟测腿可搭 281/286 便车
- **前置裁定 15（已下，2026-11-08 用户裁定）：问询协议坐标层+回流层落【通用】模型引导段（方案 a）**——用户理由：所有能力都需要坐标层（坐标层=全能力基础设施）；刹车句「问在判定权属用户处，不问模型自己能查到处」随文生效。引导文本落通用段所在文件（以实际代码结构为准），transport_set_loop 注册仍在本卡 catalog/policy 域。

## 目标

1. **transport_set_loop / transport_clear_loop 注册**（内核现成 CommandDispatcher.cpp:3224/3230，agent 零注册=256 缺口①）：catalog spec（参数 start_seconds/end_seconds）+ policy **RiskDirect**（256 验收已裁：传输态不改工程内容）+ 引导一行。
2. **参照物清单引导**（256 点②矩阵落地）：marker（position/range）/循环区/轨道与 clip 名单/transport 秒/music_context（tempo+拍号+tempo_setting_count 守门语义）/分析命令返回的 bar 坐标——写成"可锚定参照物清单"段+**问询守则**（问句指称必须落在可锚定参照物上；用户侧坐标须追问转换；刹车=问在判定权属用户处，rulings §五-15）。
3. **A/B 兜底编排引导**（256 点④兜底路径）：对话内 A/B 描述+seek+set_loop+play 循环段播放的组合编排次序（"先听 A 段再听 B 段"）；渲染主路径（区间渲染+audition mount）**不在本卡**（留乐局后续/产品线）。

## 约束

- 文件域=catalog.go/policy.go/capability_pack.go（引导段）+测试+（如搭车）烟测脚本参数；零内核；不触 M3 域。

## 验收标准

- 注册后 chat 驱动 loop 设置/清除可用（搭 281 或 286 烟测腿断言，或独立轻腿 exit 0）；
- 引导段=纯文本新增，既有 pack 行为零变化；回执附参照物清单全文+问询守则文本+commit hash。

## 停止条件

- 裁定 15 与默认落点冲突→按裁定重落（文本级改动，不阻塞）。

## 领取

- 2026-11-08 GLM L2-worker（ZCode/PC 子代理）领取。worktree `D:/Vit_DAW_wt_282impl`，分支 `port/edu-ask-anchor-1`（自 origin/main 新建）。
- 领取基线：HEAD `00722e1c68190e33b0c2e298bc3981c309c91413`（=origin/main），`git status --short` 空、`git diff --stat` 空——基线全净。
- 在飞卡域核对：310（工件+scripts，agent 源码只读）/296-R3（scripts）/309（C++ 域）与本卡 agent Go 域（catalog.go/policy.go/capability_pack 体系+测试）不相交；308 已合入 main（00722e1c），catalog.go 域已释放。

## 回执

- （执行侧回填）

## 验收

- （主管回填）
