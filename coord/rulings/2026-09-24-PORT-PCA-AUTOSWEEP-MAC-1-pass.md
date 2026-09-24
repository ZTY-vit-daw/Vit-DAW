# Ruling：PORT-PCA-AUTOSWEEP-MAC-1 pass（2026-09-24）

- 卡：`coord/cards/done/2026-09-24-PORT-PCA-AUTOSWEEP-MAC-1.md`（Mac 决策会话开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：领取 `892919e`+`82555fc` / 回执 `4471281`；仓库零代码改动（共享工具原样执行）
- 裁定：**pass**——mac 首轮全库 sweep 闭环：**候选面 23→47（EQ 3→7）**，共享工具的 mac 验证与跨平台确定性证明一并完成

## 决策侧核验（独立复算）

1. **sweep**：run 20260924_161642 七相全过——705 主体 probe 零超时；三桶 classified 185/certified 45/exceptions 118 全记因；七族表 static_eq 53/broadband 18/limiter 29/transient 29/multiband 23/de_esser 16/gate 17。
2. **跨平台确定性（本卡最有价值结论）**：共享 705 主体（mac 全库=PC 970 子集，PC 独有 265=非 Waves 厂商库差）**逐主体族集合分歧 0/705、七族计数逐桶相等**——分类器平台无关性实证；卡面 PC 数字（68/51/…）为全库口径的表观差异被干净化解（PC_COMPARISON.md 三列表）。非停止条件情形，无需取证。
3. **白名单再生长**：23→47 条目实核（EQ 7/broadband 13/limiter 9/multiband 8/de_esser 6/gate 2/transient 2），sha `2407930c…`；四源零手编（frozen builder 原样+EQ 通道收据派生 +4/排除 0）；跑前版备份带哈希；promoted 28→73。
4. **验证腿**：overlay 50 检查零失败（47 成员+3 非成员 spot 拒绝）；journey 两轮分账核——run1 assertions_red=回执如实记录（证据捕获窗口 GET 超时+占位分支，§8 记账正当），**run2 all_green**（披露 45/45 七族零泄漏→自选 Q10→零拒绝→FrozenPlan 绑定→写链 succeeded+确定性探针 C1 comp instance_ready）——决策侧分轮核对 verdict 属实。
5. **纪律**：零仓库代码改动；LLM key 实值+模式双扫 PASS；环境事件（Bash ENOENT）处置记录未影响结果；端测边界如实（HTTP 面+确定性通道+journey 断言，不含 webui 渲染面）。

## 上交项处置

- **static_eq load_gate 47 例**（与 PC 57 例同机制，`server.go:1456` 缺省 source 需补 "http"→gate 生效→admission 主体循环依赖）：归 **FIX-PCA-CERTAUTH-TOKEN-1**（todo 在册）；执行侧自举不动点核验（47 例零主体任一族有晋升，幂等重跑无可吸收）采信——token 入口解锁后该 47 例的认证吸收由决策侧在该卡处置。

## 遗留移交

- mac 候选面 47 就绪（EQ 7 个自选池）；agent 二进制无需重建（零代码改动，白名单运行时读取）。
- 队列：FIX-PCA-CERTAUTH-TOKEN-1（47+57 例解锁）、FIX-KERNEL-PLUGINLIST-HYGIENE-1（双端）、FIX-GD-TELEMETRY-BELL-1、FIX-TEST-CTXSYMLINK-1。
- mac vs PC 候选面：47 vs 58（PC 含 265 非 Waves 主体的库差红利）；共享 Waves 集上双方一致。
