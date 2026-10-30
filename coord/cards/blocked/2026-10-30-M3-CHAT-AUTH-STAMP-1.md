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

- 2026-10-30 20:2x（卡面原写 21:1x 系预估偏差，按机器钟修正）/ origin/main=3036783d（代码基线=bc7ee6a2 同树，`agent/internal/chat/`+`scripts/` 对 bc7ee6a2 零 diff 亲核）/ owner=ZCode GLM-5.3 PC 执行会话（223 卡会话）/ 分支 port/m3-chat-auth-stamp-1 @ worktree D:/Vit_DAW_wt_223impl / 领取提交=46e8cbf3
- 领取基线 status：worktree 干净（tracked 零改动）；主检出 M=VitApp/Workspace xml 两件+coord/runs 日志（非本卡域，零触碰）
- 文件域核对：224（FE-INTERACTION-CARD-KIT-1）域=automation_choreo.go 确认函数族+webui/src——本卡域 turn_control.go+turn_control_test.go+新增测试文件+scripts 与其零相交；勘察定案=stamp 点=既有 server.go:2573 调用位（语义重写在 turn_control.go 内，server.go 零改动）；真栈错峰已兑现（登记 d2566ed1 前全零亲核）

## 回执（2026-10-30 21:0x，blocked 上交）

- 实现 commit：port/m3-chat-auth-stamp-1 `fefbc4fd`（实现）→ `ef42dfae`（脚本修正+两轮 run 工件+RESULTS.md），已推 origin。
- 单测：chat 全包 115s 绿+agentloop+harness 绿；新增 `chat_authority_stamp_test.go` 五探针（server 态经真实 HTTP /agent/chat 入口→编舞直执不再手动塞 ctx / manual 字节形态 / 伪造 body+伪造 ctx 对+goal 运行态三负例 409 且 server 状态与闩零变化 / 重启 restore 态 stamp / manual 回声不降级 server full）；turn_control_test 三处旧 body-授权契约测试改新语义。
- **前提修正（证据在卷，供验收参考）**：卡面「chat ChatRequest 链无 stamp」与 HEAD 事实不符——`bindChatAuthorityMode`（08-19 起，server.go:2573=词表路由 2878 之前）本就 merge explicit 对，webui 也自 08-19 起在 ctx 带 explicit 对（App.tsx:995→turnControl.ts:31）。真缺口=bind 把 body 声明的 mode **写进 server 状态**（伪造 body full 即授权，红线负例坐实）；本次重写为与 invoke 路径同语义（validate+stamp 只读 server 状态）。用户真栈卡照弹最可能=219 桥（06994ce7@10-30 19:07）之前的旧二进制。
- **真栈腿（必跑，已跑两轮，exit 1 收口）**：`chat_auth_stamp_20261031_1`（r1）+ `_r2`，RESULTS.md 定案：
  - **绿的腿（两轮一致）**：switch persisted=true+workspace_ready=true；**裸话术**（body/ctx 零 authority 键）真栈直执——无确认卡+full_access 三标记+治理 settle written=2+mix_decision recorded+C4 in_progress+内核 get_curve 逐点 (0,0.30)/(30,0.60)+回执含自治通告。**卡面标的真栈成立。**
  - **红的腿**：agent 真重启后 GET authority=manual（want full）。r1 红因=断言体腿序（switch 先于 shadow 身份绑定→full 落旧身份；忠实复现 219 面5 身份切换窗，产品行为正确），脚本已修（wait_shadow_tracks）。r2 定案=**出域缺陷**：`RecoverWorkingSession` 仅在 `runtimeStateHasPendingWork` 时恢复前一 session（working_session.go:94-101），干净重启→新 session 从 canonical/存档代播种（:165-215）——**前一 session 的 authority_mode 跨干净重启永不携带**（server.go:7517-7548 激活链；session state 文件 full vs manual 在卷亲证）。=219 面5 的 2026-10-29 用户事故（切完全权限→下次启动不恢复）同链真身，面5 当时只关窗(i)。
  - 按 AGENTS §11 越域即停（修复在 history/workspace 层）。
- 端测覆盖边界声明：webui 渲染面未改（零 webui 文件）；manual 确认卡/伪造负例/重启后 span 采纳面真栈腿因 after 相早退未执行——单测面已钉，真栈覆盖边界如实声明。
- 上交三选：①扩域/另开修复卡（建议与 219 面5 窗口族合并：runtime state 跨干净重启携带）；②本卡按「stamp 交付+边界声明」验收合入；③返工并入修复卡。附开放问题：切换后 save 工程是否携带 authority 跨重启（未测）。

## 验收（主管裁定 2026-10-30 深夜，GLM 主管决策流）

- **stamp 面整卡 pass**：bindChatAuthorityMode 重写亲读（validate+stamp 只读 server 状态+伪造 body 409 红线负例坐实=**额外堵了 body 授权漏洞**）；真栈 phase-1 两轮全绿（完全权限裸话术直执+自治标记+治理 settle+逐点读回+persisted=true）；五探针+chat 全包全绿。前提修正落档接受（卡面「无 stamp」与 HEAD 不符——bind 自 08-19 在场；真缺口=写面授权漏洞；用户 10-30 晚症状疑为旧二进制）。合入 main=39076e2e。
- **重启腿红=出域真缺陷定案，转出为独立卡 226**（AUTH-RESTORE-CARRY-1，域 working_session.go+server.go 恢复链）：RecoverWorkingSession 仅 pending-work 恢复+新 session 从 canonical 播种，authority_mode 跨干净重启永不携带——**10-29 用户事故真身**（219 面 5「三丢失窗口」勘察结论被部分推翻：非窗口不可裁决，是恢复链从不播种）。执行侧 §11 越域即停正确。
- 223 处置：stamp 面 done 收口；blocked 解除条件=226 完成后重启腿随 226 验收（本卡不再回 doing）。
