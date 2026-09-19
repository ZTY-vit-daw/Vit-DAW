# Ruling：PORT-SMOKE-MAC-2 pass（2026-09-20，交付面；四项上交裁定）

- 卡：`coord/cards/done/2026-09-19-PORT-SMOKE-MAC-2-experiment-chain-and-reruns.md`
- 实现：`port/smoke-mac-2` @`e6c493b`（8 文件 +583/−20：dad_probe POSIX 读取器 + 5 脚本退出码卫生 + ⑥ settle 合成修复 + ⑤ per-track ready 语义修复 + py 组驱动 + SMOKE_TESTS）→ cherry-pick 入 main `9be5711`
- 裁定：**pass（交付面）**——五段全执行、绿件真实（run ID 逐段在账）、§8 纪律严格（④守住 1 轮授权上限、⑥⑤止损合规）、红件全部根因定性且分属两族（见下）

## 决策侧核验

RUN_LEDGER 逐段回指核对：①01/03/05 绿、②③复跑绿（升级断言工作：VIT1 magic/queued-run）、D 真栈双验绿（DAD L3 + L2 render probe——`//VAF` 前缀陷阱与 ctypes 变参 ABI 两个开发锚点在案）、⑦前六步全绿（D+E 双修复均获运行验证）、④提取器修复证实有效（取到模型真实终态回复）。②③ run1 的 exit 143 系 5 脚本共构的 trap/errexit 卫生缺陷（已修）——SMOKE-MAC-1 后首次进绿路径才暴露，非功能败，分类正确。

## 四项上交裁定

1. **⑥/④/⑦-product-path 步（LLM 语义族）**：与 PS1-SETTLE-1 的 F⑤ 同族——当前 deepseek 单引擎的终态行为（能力边界声明 / no admissible final decision / needs_experiment 被准入门拒）不匹配脚本期望的确认路径；⑥ run1 已证升级断言与 CCB 路由工作正常、settle 合成缺陷已修（待绿轮检验）。**裁定：随强引擎解决**（与 SETTLE-2 同一前置=用户配置 `~/.vit/config.json` 强引擎）；引擎就位后开 mac 复跑小卡（⑥④⑦ 三面各授权复跑，settle 修复同获检验）。④ 本轮红不计驱动缺陷。
2. **⑤（断言错配族）**：kernel-prepared/feature_snapshot 断言块**双端从未真正执行过**（SMOKE-MAC-1 被 mom_version 挡前、PC 绿轮走 else 分支）——快照行质量完好，是断言语义陈旧（全称 ready vs 实际 per-request 轨迹行、DAD L3 行 provenance 族无 request_id）。**裁定：开 PORT-PS1-SYNC-2**——双端静态审读后按实际 schema 一次修齐（per-track ready 已先行验证推进），ps1+`_mac.sh` 同步，PC 跑绿后 mac 复跑（可并入强引擎复跑卡）。
3. **①-02**：代码级确定——单发 B1.2 被容量评估路由到刻意未注册的治理控制器（`orchestration_controller_host.go:127`）→ 诚实 capability_unavailable；03 已证同语义链式 context 走通。**裁定：记为已知会话设计错配**（PC 同版必红），B1.2 语义覆盖由 03 链式承担；不强凑绿，prompt 改造另案可选。
4. **①-04**：裸代号 prompt 稳定 unresolved（semantic_entry waiting_clarification），fixture 构造面成功。**裁定：同 02 记为已知错配**，prompt 富化另案可选。

## 记分板（端测时代 · mac 侧）

全绿 10 件：readonly / A5 / C2 校准链 / journey1 / ab_result / preflight② / stems③ / b1_group_reset / b1_3_full / b4_low_end。⑦ 六步绿（差 product-path 步）；⑥④ 待强引擎；⑤ 待 PS1-SYNC-2；①02/04 已知错配在案。VSP 五件套（三件套口径）待 SMOKE-MAC-3。
