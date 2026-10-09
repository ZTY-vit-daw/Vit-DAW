# Ruling：REFSCHEMA-M2X-1 pass（2026-10-09，决策侧）

- **裁定：pass**。合入 main = cherry-pick 3757d115（impl+回执）+ fda992ec（done 移动）；领取提交 ff37e5b2 执行侧已按 §3 直推 main。
- **决策侧独立复验（Mac 决策会话亲核，非转述）**：
  - diff 亲核：6 文件 +584/−61 全在 `agent/internal/{tim,capabilitycontext}` + `coord/runs/REFSCHEMA-M2X-1/receipt.md`，文件域合规；**agentprotocol/mom 零 diff**（vit:// 文法/注册表/文档零改动，卡面预期兑现）。
  - 五锚点基线回查（@ff37e5b2）全中：projection.go:71/107/404、asserter.go:143-144 变量块、gain_staging.go:1321；12 条清点复核（1+3+2+变量块 3+3）与回执一致。
  - 迁移形态亲核：`mixReadRefs`→`agentprotocol.FormatRef`（Kind=mom、scope_kind=mix.read、scope_value=纯数据键、window t=all、snapshot=observation_id、hash 显式 "-"），与 M2 mom 包 13 处消费面同型；`gainStagingEvidenceRefs` 内 observationID 复用既有 `firstText(mixObservation,"observation_id")` 读取，无新增数据来源。
  - 范围外保留亲核：`dad:track_waveform_envelopes` 原样、`hygieneAssertionRefs`（project.state: 族）不动、同函数 `project.state`/`mix.observe[:id]` 族不动；两包生产代码零残留裸 `"mix.read:` 字面量（rg 亲核，vit://mom 形态内嵌与测试 fixture 除外）。
  - **红先行独立复验**：temp worktree @ff37e5b2 + 两新测试文件 → 真 FAIL（断言级，tim 4 + capabilitycontext 2 可见），与回执"实现前 6 FAIL"申报一致。
  - 测试断言面亲读：ParseRef `RefStateParsed` + 段断言（kind/scope_kind/snapshot/t=all/hash）+ 规范格式往返 + legacy opaque 兼容读钉 + dad:/plugin_hygiene legacy 保留钉。
  - gofmt 甄别复核：本卡 5 触碰文件净；回执§6 申报的两处偏差（asserter_test.go/low_end_relation.go）在本机复现且两文件本卡零触碰——既有 origin/main blob 偏差登记属实，留格式归拢卡处理。
  - 复跑：`go build ./...` exit 0；tim/capabilitycontext 定向 ok；**集成态（cherry-pick 落 origin/main 9cbef334 后）全量 `go test ./... -count=1` exit 0、90 包 0 FAIL**。
- **行为变化申报采信**（M2 先例谱系）：①空 ObservationID 不发 mix.read refs（宁缺勿假）；②capabilitycontext stablePackID 由 EvidenceRefs 派生、随形态变化（gain_staging.go:172 派生源亲核，内容寻址语义不变）；③refs 变长 +51B/条（两包内无字节预算断言，mixboard 预算门 M2 已扩至 70000，本卡不触碰 mixboard 域）。
- **端测边界裁定**：纯 Go 单测域（卡面明示无真栈腿），按 M1/M2 先例免真栈验收。
- 执行侧评价：Mac 线复卡首战达标——领取腿基线核对如实（基线净+既有偏差如实分记）、红先行申报与决策侧复验吻合、消费面清单全列可复核。
- 上交事项：无。dad:/project.state:/mix.observe 族迁移机会留 M 系列后续族卡（G1 终审记录挂账不变）。
