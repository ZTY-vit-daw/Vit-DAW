# M3-C4-AUTO-CLARIFY-UX-1：编舞澄清/确认面五断点——span 降级利用+答非所问引导+裸单端候选+完全权限桥接+权限持久化核查（218 手测反馈快修）

- 发卡：GLM 主管决策流 / 2026-10-29 深夜（依据=用户收口话术手测三轮 clarify 断点+二轮确认卡权限疑问；agent_last.log 23:33 三轮 clarify+23:5x 确认链亲证+权限链代码亲读）
- 派发确认：待用户确认（P1 体验快修，不阻主线——绝对区间通道+确认链已可用、用户已亲证写入生效）
- 验收负责人：GLM 主管决策流
- 池序 219；目标仓库=**主仓 agent/internal/chat/ 域**；P1 / 0.5 天 / 依赖=218 已合入 ✓；L1 量产档可做

## 手测复盘（断点定位，2026-10-29 23:33+23:5x 会话）

**二轮结果**：修正话术「前 30 秒…」直通确认卡→确认→写入生效（用户亲证包络线落地）——正面链 ✓。

1. 「副歌那 30 秒，…」→通道 5 命中 section=副歌+span=30→**工程段落标记缺失**→整段降级 clarify，span=30 信息被丢弃（automation_intent.go:227-241）。
2. 裸「30 秒…」→五通道全不匹配（单端无前缀不认）→clarify。
3. 用户回「确认写入」→澄清阶段不识别该词→重复同文案 clarify（automation_choreo.go:435/:726）——用户感知「打字无效」。
4. **完全权限疑问两层根因（主管亲读）**：
   a. **用户手测时权限模式未生效**——agent 启动 restore 采纳盘面 mode=manual_confirmation（agent_last.log :22 亲证；server.go:7884 adopted 分支）——用户 webui 完全权限设置未持久化到该状态面或写入面不同步（webui 开关写的面 vs restore 读的面需核对）。
   b. **choreo 确认卡未接完全权限桥**——full_access_semantic_chain.go 既有桥面（插件推荐选择/插件装载确认完全权限下 full_access_auto_authorized 自动放行）不含 choreo 面；automation_choreo.go 零查询 authority mode——完全权限即使生效编舞确认也照弹（218 接线缺口）。
5. 注：链路行为本身合规（不猜原则/文案有格式示例）；缺陷面=体验与接线五断点。

## 目标（五小面）

1. **span 降级利用**：section 命中但标记缺失且 span 在场（「副歌那 30 秒」）→不再纯丢弃：澄清问句升级为**带候选确认**（「副歌标记未找到；用开头 30 秒可以吗？回'可以'或给绝对区间」）——回「可以」=region 0..span 进确认卡。不猜原则不破（用户显式认可候选，非系统猜）。
2. **答非所问引导**：澄清 pending 时收到「确认写入/写入/可以吗的肯定词族」→差异化回复（「还在等时间区间，尚未生成方案——例如'0 到 30 秒'；回'可以'采纳候选区间」），不再原样重复问句。
3. **裸单端候选**：裸「N 秒」（无前/到/段落词）→澄清问句带候选（「是从 0 开始的 N 秒吗？」）——同面 1 的候选确认形态。不直接默认（歧义风险）。
4. **choreo 确认接完全权限桥**：完全权限显式上下文（authority_mode=full_project_access+authority_mode_explicit——**裸模式串永不构成授权**红线维持）下编舞确认卡自动放行（full_access_auto_authorized+intervention_skipped 同既有桥形态，与 pluginRecommendation/pluginGrabber 同族入 full_access_semantic_chain）。
5. **权限持久化核查**：webui 完全权限开关写入面 vs server restore 读取面（chat state AuthorityMode）是否同一持久化链——不同步则修同步（先勘察定案根因再修，回执披露写入面锚点）。

## 探针义务

场景/单测断言五面（span 候选链/答非所问引导词族/裸单端候选/完全权限桥放行+裸模式串拒绝/权限持久化往返）；既有 218 意图测试族+full_access_semantic_chain 测试族回归零回退；真栈腿可选（确定性词表面单测覆盖充分，按执行侧裁量+边界声明）。

## 文件域

`agent/internal/chat/automation_intent.go`+`automation_choreo.go`+测试。越域即停。

## 停止条件

- 候选确认形态与既有 pending 状态机冲突（状态分支不可扩展）→锚点上交两案。

## 文件域（五面版）

`agent/internal/chat/automation_intent.go`+`automation_choreo.go`+`full_access_semantic_chain.go`（桥面 4）+权限持久化涉及文件（面 5 勘察定）+测试。越域即停。
