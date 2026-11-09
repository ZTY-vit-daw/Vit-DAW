# FIX-FREEFLOAT-EQ-ANCHOR-1 类缺口登记（static_eq 带锚来源）

- 登记时间：2026-11-09（晚窗）/ 登记人：328 执行侧（ZCode GLM-5.3 L2 执行子代理）
- 证据基线：本卡 derive 重跑工件（eq_static_eq_derived.json / provenance_table_v6_full.json，run 20260924_005622，328 重跑）+ 收据原件 ~/.vit/pca_certifications/eq_channel_20260924_005622/
- 规则基线：冻结派生规则（scripts/pca_autosweep.py:1249-1253「promoted ∧ upsert bell coverage ∧ derivable band anchors」）+ 322 卡内修授权的收据自证单带锚通道（8c0ef051，仅作用于 eq_model=free_floating）

## 一、已收口（背景，非缺口）

free_floating EQ 默认态全 Unused（certify 装载即 explain、先于任何编辑 → active_bands 必空）类，在收据自带 bell upsert verified writes 时已有带锚来源——收据自证单带锚：apply.json verified upsert writes（role=freq 的物理 Hz + role=gain 的 param_id）× explain 带行交叉 × S3 双周期 stable_id 零漂移。

- Pro-Q 3：322 落盘（锚 {3400.0, "3", "3"}）。
- TBTECH Kirchhoff-EQ：328 本卡落盘（锚 {3400.0, "55578", "55578"}，逐字取自 328 重跑派生输出）。

## 二、类缺口 A（卡面具名）：bx_tonebox —— 无收据自证，诚实排除维持

冻结规则下三重不满足（收据+explain 双侧证据）：

1. **收据无 bell 覆盖**：receipt upsert shapes={'low_cut'}（apply.json 唯一 upsert 为 low_cut；准入要求 upsert+bell）；
2. **收据无 gain 写**：verified writes 仅 role=freq（param_id 1295003716，80 Hz），无任何 role=gain 写——收据自证通道的必要件（freq 物理 Hz + gain pid）缺半；
3. **explain 无带行**：eq_band_summary eq_model=fixed_slot_adjustable，bands=0、active_bands=0——即便规则扩容，带行交叉也无对象。

→ derive 诚实排除「no derivable band anchors」（328 重跑 excluded 行在卷），维持不落盘。**解锁路径（候产品小卡，非本卡）**：重新认证使其收据带 bell upsert verified writes（freq+gain 全），或规则扩容卡（scripts/pca_autosweep.py 域）；在此之前不硬造锚。

## 三、类缺口 B（一般化）：默认态无可锚带行的 EQ，收据又不自证时，带锚来源为零

**缺口定义**：static_eq 白名单条目的带锚来源现仅有两条——① explain 拓扑（active/band 行的数值 center + gain 绑定 × S3 双周期）②收据自证（free_floating 域，8c0ef051）。当 EQ 默认态 explain 不产出可锚带行（全 Unused 或 bands 空或频率无数值域）**且**收据不带可自证锚（无 bell upsert / 无 gain 写）时，不存在第三来源；白名单 fail-closed（experimentplugins Load 硬校验 static_eq 每条 ≥1 band，agent/internal/experimentplugins/whitelist.go:628）+ 治理禁硬造 ⇒ 此类 EQ 永远进不了白名单，直到重新认证或规则扩容。

**当前在卷实例（328 重跑 excluded，全部诚实排除在案）**：

| 主体 | eq_model | 排除理由 | 缺口侧 |
|---|---|---|---|
| bx_tonebox | fixed_slot_adjustable（bands=0） | no derivable band anchors | 收据无 bell 无 gain 写 + explain 无带行（类 A） |
| EMO-Q4 Stereo | fixed_slot_adjustable | band B1–B4 lacks center/gain anchor | 4 带行 current_freq_hz/fixed_freq_hz 全 None（无数值中心），gain_bindings 在场但无中心可锚 |
| Lindell 80 Channel | fixed_slot_adjustable | band BHIGH/BLOW/BMID lacks center/gain anchor | 频率域 scale=enum（switch 型频率），无数值 Hz 中心 |

注：EMO-Q4/Lindell 属相邻子类——带行在场但频率无数值域；其收据侧同样未走 bell 自证（fixed_slot 不在 8c0ef051 通道作用域）。三者共同点：**收据外无锚、收据内不自证**，与卡面「默认态全 Unused 的 EQ 无收据外带锚来源」类缺口同族。

## 四、白名单现状（328 apply 后）

static_eq 50 条（48 原样 + Pro-Q 3 + TBTECH Kirchhoff-EQ，两枚收据自证行）；reverb/delay 段原样（1/1）。Go 加载门亲证 TestTBTECHLiveWhitelistLoad PASS（50 条全量过严格解码+校验）。
