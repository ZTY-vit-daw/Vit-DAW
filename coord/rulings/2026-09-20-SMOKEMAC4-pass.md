# Ruling：PORT-SMOKE-MAC-4 pass（2026-09-20，交付面；④⑤ 处置上交用户）

- 卡：`coord/cards/done/2026-09-20-PORT-SMOKE-MAC-4-final-statistics.md`
- 实现：`port/smoke-mac-4` 三 commit（⑥ bridge provenance 分族收窄 8650a22 / ⑦ step7 前停 kernel 编排修复 ce495c9 / SMOKE_TESTS 统计口径 80a77f8）→ cherry-pick 入 main（bcfceeb/3217028/d180bb9）
- 裁定：**pass（交付面）**——统计任务按预声明口径完成、诚实上报、停止条件严格执行（④⑤ <4/5 即停，未加跑凑数）

## 决策侧核验与记分

1. **达标件**：⑥ **5/5**（含 bridge provenance 分族新口径）、⑦ **4/5**（全量不带 skip，product-path 步携双跳断言绿轮在案；环境中断 1 例带证据排除）。两处脚本修补（红证驱动、逐处列明、强度不降）采认。
2. **不达标件**：④ **2/5**、⑤ **0/5**（有效轮）——8 个有效红轮**全部**属"LLM 诚实终止族"（能力边界声明/纯观察完成/无可采纳决策/limit_reached），断言面本身可满足（④ 绿轮双跳链全绿证明链路通）；⑤ 另有复合红 1（合法 static_eq 提案 × 机器白名单未配置 → hop-2 d1_execution_blocked）。
3. **引擎口径**：执行侧发现 config 仍为 pro（mtime 09-20 11:51）——**系决策侧 09-20 报告称"已切回 flash"但实际未执行恢复命令**（本 ruling 认账：报告与事实不符，执行侧 env 逐轮恢复口径+遥测 model 字段实证全 flash 的处置正确）。**现已完成真实恢复**（defaultModel=deepseek-v4-flash 复核）。
4. **§8 账目**：23 跑/20 有效轮/失败归类全覆盖/环境中断 2 例带原始证据——合格。

## 上交三项裁定

- **① ④⑤ 处置**（上交用户，三选项）：a=容忍口径修订（链路已证+诚实终止方差入档收官）/ b=pro 引擎对照实验（⑤×5，先验：pro 在 ④ 的 3/3 协议合规记录 vs flash 诚实终止族；测长上下文协议维度——短探针电池测不到的维度）/ c=agent 工作流强化（另开大卡）。**决策侧推荐 b→再定 a**。
- **② static_eq 白名单**：`~/.vit/free_state_experiment_plugins.json` mac 未配置——开迷你卡：以 C2 白名单草稿（24 主体）为源做 mac 机器采纳（pluginprobe 实测口径，不手编）。
- **③ doing 滞留卡**：已处置（13b13b2 删陈旧副本归位；49eb7de 整理失败的"destination exists"根因入档）。

## 记分板（23 件套件）

**达标 17 件**（15 全绿 + ⑥ 5/5 + ⑦ 4/5）+ ④⑤ 待处置 + ①-02/04 已知错配在案。**端测收官声明暂缓**，待 ④⑤ 处置裁定。另：⑥ bridge 断言 ps1 同款错配待双端同步（PS1 sync backlog）。
