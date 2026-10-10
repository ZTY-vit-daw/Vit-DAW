# Ruling：2026-10-10 晚窗批量验收——四卡 pass + SMOKE 执行 pass/verdict FAIL 登记（含 G3 误杀缺陷链采信与四后续卡）

- 裁定：M4B/WARN/PROBE-METER/FE-RACK-CTX 四卡 **pass**；FS-LARGEPROJECT-SMOKE-1 **执行 pass + 烟测 verdict FAIL 正式登记**（烟测完成使命=发现真缺陷）。cherry-pick `a62f7125→21b70b54`、`9ceb62ea→6d31fb4f`、`12792d0a→0a7797f9`、`2291d055→4ccb02dd`；前端仓 924318d 合入活线 port/vitnote-region-time-1 并推 origin。
- 四层亲核：①diff 亲读（四卡全量）；②我方复跑——合并态 main `go build` exit 0+定向三包 ok+**全量 92 包 0 FAIL**；前端探针**独立复跑 PASS 76/76 exit 0**（主 worktree 环境勘测记录按回执复用）；③工件核对（SMOKE 四 run+REVIEW-independent.md 六节全读；FE RECEIPT+probe log×2）；④锚点核对（M4B 三前缀锚点/PROBE-METER runner 计时点/WARN 降级分支）。

## 1. REFSCHEMA-M4B：pass

三词条注册与重裁逐款一致（identity→snapshot 对齐 ccbobs_、族内包含对最长匹配测试钉住）；钉住测试翻面+初值表 19→22；G1 记录 ccbr_ 缺口闭合回写核实。**G1 迁移账 capabilitycontext 批次全数完结**。

## 2. HYGIENE-GOALPERSIST-WARN-1：pass

nil 守卫 WARN 一行（error=%v 全文含租约 owner/expires）；纯日志加法；BOUNDARY-PERSIST 观察项 A 销项。

## 3. PULL-PROBE-METER-1：pass + 分级裁定

- 实现：execRecord `elapsed_ms`（executeTool 真实执行后构造，未执行无键不填 0）+settleBatch D5 分级（probe 三工具逐笔入账/index 四工具不计不翻/**unlisted 不私定按 unknown 翻 false**——诚实语义保留）。
- **分级裁定（上交项）：`clip.warm_waveform_bake` 归 probe 级**——D5 语义 render/probe 级逐笔计入，波形烘焙=真实渲染物理工作（L2-2 实证其为串行瓶颈，正是成本账户该捕捉的对象）；"warm 预热"属性不改变物理成本事实。落地=PULL-PROBE-TIER-EXT-1 微卡（本 ruling 为其授权依据）。
- probe 计量面自此真实执法（probeCostKnown 在分级表内保持 true）。

## 4. FE-RACK-CTX-PLUGIN-LOAD-1：pass

- 设计三件齐（空白区右键热区/检索分词 AND+前缀>子串排序/落点=右键处+50,50 镜像克隆先例偏移）；停止条件分析正确（`_drop_data` 契约无拖放源耦合，两处非拖放先例：auto_spawn 双击、镜像克隆）——装载链零新写亲验。
- 探针 76 钉（命令面断言到 rack_add_node 命令字典+回归钉：节点菜单/右键平移/拖放线/auto_spawn 零变化）+vitnote 两面回归 PASS；**我方独立复跑 PASS 76/76**。
- 渲染面边界声明采信（真实内核落链/视觉/指针行为归用户手测，清单 6 条已入回执 [等待用户排期]）。
- **前端仓活线事实登记**：集成线=port/vitnote-region-time-1（主工作树检出=用户运行面，924318d 已合入并推 origin）；**origin/main 落后活线 13+ 笔**（停于 9-24 移植时代）——既有卫生问题，登记待裁（归一 main 或正式声明活线为 trunk），不在本 ruling 擅动。

## 5. FS-LARGEPROJECT-SMOKE-1：执行 pass + verdict FAIL 登记

- **执行侧 pass**：纪律全合规——≤2 功能轮用尽即停（断点不同止损未触发：model_protocol_failure@processor_selection vs capability_blocked@准入门，分类分记诚实）；两次工具性中止不计概率预算（无 LLM 轮实证）；E:\ 三 manifest 逐字节一致（§10 红线）；栈占用登记→释放闭环（af0de711）；脚本四迭代的缺陷-修复对应可核。
- **verdict FAIL 正式登记**：A1（pull 遥测面：pullharness 带指纹 3-4 条、prefix 17827B 四轮稳定）pass；A4（零固定编排）pass；A2（站图）/A3（AB 往返）fail。
- **高价值发现采信（独立复核修正版归因）**：61 轨大工程上，模型**已请求** mix.multitrack_relationship（full_project scope）且**已部分交付**（28.5KB MOM 事实，status=partial；仅 frequency_relationship 被披露预算裁掉）——但**终态台账只剩 track 级一张收据**（receipt_count=1），mix 回执未存活到准入评估；fs_loop cycle=0 与 ≥4 个模型轮矛盾。→ **跨轮台账持久化/循环激活时序缺陷**，准入门在合法已交付全曲 scan 收据在场时仍判 capability_blocked（**G3 误杀**；法条 free_state_gate.go:137/:139-146/:185-215 freeStateReceiptUsable 接受 partial——若收据存活 G3 本应 pass）。
- 次级发现：receipt project_revision 窄回退（:738，宽链 :1865-1871 在位未用，不改门结果）；blocked 面未接 surface（stop_reason=done/goal_status=completed 与 fs_loop capability_blocked 并存——FS-CAPABILITY-BLOCKED-SURFACE-1 边界在 fs2_capacity_assessed 阶段未接合）；61 轨 bake 预热实测 30s 就绪（修正 L2-2 数十分钟预期——pcverify1+自有 bake 干扰形态）。
- §9 面缺口如实登记：command_line 空串（$MyInvocation 限制）/退出码未数值化/agent_binary 死字段/run 184500 因 $pid 崩溃无 run_report（RUN_NOTE+console 替代）——归 SMOKE-SCRIPT-HYGIENE-1。
- **结论定级：汇合点 blocker=台账持久化缺陷（FS-LEDGER-PERSIST-1，P1）**——修复并复跑 SMOKE 通过前，编曲/生成/混音三线不启动（用户裁定汇合点启动条件=烟测 PASS 维持有效）。

## 6. 后续卡四张（随本 ruling 立卡）

1. **FS-LEDGER-PERSIST-1**（P1）：台账跨轮持久化/cycle 计数缺陷取证+修复——汇合点 blocker。
2. **SMOKE-SCRIPT-HYGIENE-1**（P3）：脚本分类阶梯+A4 标记补+run_report 字段。
3. **FS-RECEIPT-REVISION-1**（P3）：receipt project_revision 宽回退链接入。
4. **PULL-PROBE-TIER-EXT-1**（P3）：clip.warm_waveform_bake 归 probe 级扩表（本 ruling §3 授权）。
