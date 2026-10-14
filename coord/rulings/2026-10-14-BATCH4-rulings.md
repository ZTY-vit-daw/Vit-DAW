# 2026-10-14 批量四裁定（109 rerun5：执行采信+三修正向确认+新确定性缺陷立卡 115）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"任务卡已完成，可以检查"。
> 方法：REPORT §九亲读+四代码锚点亲核（smoke 断言 :1106/driver.go:183 跨修订守卫/events.go fold/audio_closure_controller.go:596-634 回放过滤）+run10/11 终局工件亲核（terminal 行+settlement 时间戳+waiting_for_user 停泊证据）；本腿零代码改动无需亲跑。

## ① rerun5 执行 —— **采信：三修真栈效果全部正向，链路历史最深**

- **112 正向**：run9 形态（观察包绑定缺失）未复发，两轮全部观察绑定三元组完整；标记 0 次=路径未触发（预期形态，非失覆盖——复发且标记仍缺才构成未覆盖）。
- **113 正向**：run11 弹回新文案真栈在场（"requires only target-level track.* observations"，不含被废的 decide-from-recorded-evidence），模型随后成功换视图=教学可达性修复生效；收窄子集通道未被选用（整集重取已够）——未 exercised 如实登记。
- **腿 E 正向**：**打地鼠死透**——两轮均一次教学后即拼出完整合法 materiality（run10 三违反+骨架→下一尝试即合法；run11 同形）；教学有效性首次真栈验证通过。
- **链路抵达历史最深**：干预 applied→post-action 观察入账→合法 settle 停泊 user_judgment_pending（waiting_for_user 报告证据亲核）→1 秒内被机器结算压过（run10 12:01:48→12:01:49.135 亲核）。LLM 面历轮最健康（0/11+1/13 超时）。

## ② 新缺陷 —— **确定性产品缺陷（第四次），v4-pro 不发（梯子例外，第三次同构判断）**

- 根因链亲核成立：干预使 revision 前进→events.go fold 清空闭包 Observations（按设计）→合法 settle 使 requires_post_action_observation 翻 false→停泊轮回放把 pre-action 旧修订观察再水合（过滤器因标志 false 放行+去重因记录已清失效）→driver.go:183 跨修订守卫正确判 stale→机器结算压过停泊。烟测 2s 轮询必输 1s 竞速=exit 0 结构性不可达。
- **梯子例外裁定**：卡面"flash 思考档 2/2→升 v4-pro"条款按立法意图（模型能力假设失效时换档）不适用于产品缺陷签名——两轮模型行为已完美（一次教学收敛+合法停泊），换档预判空烧（与裁定三/裁定四同构，第三次）。梯子恢复条件=115 合入后 rerun6 仍同签名 2/2 且取证排除产品面。

## ③ 立卡 115：FIX-SETTLE-PARK-REPLAY-RACE-1（audioclosure/chat 闭包回放面，L2）

- 三修复面候选（执行侧取证后定 seam 并写设计依据，109 B-i/B-ii 先例）：(a) 停泊态（round decision=user_judgment_pending / experiment=waiting_for_user）不回放旧修订观察；(b) project_revision_changed fold 将被替换 revision 登记 superseded（与 governed booking 对齐）；(c) 停泊后闭包回放整段短路。
- **硬约束**：不弱化跨修订守卫语义（守卫按设计工作，防旧修订证据伪装新鲜；修的是回放侧不该把旧修订观察喂进停泊闭包）；守卫对非停泊路径行为不变；停泊不变量（FS-SETTLE-TERMINAL-1）加单测钉。
- 验收：diff+seam 论证+单测三形态（停泊态旧修订观察不回放/停泊 2s+ 不被结算/非停泊回放路径不变）+95 包 0 FAIL。纯单测收口，真栈归 109 rerun6。

## ④ 109 状态

- 卡维持 doing；**rerun6**（flash 思考档 2 轮，判据不变：exit 0=三账同销+体验场次解锁）在 115 验收合入后发；fsdc1r2 保留。三笔挂账未销（如实）；delay 加分腿未跑（前提未达）。

## 工件

- 裁定文件=本文件；本腿零代码零合入；run 工件 coord/runs/FIX-FS-SETTLE-DEADCHAIN-1/（console×2+报告×2+debug/telemetry/探针）在卷。
- 环境中断类记录采信：run10 首启 bash 反斜杠吞路径（未启栈，rerun2 run5 先例）；VSP Hub 8787 未起=与 run9 对照等价非新差异。
