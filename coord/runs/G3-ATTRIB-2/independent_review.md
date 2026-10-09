# G3-ATTRIB-2 独立复核报告（复核腿，2026-10-09 晚窗）

- 复核人：独立复核 agent（general-purpose 子代理，与执行会话分离）；报告交主管裁定，非最终验收
- 被审：commit `24a6e722`（基线 origin/main=7eb38fc5+d6c4f060/28d84fdf 领取提交）；concern 清偿后补提交 `af12b583`（驱动级测试文件入库）
- 方式：只读复核+临时目录独立 dump 对照（git archive 两版模块，13 状态矩阵逐字节 cmp）+变异法验证测试有效性（临时副本植错，不动 worktree）

## A. 字节恒等（普通族组合渲染 vs 基线 7eb38fc5）— PASS

结构核对：基线 `messageLoopSystemPrompt` 用 `fmt.Sprintf` 模板尾 `\n%s\n\nAvailable tool catalog:\n%s\n\nAllowed tools:\n%s` 组合；新实现（message_loop.go:4238-4240）为 `skeleton + "\n" + modeRules + "\n\nAvailable tool catalog:\n" + catalog + "\n\nAllowed tools:\n" + allowed`——分隔符逐字符一致。verified-entry 裁剪基线作用于整串 prompt，新版作用于 skeleton；两个裁剪目标串在整串中的最左出现均落在 skeleton 前缀区内，裁剪区域等价。

亲验（不只看测试绿）：临时目录分别解出 7eb38fc5 与 24a6e722 两份完整模块，注入同一 dump 测试（13 状态矩阵：nil state、空 input、modeRules 空/非空×4 形态、verified-entry 裁剪×3 形态、中性族两轮），输出 JSON 落盘后 `cmp` 逐字节比对：**13/13 BYTE-IDENTICAL**（nil_state 17497B / plan_mode 18039B / autonomy 19299B / verified_entry_action 16503B〔裁剪〕/ neutral_family_reasoning 18629B sha=fd0f5ce2… 等）。旁证：卡内对比工件 neutral turn1 old_stable_segment=18629B/sha256 fd0f5ce2… 与独立 dump 完全一致——工件与代码互证。既有 push 锚定测试复跑 PASS。

## B. P1 供给语义（prefix_service.go）— PASS

- 比对基础：`prefixSnapshot.systemBytes` 存最终 system 消息内容；比对 `strings.HasPrefix(systemBytes, previous.systemBytes)`——方向正确（本轮字节以上轮开头=append-only 合法）。`Build` 把全部 SystemSections 渲染为单条 system 消息，"最终 system 消息"＝整个稳定前缀，与 `PrefixBytes` 口径同源。
- nil 诚实：无上一轮或 SessionKey 空 → 字段保持 nil；遥测侧非 nil 才写键（不可比不造值），有测试锚定（TestPrefixServiceP1SupplyTelemetryKeys 断言首轮键缺席）。
- 加法式遥测：既有 8 键不变，新增 prefix_fingerprint/prefix_content_hash 恒写、prefix_starts_with_previous 条件写；消费方均为 map 合并，无封闭键集校验。
- 签名冻结：PrefixService 接口与 Assemble 签名未动；AssemblyReport 仅加字段。

## C. 拆分挂载（loop.go assemble）— PASS

- 骨架：SectionStatic+stable=true，段 ID `pullharness.protocol` 与拆分前相同（层连续性保留，族切换映射到该层 content 变化→ruleset_changed）。
- 指令块：SectionRuntime+stable=false 进 user 段 → 渲染进 user 消息，绝不进 system 消息；PrefixBytes/systemBytes 语义不被污染。
- legacy 回退：`else if`——仅骨架键缺席或空白时按原单段 stable 形态挂载，段 ID 不变。宿主侧不再设置 legacy 键（有测试断言缺席）。

## D. 拆分正确性（pull_session.go pullProtocolSegments）— PASS

- 中性族分支直接返回 L1-4 的 messageLoopNeutralFamilySystemSkeleton/messageLoopNeutralFamilyTurnDirectives——原样复用，无二次包装。
- 普通族 directives = modeRules+目录行+allowed 行——G3-RULING §2.4 列的族内两个内容变化源全部进动态区；modeRules 提取体与基线逐字一致。verified-entry 裁剪刻意留在骨架并注释为合法 ruleset_changed——与裁定一致（真实规则变化）。
- 族切换：换族＝整个 skeleton 换渲染器→层 content_hash 变→ruleset_changed；directives 变化只落动态区不断前缀。
- 健壮性备注（非缺陷）：普通族分支裸解引用 state.input，nil state 会 panic——生产不可达（newPullSession 拒 nil state）。

## E. 测试有效性 — PASS（concern 已清偿）

变异法亲验（临时副本植错）：
- 植入 `startsWith := true`（判定恒真）→ TestPrefixServiceP1SupplyPerturbationDetected FAIL ✓
- 植入旧接线（skeleton=组合渲染整串）→ TestPullProtocolSegmentsStableWithinFamilyAcrossTurns 在"ordinary-family skeleton must stay byte-identical"处 FAIL ✓
- 比较方向反置会被 TestPrefixServiceP1SupplyAppendOnlyGrowth 抓住 ✓

Concern-1（前置）：**protocol_split_test.go 未跟踪、不在 24a6e722 内**——驱动级最强四测试（双段挂载形态/族内连续轮真实 driver+PrefixService 骨架恒等/族切换恰一条 ruleset_changed/驱动级扰动检出）落在未提交文件上；loop_test.go 的 fakeSession 支撑字段在提交树内无使用者。→ **已清偿：af12b583 补提交**。

次要备注（不计入放行）：TestPullProtocolSegmentsNeutralFamilyReusesL1_4Split 与 TestPullProtocolSegmentsOrdinaryFamilyParity 为构造性锁（wiring-lock/parity-lock，防改道与防漂移有价值，但不计为行为证据）。

## F. 越域与冻结面 — PASS（concern 同 E-1，已清偿）

- 提交只触碰声明文件域；`git diff 7eb38fc5 <commit> -- pull_entry.go / agent/internal/chat / server.go / scripts` 为空——G3-ATTRIB-1 禁域未越。
- 冻结面：Session 六方法签名未动（session.go 仅 const 注释）；PrefixService 接口/Assemble 签名未动；ExitWiring 未触碰；AssemblyReport 加法式扩展。

## G. 复跑验证 — PASS

- worktree：`go test ./internal/promptruntime/ ./internal/pullharness/ -count=1` ok/ok（TestPullLoopProtocolSplit 4/4 PASS）；agentloop 拆分测试组 5/5 PASS。
- 隔离提交树（临时副本、无未跟踪文件）：promptruntime+pullharness ok——commit 自身可独立编译且绿。
- `go build ./...` BUILD_OK；push 锚定测试组 PASS。

## 总评：pass-with-concerns → 前置 concern 已由 af12b583 清偿

技术实质成立且经独立复核证实：①P1 供给语义诚实、机械可判、遥测加法式、签名冻结；②协议段双段拆分正确复用 L1-4 形态，§2.4 两个族内漂移源全部落动态区、族切换保留合法 ruleset_changed；③普通族组合渲染与基线 13 状态矩阵逐字节恒等（亲验，含裁剪/nil/中性族），对比工件与独立 dump 哈希互证；④负例经变异法验证有牙。放行前置 concern（驱动级测试未随卡提交）已由补提交 af12b583 清偿。
