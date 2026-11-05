# worktree 积压整理报告 — 卡 250 REPO-WORKTREE-CLEANUP-1

- Phase 1 盘点（检查点上交版）｜执行：GLM-5.3 PC L1 执行会话（ZCode）｜2026-11-05
- 领取基线：origin/main=7413a8bf（领取提交 7413a8bf 已推 main 并核对远端 owner）
- 方法声明：本阶段全部为**只读 git/du 命令**，零源码写操作、零真栈占用、零 LLM 成本；主检出工作树未提交改动（领取快照 369 项）全程未触碰。
- 判据：可达性=`git merge-base --is-ancestor <HEAD> origin/main`；干净=`git -C <wt> status --porcelain` 空输出；dirty 文件尺寸为绝对路径逐项 du/stat（首轮相对路径误测已全部重算修正）。

## 1. 总览

- 注册 worktree 共 **142 项**（含主检出），**全部在场**，无 locked、无 prunable。
- HEAD 可达 origin/main：100 项；不可达：42 项。
- 工作树干净：117 项；dirty：25 项（含主检出自身）。

分类（五类互斥，142 项全覆盖）：

| 类 | 定义 | 数量 | 处置建议 |
|---|---|---|---|
| A | 干净 + HEAD 可达 main | 78 | 建议删 worktree + 删已合入分支引用 |
| A₀ | 干净但 HEAD 不可达 main | 27 | worktree 可删（提交仍在分支引用上）；分支引用逐支待裁 |
| B | dirty（非跨流、非硬保留） | 19 | 逐条归档/弃置建议见 §5，待裁定 |
| C | 跨流（codex/claude）/ detached / E 盘演练 | 13 | 列清单不判删（各流确认） |
| 硬保留 | 卡面指定 | 5 | 保留（见 §7） |

## 2. A 类清单（78 项，干净+已合入，建议删）

42 个 coord 流（claim/close 协调树）+ 36 个 port 流（impl/stack 复验树）。分支 tip 均可达 main，`worktree remove` 不需 --force；分支引用可 `-d` 删除。

coord/*（42）：
coord/207-claim=D:/Vit_DAW_wt_207coord；coord/214-claim=wt_214coord；coord/231-claim=wt_231coord；coord/232-claim=wt_232coord；coord/235-close=wt_235impl；coord/236-claim=wt_236coord；coord/237-claim=wt_237coord；coord/238-claim=wt_238coord；coord/240-claim=wt_240coord；coord/241-claim=wt_241claim；coord/242-claim=wt_242claim；coord/243-claim=wt_243claim；coord/244-claim=wt_244coord；coord/245-claim=wt_245coord；coord/246-claim=wt_246claim；coord/247-claim=wt_247claim；coord/authcarry1-claim=wt_authcarry1_coord；coord/bscd1-claim=wt_bscd1_coord；coord/bspill1-done=wt_bspill1_coord；coord/carry1-done=wt_carry1_coord；coord/farc2-claim=wt_farc2_coord；coord/fasc1-stack=wt_fasc1_coord；coord/fe-send-chain-position-1-claim=wt_sendchain1_coord；coord/fe-send-level-click-fix-1-stack=wt_lvlfix1_coord；coord/fe-send-ui-live-chain-1-claim=wt_sendui1claim；coord/fsog1-claim-v2=wt_fsog1_coord；coord/fsrr1-claim=wt_fsrr1_coord；coord/hsnorm1-claim=wt_hsnorm1_coord；coord/klte1-close=wt_klte1_coord；coord/lgrow1-claim=wt_lgrow1_coord；coord/loudnessb-claim=wt_loudnessb_coord；coord/m3-c4-auto-disclose-1-claim=D:/Vit_DAW-coord-m3c4ad1；coord/m3-c4-auto-intent-1-claim=wt_ai1_coord；coord/m3-continuation-park-storm-1-claim=D:/Vit_DAW-coord-248；coord/mixsend-design-claim=wt_mixsend_design；coord/parkdeli1-close=wt_parkdeli1；coord/rfbl-stnd-claim=wt_rfbl_stnd_coord；coord/scopy1-claim=wt_scopy1_coord；coord/sge1-release=wt_sge1_coord；coord/splitdrag1-claim=wt_splitdrag1_coord；coord/startfeed1-b=wt_startfeed1；coord/timefx146-note=wt_timefx146_coord；coord/topoplan154-done=wt_topoplan154_coord；coord/uxab177-claim=wt_uxab177_coord；coord/writegate155-claim=wt_writegate155_coord
（上列路径均省略 `D:/Vit_DAW_wt_` 或 `D:/Vit_DAW` 前缀，按等号左=分支、右=worktree 读）

port/*（36）：
port/fe-autolane-layout-grow-1-stack=wt_lgrow1_stack；port/fe-bridge-spill-ns-1=D:/Vit_DAW_worktrees/fe-bridge-spill-ns；port/fe-interaction-card-kit-1=wt_ick1；port/fe-interaction-card-kit-1-stack=wt_ick1stack；port/fe-lane-catalog-repo-wire-1-stack=wt_207stack；port/fe-lane-descriptor-bulkreply-1-stack=wt_209stack；port/fe-lane-point-click-dead-1-stack=wt_211stack；port/fe-send-level-control-1-stack=wt_sendchain1_stack；port/fs-frontier-observe-gap-1=wt_fsog1；port/kernel-sendroute-impl-1=wt_sendroute1；port/m3-auth-restore-carry-1=wt_authcarry1；port/m3-automation-result-disclose-1=wt_237impl；port/m3-automation-surface-pin-1=wt_241impl；port/m3-breakpoint-forensics-1=wt_244work；port/m3-c4-auto-disclose-1=wt_m3c4ad1；port/m3-c4-auto-intent-1=wt_m3c4ai1；port/m3-c4-auto-llm-route-verify-1=wt_m3c4lr1；port/m3-chat-auth-stamp-1=wt_223impl；port/m3-confirm-driver-align-1=wt_234impl；port/m3-engine-swap-discrim-1=wt_238impl；port/m3-fs-gate-evidence-wire-1=wt_233impl；port/m3-llm-gate-open-1=wt_227；port/m3-llm-neutral-embed-1=D:/Vit_DAW_wt/m3-neutral-embed；port/m3-llm-turn-budget-1=D:/Vit_DAW-229；port/m3-obs-evidence-fixture-1=D:/Vit_DAW-wt-m3obs；port/m3-obs-identity-bind-1=wt_231impl；port/m3-plugin-resolve-candidates-1=wt_236impl；port/region-intent-wire-1=wt_region_intent_wire_1；port/semantic-timefx-softguide-exp-1-ctrl=wt_sge1_ctrl；port/smoke-guidance-check-discrim-1=wt_252impl；port/ux-ab-audition-coverage-1=wt_uxab177；port/vitnote-region-time-1=wt_region_time_1；port/webui-session-semantics-1=wt_sess_sem_1

## 3. A₀ 类清单（27 项，干净但分支未合入）

worktree 本身可安全 remove（提交保存在分支引用上不丢失），但**分支引用必须保留**，是否废弃由主管逐支裁定（多为被 cherry-pick 合入或已废弃的实验线，原 tip 不可达 main）。

| worktree | 分支 | 领先 main 提交数 |
|---|---|---|
| D:/Vit_DAW_jsss1 | port/judgment-settle-stall-1 | 1 |
| D:/Vit_DAW_wt_214stack | port/fe-lane-view-tuning-1-stack | 1 |
| D:/Vit_DAW_wt_aba172_coord | coord/aba172-claim | 1 |
| D:/Vit_DAW_wt_bscd1 | port/budgetstop-chainend-delivery-1 | 2 |
| D:/Vit_DAW_wt_farc2 | port/fullaccess-reply-copy-2 | 1 |
| D:/Vit_DAW_wt_fasc1 | port/fullaccess-semantic-chain-1 | 2 |
| D:/Vit_DAW_wt_fedraft1 | port/fe-start-empty-draft-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_feguic_probe | port/fe-gui-shrink-c-probe-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_fsrr1 | port/fs-receipt-revision-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_fxstartrec1 | port/fix-start-state-recovery-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_hsnorm1 | port/harness-struct-normalize-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_lwfix1 | port/fix-lw-config-persist-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_mastervol1 | port/fix-mastervol-calibration-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_memschema1 | port/mem-schema-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_named1 | port/fs-named-target-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_notestream2 | port/vitnote-notestream-2 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_rendergain1 | port/kernel-rendergain-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_reverb1 | port/pca-reverb-promotion-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_rfbl1 | port/reply-face-boundary-layer-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_scopy1 | port/semantic-card-copy-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_sge1 | port/semantic-timefx-softguide-exp-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_startside1 | port/fix-start-draft-side-effect-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_stnd1 | port/stallsmoke-no-delivery-class-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_testiso1 | port/fix-test-draft-isolation-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_timefx146 | port/semantic-timefx-bus-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_trackfix1 | port/fix-tracklist-shadow-freshness-1 | 未计（同型 1–2） |
| D:/Vit_DAW_wt_webui_ia_1 | port/webui-ia-redesign-1 | 2 |

（"未计（同型 1–2）"=本报告未逐支跑 rev-list；Phase 2 裁定前可按需补计，数据在临时工件 branch_inv.tsv）

## 4. B 类逐条建议（19 项，dirty 待裁定）

判读背景：M3 链 229→251 已全部定案收口（rulings 在库），链上各 impl worktree 的 dirty 文件几乎全是 §6.1 工件纪律的 run 留存件（.vit_derived / agent_prompt_render.jsonl / agent_llm_payload_capture.jsonl / run 工程）。

**建议弃置组（12 项，~60MB payload + klte1 大头 2.7GB）：**

| worktree | dirty 概要（尺寸） | 建议 |
|---|---|---|
| wt_208stack | vspdelta run project/ 1MB | 弃置（卡已验收合入，run 工件无待审依赖） |
| wt_212stack | probe_diag_r6.log 87KB（**tracked M**） | 弃置（诊断日志；唯一 tracked 改动，删除即还原，主管确认 r6 复核结论已在库） |
| wt_232impl | slicebudget_1 工程件 ~1MB | 弃置 |
| wt_240impl | guardrun1-3 prompt_render 3 份 12.5MB + rounds 10MB | 弃置（若主管要留组 2 判别 payload 则单独归档） |
| wt_242impl | agent_prompt_render_local_only.jsonl 9.5MB | 弃置 |
| wt_243impl | .runleft_243/ 7MB | 弃置（残留工作目录） |
| wt_245impl | carryrun1 payload_capture 6.4MB + prompt_render 3.5MB | 弃置 |
| wt_246impl | wordingrun1 payload 3.3MB | 弃置（措辞链已定案结构性不可达） |
| wt_247impl | guardrun1 payload 7.4MB | 弃置 |
| wt_248impl | parkrun1-3 三轮 payload 共 ~41MB | 弃置（链收口报告已在 main） |
| wt_envp1 | envelope_presence_project.vit 5.6KB | 弃置 |
| wt_sendext1_stack | 补丁 fe-send-external-refresh-1_10324d8.patch 43KB + console 4.5KB | 弃置（分支已合入=补丁内容已进 main；若主管要留补丁工件可先拷贝） |

**建议归档组（3 项，删除前先拷贝/入库）：**

| worktree | dirty 概要 | 建议 |
|---|---|---|
| wt_pgtdc_compat | 插件参数抓取 122 个 parameters.json（两轮 run 目录，~0.5MB） | **归档**：是 2026-10-18 reaper 插件参数研究报告的原始 payload（该报告本身尚为未跟踪件）；建议两 run 目录随报告一并入库后再删 |
| wt_carry1 | FS-LEDGER-CARRY-1 六目录 ~41MB（driver×3/kernel_workspace/logs/drafts） | **归档**：分支未合入（ahead=2），payload 仅此一份；入库或压缩留存后再删 worktree，分支另裁 |
| wt_klte1 | VitApp/build-klte1/ **2742MB** 构建产物 + default_project.xml M（2KB） | **build 弃置**（可再生，回收 2.7GB）；xml 改动与分支（kernel-l3-timeenergy-1，ahead=2）交主管裁定 |

**上交主管组（3 项，未合入在途/搁置链，执行侧不建议删）：**

| worktree | 分支（ahead） | dirty 概要 | 上交点 |
|---|---|---|---|
| wt_topoplan154 | port/timefx-bus-topoplan-1（**12**） | 9 个 run 目录共 27 项 ~35MB | 链是否仍有后续卡/是否合入由主管定 |
| wt_writegate155 | port/timefx-bus-writegate-1（2） | 20261021 run 6 项 | 同上（与 topoplan154 同族） |
| wt_tracklist1 | port/track-list-gap-1（5） | ledger 目录 ~1MB | 同上 |

## 5. C 类清单（13 项，跨流/detached/E 盘，不判删）

**codex 流（8）**：
- C:/Users/timoz/.codex/worktrees/3ca4/Vit_DAW（DETACHED 46176efd，干净）
- C:/Users/timoz/.codex/worktrees/8bf5/Vit_DAW（codex/phase-c-closeout，干净）
- C:/Users/timoz/.codex/worktrees/harness-review-evidence/Vit_DAW（DETACHED，dirty=2：L1-5 复核留存目录 ~2MB）
- C:/Users/timoz/.codex/worktrees/l1-5-a-review-publish/Vit_DAW（codex/l1-5-impl-a-review-1-report，干净）
- D:/Vit_DAW_worktrees/eq-vps-control-graph（codex/eq-vps-control-graph，干净）
- D:/Vit_DAW_worktrees/g3-gap-composer-layout-fix（codex/g3-gap-composer-layout-fix，干净）
- D:/Vit_DAW_worktrees/g7-branch-worktree-multi-agent（codex 同名，dirty=1：agent/internal/chat/.vit_history/ 1MB——T3 时代历史泄漏同型目录）
- D:/Vit_DAW_worktrees/l1-5-c-review-coord（codex/l1-5-c-review-done，**干净+已合入**——codex 流确认后即可删）

**claude 流（1）**：
- D:/Vit_DAW/.claude/worktrees/xenodochial-poincare-1c86a8（claude 同名分支，dirty=2：orchestration/builtins.go、orchestrationruntime/runtime.go **tracked M 共 ~17KB 源码改动**——跨流未提交实现，须 claude 流确认归属）

**本流 detached（3，干净+已合入，实为 A 性质，裁定后可随 A 组删）**：
- D:/Vit_DAW_wt_221stack（DETACHED e941967）
- D:/Vit_DAW_wt_234coord（DETACHED 66c332d）
- D:/Vit_DAW_wt_sendux1_coord（DETACHED 6cf1ff4）

**E 盘恢复演练（1）**：
- E:/Vit_DAW_Backups/ADR_AGENT_CLEANUP_PHASE0A_20260721-101438/restore-drill/phase0b_commit_test_v1（DETACHED，干净；E 盘归属）

## 6. 硬保留（5 项）

| worktree | 状态 | 注记 |
|---|---|---|
| D:/Vit_DAW（主检出） | main 7413a8bf，dirty=368 | 全为领取前既有（运行态 Settings/default_project + coord/runs 未跟踪工件 + 他卡卡面移动）；领取快照存证，本卡零改动 |
| D:/Vit_DAW_wt_228 | port/m3-track-plugin-disclose-1，dirty=1 | kernel f9e3da0a 只读复用源（235-249 在用）；dirty=VitApp/build-tests/ **138MB 构建产物**——如需回收磁盘可单独裁定清 build 产物但保 worktree |
| D:/Vit_DAW_wt_249run | port/m3-group23-discriminate-1，干净 | 249 判别 run 源（已收口，裁定后可降级） |
| D:/Vit_DAW_wt_251run | port/m3-group3-discriminate-1，干净 | 251 判别 run 源（已验收合入，裁定后可降级） |
| D:/Vit_DAW_wt_253impl | port/m3-timewindow-intent-arm-1，dirty=2 | **253 在飞卡**（automation_intent.go/test 实现中），勿动 |

## 7. 分支引用层面（供 Phase 2 一并处置）

本地分支共 372 支（不含 main）：port 167 / coord 158 / codex 33 / fix 5 / 杂项 9。

- **已合入 origin/main 的 port/coord 共 241 支**：
  - 147 支**无任何 worktree 持有** → 可直接 `git branch -d`（纯引用清理，不涉及工作树）；
  - 94 支 attached（=A 类 78 + 硬保留 4 + B 已合入 12）→ 随对应 worktree 删除后再 -d。
- 已合入的非 port/coord 12 支（codex/fix/杂项）→ codex 部分跨流不动，其余待裁。
- **未合入 119 支**：port 75 / codex 27 / coord 9 / fix·杂项 8。其中 attached 34 支（=A₀ 27 + B 未合入 7）；其余 85 支为孤儿引用（提交仅存于引用，例：port/mixlayer-m1-1…m1-5、port/webui-*、port/midi-* 等历史实验线，多为 ahead=1–3）。**未合入分支一律保留待主管逐支裁定**，本报告不提交废建议。
- 提示：分支引用删除不缩小 .git 对象库（对象共享，需 gc 才回收，且合并过的对象仍被 main 引用）；分支清理的意义在引用面收敛而非磁盘。

## 8. 磁盘回收预估（抽样法）

方法：逐 worktree 精确 du 全量 142 项约需 2 小时，后台趟仍在跑（供 Phase 2 前后对照用）；本预估基于 7 个代表样本（coord 树 650MB、impl 树 977-1033MB、stack 树 777MB、A₀ 496MB、B 665MB、klte1 3355MB）。

| 裁定范围 | 数量 | 预估回收 |
|---|---|---|
| 仅 A 类 | 78 | ~55 GB（均值 ~0.7GB/个） |
| A + A₀ | 105 | ~73 GB |
| A + A₀ + B 全弃置 | 124 | ~89 GB（含 klte1 build 2.7GB；B 归档组拷贝后删除仍回收 worktree 本体） |
| 附：分支引用清理 | 241 支 | ≈0（见 §7 提示） |

误差声明：单 worktree 0.5–1.0GB 区间，预估 ±30%；Phase 2 执行时以逐项 du 前后对照出精确值（验收标准项）。

## 9. 检查点：请主管+用户裁定的事项

1. **A 类 78 项**是否授权删除（worktree + 已合入分支引用）；
2. **A₀ 类 27 项**：worktree 删除与分支废留是否分离处置（建议：删树留支，分支另开裁定）；
3. **B 类 19 项**逐条：弃置组 12 / 归档组 3（pgtdc_compat 归档前置=研究报告入库）/ 上交组 3 的处置；
4. **C 类**是否发起跨流确认（codex 8、claude 1——含 17KB tracked 源码改动、E 盘 1）；本流 detached 3 项是否并入 A 组；
5. 硬保留 5 项维持确认；wt_228 build-tests 138MB 与 wt_249run/wt_251run（已收口）是否降级；
6. 147 支已合入孤儿分支引用是否本轮一并 `-d`；
7. 磁盘回收范围选择（§8 三档）。

## 10. 工件与可复核性

- 逐项数据（TSV）：`%TEMP%/wtcleanup250/`（inventory.tsv=142 项分类底表、dirty_detail2.txt=dirty 明细修正版、branch_inv.tsv=372 支分支表、worktree_list.txt=porcelain 原始输出、baseline_*.txt=领取基线快照）——本报告关键结论已自含，临时工件仅供复核，不入库。
- 全量精确 du 后台趟完成后落同目录 disk.tsv，Phase 2 用作前后对照底数。
- 本卡 Phase 1 零 git 写操作（除领取协调提交）；主检出 status 与领取基线一致（差异仅本卡卡面+本报告）。

---
*Phase 2（删除执行）待本检查点裁定后另起；本文档 Phase 2 部分将由后续回执追加。*
