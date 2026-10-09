# Ruling：CONFIG-BOM-1 pass（2026-10-09 决策侧）

- 裁定：**pass**。cherry-pick 25e251ac → main 0b595a19。
- 亲核四层：
  1. diff 亲读：`bytes.TrimPrefix(data, []byte{0xEF,0xBB,0xBF})` 于 Unmarshal 前——恰为卡面单点容错；config_invalid fail-visible 语义保留且有新测试锚定（BOM+malformed 不被挽救）；env>BOM 文件优先级用例在位。
  2. 我方复跑：`go test ./internal/chat -run 'Test.*Blind' -count=1` 于执行侧 worktree——ok（1.251s）。
  3. 卡外改动核验：同文件 1148 行 gofmt 空白修复（map 字面量对齐，纯空白），无逻辑触碰。
  4. 全量门：执行侧 90 包 0 FAIL 申报 + 决策侧 main 合入后全量复跑（见 decision-log 当日笔）。
- 烟测豁免裁定（决策侧）：本卡为配置解析容错，无渲染面/旅程面行为变更——单元级验收充分，**豁免真实栈烟测成立**（沿 QUERY_ENGINE §6.5 豁免先例谱系：零行为面变更类）。
- 备注：执行侧"端侧烟测豁免待裁定"的申报正确走了上交而非自判，链路健康。
