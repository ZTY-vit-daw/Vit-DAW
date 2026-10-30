# M3-CHAT-AUTH-STAMP-1：chat 消息链补权限状态 stamp——完全权限下编舞确认卡放行接线（219 面 4 真栈缺口）

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=用户手测亲证完全权限下编舞确认卡照弹+主管代码亲读定链：stampInvokeAuthorityMode 只覆盖 harness InvokeRequest，chat ChatRequest 链无 stamp，choreo 桥读 req.Context 永缺 explicit 对；219 单测过=测试手动塞 context，真栈无人塞）
- 派发确认：已确认（用户 2026-10-30「我开了完全访问权限就不该弹出确认卡」即诉求+「可以开卡了」）
- 验收负责人：GLM 主管决策流
- 池序 223；目标仓库=**主仓 agent**；P1 / 0.25 天 / 依赖=无；L1 量产档可做

## 标的（主管已定链，不重考）

- `stampInvokeAuthorityMode`（turn_control.go:127）注释自述"binds the server's own authority state onto the harness context of one invoke"——只打 harness invoke（PCA load gate 族）。
- 编舞是词表确定性路由（chat 直达，不经 harness invoke）→ `fullAccessChoreoConfirmationContext(req.Context)`（full_access_semantic_chain.go:64）判 false → 完全权限下确认卡照弹（用户 2026-10-30 手测亲证）。

## 目标

1. chat 入口补 stamp：chat 消息进入词表确定性路由**之前**，把 server 自身 authority 状态（`authorityModeSnapshot()` + `authorityModeExplicit`）merge 进 req.Context——与 stampInvokeAuthorityMode 同语义同源。
2. **红线维持**：stamp 只读 server 内部状态；**request body 声明的 authority_mode 不得构成授权**（裸模式串永不授权不变；正例=webui 经 /agent/authority 控制面设置的 full_project_access+explicit 闩）。
3. 三放行点回归：两入口确认卡发出点+span 采纳路径在 stamp 后于完全权限下全部直执；manual 模式确认卡字节形态不变。

## 探针义务

- 单测：完全权限态（server 状态面）下编舞话术→桥放行直执（不再手动塞 context，从 server 状态走）；manual 态确认卡保持；request body 伪造 authority_mode=full_project_access 不生效（负例，红线）；409/locked 形态不越权。
- 真栈腿：**必跑**（219 真栈缺口的教训——单测过≠接线通）：真实栈 webui 切完全权限→编舞话术→无确认卡直执+重启后仍直执（持久化链一并覆盖）；manual 回切→确认卡恢复。走现有 ps1 体系扩展，exit 0 为门。

## 文件域

`agent/internal/chat/turn_control.go`+chat 入口 stamp 调用点（server.go 或路由入口，勘察定最小侵入点）+测试+scripts 真栈腿。越域即停。

## 停止条件

- 勘察发现 chat 入口存在多条（webui/HTTP/其他 transport）且 stamp 位置有歧义→锚点+方案上交；
- stamp 引入确认话术分支行为回归→停止上交。

## 领取

（待领取：记录 HEAD/status/diff --stat；真栈与 224 错峰登记 PC-RUNTIME-STACK.md）
