# GEN-API-ADAPTER-1：生成模型 API 统一适配+Suno 反代封装（生成集成面①）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=用户三线同步设计指令："SUNO 作为只提供网页端的平台，想办法把它反代出来单独封装一个版本；其他模型统一用一个模板，用户只需填入不同的 API 配置就能同步使用"）
- 派发确认：已确认（用户 2026-10-11 晚窗）
- 验收负责人：GLM 主管决策流
- 池序 79；目标仓库=D:\Vit_DAW
- 归属：Mac 生成线（生成=生成模型调用能力——用户"生成≠MIDI"口径）
- 优先级 / 预估 / 依赖：P1 / 1.5-2 天 / 无硬依赖（genctx 注入块为收口项：GEN-CTX-INJECT-1 合入后 prompt 组装消费 RenderGenerationPrompt gen_api profile——设计件三 §2 契约）
- 模型分级：L2 / GLM 执行会话（Mac）

## 目标三件

1. **统一适配模板**：用户填 API 配置（provider/base_url/api_key/model）即用的适配层——请求构造/参数归一/响应解析/错误分类；产物落 GeneratedAssetService 既有 ingest 路径（bridge_ingest_generated_asset→clip/take，Mac 基线已实证）。
2. **Suno 反代封装**：网页端平台反代出 API 面（中转站先例）——会话/token/任务轮询/产物下载封装为适配器一型；**风险如实入档**：非官方接口稳定性/账号/条款风险，统一模板的官方 API 型（MusicGPT/Stability/Gemini/自托管 MusicGen 类）为天然降级路径。
3. **统一化收口项**（INJECT 合入后）：prompt 组装带工程音乐上下文注入块（gen_api profile），验收断言注入块在场证据。

## 文件域

agent 侧新适配包+配置面+scripts 冒测；零内核改动（产物走既有 ingest 命令）。

## 验收标准

- 单测：配置驱动适配器（多 provider 形态）+错误分类；
- 真栈端到端 ≥1 家：用户配置→生成→产物 ingest 落轨→clip 可寻址（数据级断言，exit 0）；
- Suno 型可行则同链路演示一轮；**不可行（接口封闭/风控）→如实记录 blocker 转官方 API 型为主，不冒充**（AGENTS §8 失败分类）；
- 全量 `go test ./... -count=1` 0 FAIL+gofmt 净。

## 停止条件

Suno 反代探测发现根本性封闭（无法经会话完成生成任务）→ 取证上交，卡内余量转官方型收口。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 适配器清单+可行性结论 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
