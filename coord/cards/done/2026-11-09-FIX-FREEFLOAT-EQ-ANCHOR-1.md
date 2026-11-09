# FIX-FREEFLOAT-EQ-ANCHOR-1：free_floating EQ 派生带锚类缺口收口（PENDING 分流④，TBTECH withheld 落盘）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 终验裁定①：derive 修复 8c0ef051 使规则统一生效，TBTECH Kirchhoff-EQ 派生行 withheld 不落盘候主管点头；bx_tonebox=类缺口在案）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流；**TBTECH 行落盘=主管预授权**：验证充分即落，无需再候）
- 验收负责人：GLM 主管决策流
- 池序 328；P2 / 0.5 天 / 依赖=322 derive 修复已在位（8c0ef051 已合入）；L1 / PC；**零栈优先**（derive 离线跑）

## 目标

1. **TBTECH 行落盘**：重跑 derive（离线）→ 核对 Kirchhoff-EQ 派生行（收据自证带锚，322 工件在卷）→ 白名单 EQ 段外科 apply（同款纪律：diff 仅增行、双备份、Go 加载门亲证）；
2. **类缺口登记**：bx_tonebox（无收据自证→仍诚实排除）与「默认态全 Unused 的 EQ 无收据外带锚来源」类缺口写成登记段（进白名单 provenance 或 coord 登记，不硬造锚）；
3. 披露面亲证：白名单加载门 PASS + static_eq 段计数更新在卷。

## 文件域

`scripts/pca_autosweep.py` derive 产物 + `~/.vit/free_state_experiment_plugins.json`（先备份）+ 工件 `coord/runs/FIX-FREEFLOAT-EQ-ANCHOR-1/`；零 agent 源码。

## 验收标准

derive exit 0 + 外科 apply diff 仅增 TBTECH 行 + 加载门 PASS + 双备份在案 + 类缺口登记文本在卷。

## 停止条件

重跑 derive 后 TBTECH 行不再派生（规则漂移）→ 停手取证上交，不手改派生输出。

## 领取/回执/验收

- 领取：2026-11-09（晚窗）/ origin/main=31d0b9d1d336d3d2e459c5c018b77c7794a4d8fe（fetch 亲证）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 328 freefloat-eq-anchor，派发已确认+TBTECH 落盘预授权在卡面）/ 执行分支=port/freefloat-eq-anchor-1 @ worktree D:/Vit_DAW_wt_328impl（领取基线 HEAD=31d0b9d1、status 0 行、diff 空，亲证）/ 协调分支=coord/328-claim @ worktree D:/Vit_DAW_wt_328coord（首推被拒于 26c9378a[309 卡面回填]，按 §2.1.3 重读远端无同卡 owner、域外提交，重应用于新 origin/main 后推成）/ 领取提交=21ed32ce（远端 doing/ owner 亲读核对）。依赖亲证：8c0ef051 已在 origin/main（merge-base 祖先亲证）。doing 四卡文件域（EDU-ASK-ANCHOR/ARRLOOP-AUTHBRIDGE/CLOSURE-ROUTE-ADAPT/WAVESHELL-OFFRENDER-FIX）与本卡（scripts derive 产物+~/.vit 白名单+工件目录）不相交。主检出他人未提交改动（新 todo 卡×4+POOL-INDEX.md/CURRENT-STATE.md 等）为卡域外权威输入，不碰。
- 回执：（2026-11-09 18:52 +0800 收口，done=自验完成候主管验收）三目标全兑现，零栈零 LLM 纯确定性（无 run ID、零计费、未登记栈占用——全 198 promoted 主体 .b 快照在位亲证后 derive 全程未起 ProbeHost）：
  - **目标一 TBTECH 行落盘**：
    1. **derive 重跑**：命令=`python scripts/pca_autosweep.py --phase derive --run-id 20260924_005622`（worktree D:/Vit_DAW_wt_328impl，无 --apply-live 预览模式）→ **exit 0**（console=工件 derive_console_20261109_328.log）。§8 跑前口径成文：单 run/成功=exit 0+TBTECH 派生行/失败分类=规则漂移·builder 非零·环境中断/止损=确定性失败不重跑。**停止条件未触发**：eq_static_eq_derived.json derived=[TBTECH Kirchhoff-EQ]（收据自证单带锚 bands=[{center_hz:3400.0, gain_param_id_ch1:"55578", gain_param_id_ch2:"55578"}]，provenance _attestation 行回指收据 eq_channel_20260924_005622/TBTECH_Kirchhoff-EQ/summary.json），**与 322 withheld 行逐字一致（sort_keys JSON 相等亲证=规则无漂移）**；Pro-Q 3 记 _s0_kept（已在 live 原样保留）；excluded=[EMO-Q4 Stereo, Lindell 80 Channel, bx_tonebox] 与 322 完全一致。live/v1/v2 三文件 derive 前后 sha 相等（ac5f03f9/5c88a717/2d2f1806，pre/post 文件在卷）=derive 零副作用亲证；
    2. **外科 apply**：先双备份（~/.vit/free_state_experiment_plugins.json.bak_20261109_tbtechapply + 工件 live_backup_pre_apply.json，前值 sha256=**ac5f03f9e77d614d291918a53480b1ab6818e98991b2f941fec08bc7ff67e72d** 两份相等）→ 二进制保持 CRLF 插入 TBTECH 行（锚逐字取自本卡派生输出，位置=派生 preview 的 static_eq 末位）→ **diff 亲证=仅 1 行 `}`→`},` + 13 行新增（surgical_apply_diff.txt），其他家族字节同**；apply 后 live sha256=**c9fb563c742b2361581a50c3743276d7b42e288cc21795c9e266af0575292350**。诚实披露：首次 apply 误用文本模式写盘致全文件 CRLF→LF（diff 全文件假阳）；即从备份整体还原后二进制重 apply（备份无污染、终态为唯一有效写入，pre/post sha 链在卷）；
    3. **披露面亲证**：throwaway 加载门 **TestTBTECHLiveWhitelistLoad PASS**（agent/internal/experimentplugins/tbtech_live_whitelist_load_test.go，port 分支；真 live 文件过 experimentplugins.Load 严格解码+校验：TBTECH 锚三值断言+Pro-Q 3 行未回退+**static_eq 49→50**+reverb/delay 1/1 原样）；同款命令 `go test ./internal/experimentplugins -run TestTBTECHLiveWhitelistLoad -count=1` exit 0（go_load_gate_test.log）。
  - **目标二 类缺口登记**：class_gap_registration.md（工件目录+port 分支在卷）——bx_tonebox 具名（收据 upsert shapes={low_cut} 无 bell 覆盖+verified writes 仅 role=freq 无 gain 写+explain fixed_slot_adjustable bands=0 无带行可交叉 → 三重不满足，诚实排除维持；解锁=重认证或规则扩容卡）；一般化类缺口（默认态无可锚带行且收据不自证 ⇒ 锚来源为零，白名单 fail-closed whitelist.go:628+禁硬造）；相邻子类 EMO-Q4 Stereo/Lindell 80 Channel（带行在场但频率无数值域——current/fixed_freq_hz 全 None，Lindell freq_domain scale=enum）。登记锚点全部亲读收据/explain 原件，零硬造。
  - **候主管知悉（域外影响，未扩修）**：main 上 322 合入的 TestProQ3LiveWhitelistLoad 硬编码 static_eq=49（proq3_sufficiency_proof_test.go:37），本卡预授权 apply 后计数 50 → 该测试现仅因计数断言 FAIL（Pro-Q 3 锚断言仍过，proq3_322_test_post328.log 在卷）；修复一行属 agent 源码域外，候主管合入时裁定（改 50 或去硬编码）。本卡 throwaway 测试为新增文件（322 先例），卡面「零 agent 源码」按 322 同口径不含 throwaway 取证测试。
  - **工件**：port/freefloat-eq-anchor-1 @ **2dc469d5**（已推远端）——coord/runs/FIX-FREEFLOAT-EQ-ANCHOR-1/ 全套（console/diff/测试日志/双 sha/登记/备份副本）+ FIX-PCA-AUTOSWEEP-1/20260924_005622 derive 产物重生成（git diff 对 origin/main 在卷=规则稳定复核面）；卡面提交=本 doing→done 移动。
  - **端测边界声明**：零栈零 LLM 纯确定性（derive 离线 pluginprobe 未起、加载门为 Go 单测面）；无任务轮数据、零渲染面/用户旅程主张；无计费。
  - **验收裁定后补（2026-11-09 主管验收 pass，遗留补齐）**：proq3_sufficiency_proof_test.go 计数 49→50 随卡后果修复已落在 port/freefloat-eq-anchor-1 @ **df51aa66**（最小 hunk 1 处/2 物理行：行 36 条件 `!=49`→`!=50`+行 37 消息串同步，行 67 的 322 temp-candidate 断言不动——该 fixture 仍 49 且 PASS）；三测试 `go test ./internal/experimentplugins -run 'TestTBTECHLiveWhitelistLoad|TestProQ3LiveWhitelistLoad|TestProQ3SufficiencyProofWhitelistLoad' -count=1` 全 PASS exit 0（three_tests_post_countfix.log 在卷）；分支已推远端不合入 main，候主管合入。
- 验收：（主管回填）
