# Ruling：REFSCHEMA-M2 —— pass（2026-10-08 晚窗，决策侧；首轮裁定+补证返工后终审）

- 执行侧：flash（用户转交）；实现 `ec0b42f4` + 补证 `d4f4facd` @ `port/refschema-m2`（worktree D:/Vit_DAW_wt_m2，base 509e37f9）
- 落位：cherry-pick → main `433a734f`（实现）+ `06d275f6`（补证）；worktree/分支随终审清理

## 终审亲核（首轮四层+返工两层）

### 首轮（2026-10-08 晚窗，实现/兼容/测试面采信）

1. **迁移形态**：三构造器经 momEvidenceRef→FormatRef（scope=数据键族两段、snapshot=observation_id 身份族沿 M1 裁定、t=all+#- 双段显式）；vit:// 文法零改动；mom kind 零注册改动（已在注册表）。停止条件未触发（承载力充足，转义往返有测试）。
2. **消费面清单采信**：13 处迁移 + 保留面实证排除（裸族指针/mix.derive: 族/observation:unbound 身份哨兵/dad 系）+ mom 包外同族点列出备后续（tim/capabilitycontext/mixboard 路由条——legacy 兼容读必留）。
3. **测试**：8 新测我方 worktree 复跑绿（parsed 段断言/INV2 闭环/黄金串/legacy 三态兼容/转义往返/frequency 回退/端到端）。
4. **行为变化申报采信**：空 ObservationID 不再发无身份数据 refs（宁缺勿假，G1 语义正确面）；weakCutRef 回退形态迁移消费面亲核无影响。

### §11 上交与扩域裁定（首轮）

mixboard 预算门饱和（50000 上限 vs HEAD 基线余量 16B+本卡 165 ref×~51B 结构性增长）——决策侧亲验 HEAD 基线通过+采信探针证据，裁定：护栏目的（原始波形泄漏防护）由 forbidden-string 断言独立承载，预算上调不放松防护；授权扩域两行（70000+词汇表同步）。

### 补证返工（d4f4facd，终审）

1. 返工 diff 恰为授权两行（预算 70000+行内注释 / REF_SCHEMA_V1.md §6 三个 scope_kind 数据键族登记——§5 遗留闭合）。
2. 预算门单测我方复跑过；**决策侧全量套件在其 worktree 复跑 exit 0**（90 包 0 FAIL，与执行侧两轮申报一致）。
3. chat TempDir 清理竞态：观察后累计 3 轮未再现（执行侧 2+决策侧 1），维持"首次观察不判 known flaky"（§11：再现即开修复卡）。

## 挂账移交

- mom 包外同族生产点（tim×4 处/capabilitycontext×1 处 mix.read: 字面）备后续 M 系列机会卡；queryengine legacy 路由条保留（旧落盘兼容读）。
- G1 终审记录 M2 行状态更新归下一张 M 系列卡或 gate 时一并回写。
