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

- 领取：2026-10-11 21:42 CST / origin/main=dba5c516ff6f6668e1c9513a0ab244f66e030236 / owner=GLM-5.3 Mac 执行流（ZCode 会话）/ 分支 port/gen-api-adapter-1 / worktree /Users/timozty/Documents/Vit-DAW-genapi1 / 领取提交=本提交（coord-only 直推 main；doing/ 目录随领取重建——origin/main 当前无在飞卡）
- 回执：**done（自验 exit 0）**——实现 commit **1e23fa8f** @ port/gen-api-adapter-1（已推远端；零内核改动 VitApp/ 0 文件）。权威 run `coord/runs/GEN-API-ADAPTER-1/run4-pass-20261011-232432/`（**17/17 门 exit 0**，schema genapi.adapter.smoke.mac.v1）。**适配器清单**：新包 agent/internal/genapi 五方言=sync_binary（自托管 MusicGen 型）/sync_json_url/sync_json_base64/async_poll（Stability 202-200 双语义）/suno_relay（new-api 中转站契约逐字段：submit 双模式+fetch 轮询+data[].audio_url 双候选）；配置面=~/.vit/genapi_providers.json 用户填 provider/base_url/api_key/model 即用（VIT_GENAPI_CONFIG/VIT_GENAPI_KEY_<ID> 覆盖）；工具面=gen.generate/gen.providers（catalog+invokeLocal 先例模式，凭据零回显单测+真栈双断言）；产物经**既有** aigc_register_job+bridge_ingest_generated_asset 落轨多候选 take。**可行性结论**：Suno 反代技术面可行已实现（中转站先例成立，契约锚 new-api 公开文档）；外源真账号腿=环境阻塞——环境无任何外部 API key（§8 环境中断类如实申报不冒充），真栈由本地自托管型模拟器承载全链路实证（config→生成→下载→ingest→clip 可寻址→多候选 switch_take→fail-closed 三负探针），有凭据后填 config 即跑同链路零代码改动；风险（非官方接口稳定性/账号/条款/中转站第三方）入档 report.md §一.2，官方型为天然降级路径。**单测**：genapi 21+harness 5（多 provider 形态+错误分类十一类）；**全量** go test ./... -count=1 93 包 0 FAIL exit 0+gofmt 净。**端测边界声明（AGENTS §5）**：验证面=Mac 两件套真栈（内核复用 GEN-MAC-BASELINE-1 二进制 9a70f064，VitApp/Source 与该基线零 diff 已核）命令/回执面+本地 provider 模拟器 HTTP 面；未含 Godot 前端/webui 渲染/用户旅程面（本卡改动不涉）；外源 provider 面未测（无凭据）。收口项③（INJECT 注入块）未做=卡面约定。详证 [report.md](../../runs/GEN-API-ADAPTER-1/report.md)（含探针迭代三红轮均脚本/断言缺陷非栈失败的 §8 分类记录）。栈占用登记 fa25ae47→释放随本提交（23:25 亲核四端口监听=0、VitApp/vitagent 进程=0）。
- 验收：pass（2026-10-12）——[rulings/2026-10-12-GEN-API-ADAPTER-1-pass.md](../../rulings/2026-10-12-GEN-API-ADAPTER-1-pass.md)；决策侧复验=内核 diff 亲读/独立复跑 exit 0/全量 0 FAIL（详裁定文件）；合入=cherry-pick 实现提交；验收提交随 accept 链合 main
