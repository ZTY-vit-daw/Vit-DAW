# Ruling：PORT-PLUGIN-SELECT-MAC-1 pass（2026-09-23）

- 卡：`coord/cards/done/2026-09-23-PORT-PLUGIN-SELECT-MAC-1.md`（PC 决策侧开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：领取 `189f77d`（含 4b758fe 入库后 claim 重放处置）/ 回执 `6da0f33`；零仓库代码改动（机器本地白名单+工件）
- 裁定：**pass**——**LLM 插件自选（用户设计的候选选择工作流）在 mac 自由态路径复活实证**

## 决策侧核验（独立复算）

1. **v6 白名单**：`~/.vit/free_state_experiment_plugins.json` 实核=v6、static_eq 三候选数组（Q10 Stereo 10 带/API-550A 3 带/API-560 10 带）、其余五族 v5 单对象原样、sha256 `38176f05…` 与回执一致；v5 备份 `2b893323…` 在工件目录（演示安全网）。**EMO-F2 如实不入列成立**：探测实证其参数面为 Link Mode+Hpf/Lpf On/Freq×L/R 纯滤波器（零增益参数），v6 条目 gain_param_id 无诚实值——零手编纪律正确执行，候选 3 个满足验证需求。
2. **overlay**：verify_run.log 11 PASS——三候选 membership 各自解析+活 v1 store admission、空 pin 拒绝、非成员 pin fail-closed spot（"admits one of"+点名成员集）、550A/560 nearest-band 与认证锚点吻合、五族 v5 回归、v5 备份过兼容阀。
3. **自选腿五环（run2_20260923-214216 all_green 9/9，逐环原始工件实证）**：
   ①披露：`PLUGIN CANDIDATES` 6 次（prompt_render），三候选并列结构性字段，全工件零 path/param_id 泄漏；
   ②选择：模型 needs_experiment 提案 parameter_bounds 携带 `VST3-Q10 Stereo-10456661-3a8f251e`（提案上下文 172 处 vs 其他候选各 45=披露回显）——**逐字回显=自选**；
   ③准入：三个 membership 拒绝签名全零；
   ④绑定：FrozenPlan args `plugin_identifier=Q10`+param 12/12（120Hz 假设→Q10 125Hz 带）；
   ⑤写链：rack_item_id 42 处+WavesVST3UI Q10 component_create success（CID 精确）+actual_readback_value −1+revision 3→4+audition.ready。
4. **§8**：两轮账目合规（run1 前提案轮预算耗尽=模型随机分支，非准入防御签名非断言红，加跑正当）；LLM key 零泄漏扫描过。
5. **瑕疵（不构成返工）**：回执自带 evidence_chain.json 的 load_write_chain 计数器全零=抽取器自身搜索路径缺陷（原始工件齐全，结论不受影响）——已在本 ruling 记录，工件侧如复用该脚本需修正。

## 双端收口与演示面抉择

- 用户设计的"LLM 从候选集自选"工作流历经：AGENT-1 设计→迁移停摆（legacy 默认）→今晚 FIX-PLUGIN-SELECT-1（v6+membership+披露，PC）+本卡（mac 四候选实证腿）——**双端复活闭环**。
- mac 白名单现为 v6 三候选：聊天 EQ 请求将出现候选披露+模型自选（含选择说明）。**演示面抉择交用户**：v6=演示"模型自选"（更强的第二幕素材）；还原 v5 备份=单 Q10 简单形态。v5→v6 兼容阀保证单候选族行为逐字节不变。

## 遗留移交

- FIX-KERNEL-PLUGINLIST-HYGIENE-1（P2，todo）不变；FORENSIC-MAC-CLARIFY-CHAIN-1（P1）在队。
- 决策侧流程备忘（本次随验收一并补入）：FIX-FE-SCANPATH-MAC-1 卡验收回填漏暂存（与 PC 1d065b9 同款暂存时序事故，第三次出现）——**git mv 后必须 re-add 该文件再提交**，后续验收执行。
