# FE-INTERACTION-CARD-KIT-1 浏览器级 DOM 断言记录（dom_assert_20261030_1）

- 日期：2026-10-30 晚；执行：ZCode GLM-5.3 PC 执行会话（224 卡会话）
- 被测：worktree `D:/Vit_DAW_wt_ick1` @ `port/fe-interaction-card-kit-1`（实现提交后回填 hash）webui dev server（vite 6.4.3, 127.0.0.1:5178/app/）
- 通道：真实浏览器（Edge，playwright-cli 0.1.22 `--browser msedge`）；/agent 流量经 vite 代理至本地 mock agent（`mock_agent_server.js`, 127.0.0.1:5799，请求体全量落 `requests_log.jsonl`）
- fixture：`chat_choreo_confirm.json`＝agent 侧 `automationChoreoConfirmationResponse` 输出的忠实镜像（payload.card 契约=interaction_card.v1 六字段+approve/cancel）；`respond_settle.json`＝写入完成回执

## 断言清单（全数 PASS）

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| D1 | 编舞话术发送后确认卡渲染：`[data-card-schema="interaction_card.v1"]` 在场，标题「段落包络写入方案（确认后执行）」+徽标「待确认」 | PASS | page-2026-10-30T12-46-42-851Z.yml（e235/e236） |
| D2 | 六字段在场且类型与声明一致：shape:select / start_value:number / end_value:number / curve:range / region_start:number / region_end:number | PASS | eval 结果（本文件下引）+快照 e251-e297（曲率为 slider、区间字段带 unit "s"、min/max/step 均传入控件） |
| D3 | select 选中态渲染：`[data-field-option].selected` = fade_out（与卡声明 value 一致） | PASS | eval |
| D4 | 动作按钮组在场：data-action-id=approve（label 确认写入，primary）与 cancel（label 取消，secondary） | PASS | eval + 快照 e298-e301 |
| D5 | 字段编辑生效：fill 起点值=0.55 后 input.value="0.55" | PASS | eval（startValue:"0.55"） |
| D6 | 点击链路-approve：点击「确认写入」→ POST /agent/interaction/respond 载荷含 interaction_id=interaction_choreo_dom1 + action_id=approve + decision=approve + payload.fields 六键全量（含编辑值 start_value="0.55"） | PASS | requests_log.jsonl（本目录） |
| D7 | 确认卡收敛：点击后卡 dismissed、无剩余可点 approve、用户转录「确认写入」+回执「已写入并逐点读回验证通过…」入流 | PASS | eval（cardCount:0/clickableApprove:0/replyTail） |
| D8 | 点击链路-cancel：第二轮卡点击「取消」→ action_id=cancel + decision=cancel | PASS | requests_log.jsonl 末行 |

eval 原始返回（D2/D3/D4/D5）：
```json
"{\"fields\":[\"shape:select\",\"start_value:number\",\"end_value:number\",\"curve:range\",\"region_start:number\",\"region_end:number\"],\"buttons\":[\"approve\",\"cancel\"],\"selected\":[\"fade_out\"],\"startValue\":\"0.55\"}"
```

approve 回传载荷（D6，requests_log.jsonl 原文解析）：
```json
{"interaction_id":"interaction_choreo_dom1","action_id":"approve","decision":"approve","payload":{"fields":{"shape":"fade_out","start_value":"0.55","end_value":"0.30","curve":"0.00","region_start":"0.0","region_end":"30.0"}}}
```

## 端测覆盖边界声明（§5）

- 本腿=渲染面+点击链路（浏览器级真实 DOM、真实 React 事件、真实 fetch→vite 代理→HTTP），**server 侧为 mock**：executeAutomationChoreo 的治理写入/内核读回不在本腿覆盖内——由真栈腿（PC-RUNTIME-STACK.md 登记）覆盖「按钮点击→写入生效」，与话术确认对照轮。
- 历史水合后的 settle 态渲染已由 vitest（renderToStaticMarkup）断言（已确认/已取消徽标+无按钮），浏览器腿未单独重放刷新场景。
- E2E-WEBUI-1 建设卡未就绪，本腿按卡面指示以 playwright-cli 先例形态执行并留工件。
