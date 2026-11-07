# VSTI-WRITEGUARD-1：agent 通用写面 de-esser 守卫对 VSTi 的误拦分类修（298 转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 晚窗（依据=[298 回执确定性发现](../done/2026-11-07-VSTI-PARAM-PROBE-1.md)——agent 通用写面 `set_plugin_param` 对 Grand Rhapsody 被 de-esser 守卫 fail-closed 拒绝（插件级 `unresolved_de_esser_surface`→默认分支拦截，goalrunner_chat.go:1425-1455），Surge XT 同面通过；内核 ZMQ 面两件全绿（apply→readback→restore 零 mismatch）——缺陷仅在 agent 通用写面的守卫分类）
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 302；目标仓库=主仓 agent（goalrunner_chat.go 守卫分类面）；P2 / 0.5 天 / 依赖=298 工件在库（probe1_20261107_1920 参数面+拒绝回执为夹具）；L2 / PC；零栈先行，真栈回归搭后续 run

## 目标

1. **守卫分类修**：de-esser 守卫默认分支对 instrument 族插件的正确分类（不落 `unresolved_de_esser_surface`——按 298 采证的 template_role=instrument 通道或插件族判据分流；非 VSTi 插件行为零变化）；
2. 配对单测：instrument 写面通过+效果器原守卫行为不变+未知族维持 fail-closed 三方向钉死（298 工件拒绝回执为负锚蓝本）；
3. 回归：`go test ./internal/chat -count=1` 全绿+build 0；真栈回归腿（Grand Rhapsody agent 面写通+快照恢复）搭 296-R2 或后续栈 run 顺带，不单占。

## 约束

- 文件域=goalrunner_chat.go 守卫分类分支±测试；不触 PCA 门/装载链/内核；**非 instrument 插件的守卫语义零改动红线**；
- 禁止 git reset --hard / git clean；不动主工作树；零真栈（本腿）。

## 停止条件

发现守卫分类存在在案裁定约束（如 de-esser 面曾有明确 fail-closed 裁定）→锚点取证上交转裁定。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
