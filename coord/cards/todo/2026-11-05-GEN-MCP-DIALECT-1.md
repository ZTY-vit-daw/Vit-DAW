# GEN-MCP-DIALECT-1：MCP 作为 genapi dialect/transport（生成线第二张）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[259 报告](../../reports/2026-11-05-gen-recon-1.md) 点③ + generation-mcp-pivot §一）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 285；前置=278；P2 / 1-1.5 天；L1-L2 / PC；零 VitApp 栈

## 目标

1. **genapi 增加 MCP transport dialect**：Generate 管线复用（278 已立），仅 call 层换 MCP RPC（stdio/uvx 形态）——外部能力接入的通用模式落地（259 点③结论：MCP 只作 dialect 选项，不作默认；对有 HTTP API 的服务冗余）。
2. **SunoMCP 形态对接准备**（不验 Suno，验证归 288）：目标 server=[AceDataCloud/SunoMCP]（uvx stdio，生成+stems 两分+12-stem）的调用面封装；无凭据环境用 mock server 走通 RPC 面。
3. dialect 注册与选择机制（yue-local=HTTP 直连默认 / mcp=外部源选项），零 LLM 行为变化。

## 约束

- 文件域=agent genapi 包+测试（mock 腿）；零注册面（278 已注册 genapi 命令族，本卡纯 transport 层）；零内核。

## 验收标准

- mock MCP server 走通 generate/stems 调用面单测；既有 yue-local dialect 零回退；回执附 transport 层设计一页+commit hash。

## 停止条件

- genapi 架构与 dialect 假设不符→锚点取证上交。

## 领取

- （执行侧回填）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
