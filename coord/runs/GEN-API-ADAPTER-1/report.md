# GEN-API-ADAPTER-1：生成模型 API 统一适配 + Suno 反代封装——实现与真栈实勘报告

- 执行：GLM-5.3 Mac 执行流（ZCode 会话）/ 2026-10-11
- worktree：`/Users/timozty/Documents/Vit-DAW-genapi1` / 分支 `port/gen-api-adapter-1` / 领取提交 b654a9a7（origin/main=dba5c516）
- 权威 run：`run4-pass-20261011-232432`（脚本 exit 0，17/17 门绿，schema `genapi.adapter.smoke.mac.v1`）
- 零内核改动：本卡全部提交 = `agent/internal/genapi`（新包）+ `agent/internal/harness`（本地工具接线）+ `agent/internal/tools/catalog.go`（两条 spec）+ `scripts/`（冒测脚本+SMOKE_TESTS）+ `coord/` 工件

---

## 一、交付物清单

### 1. 统一适配模板（目标①）

新包 `agent/internal/genapi`（六个文件：genapi/dialects/config/errors/paths/audio），统一管线=
**配置解析 → 请求构造（参数归一）→ 提交 → 轮询 → 产物下载 → 音频嗅探落盘 → 错误分类**。

**配置面（用户填了就用）**：`~/.vit/genapi_providers.json`（`VIT_GENAPI_CONFIG` 环境变量可改路径；`VIT_GENAPI_KEY_<ID>` 按 provider 覆盖凭据）。每条目=用户要填的四件事 provider 方言/base_url/api_key/model，其余全部有方言缺省。可复制起点=`ExampleConfig`（源码内带三方言示例，单测锁定其可解析可校验）。

**方言（API 形态）= 适配器一型的五个实例**：

| 方言 | 形态 | 典型承载 |
|---|---|---|
| `sync_binary` | POST→原始音频字节 | 自托管 MusicGen 类（官方型） |
| `sync_json_url` | POST→JSON 含下载 URL | 各家同步 JSON API |
| `sync_json_base64` | POST→JSON 含 base64 音频 | Stability 同步/Gemini 包装类 |
| `async_poll` | POST→task id→轮询（JSON status；兼容 HTTP-202 进行中/HTTP-200 音频双语义） | Stability Stable Audio v2beta、Replicate/MusicGPT 类异步 |
| `suno_relay` | 中转站事实标准面：POST `/suno/submit/music`→`{code,data:task_id}`→GET `/suno/fetch/{id}`→`IN_PROGRESS/SUCCESS/FAIL`→`data[].audio_url`（双候选） | Suno 反代（目标②，契约按 new-api 公开文档锚定） |

**参数归一**：`GenerationRequest`（prompt/lyrics/style_tags/title/instrumental/duration/tempo/key/count）→ 方言各自映射（缺省映射+`param_mapping` 覆盖+`extra_params` 静态合并；`"-"` 映射=显式丢弃）。suno_relay 按 new-api 契约双模式：lyrics 在场=custom 模式（prompt/title/tags），否则 inspiration 模式（gpt_description_prompt）；model→`mv`。**不可表达字段以 limitations 披露不静默丢**（suno 的 duration/tempo，真栈门断言在场）。

**错误分类（验收项）**：十一类 `ErrorClass`——config/auth/quota/rate_limit/not_found/provider/moderation/timeout/network/response_parse/unsupported_format，HTTP 状态+body 语义双源判定（429+quota 语义词→quota；provider 失败原因含内容政策词→moderation），每类带 retryable 与 provider_message，工具面透出 `error_class/retryable/http_status/provider_message`。

**产物落轨（走既有路径，零内核改动）**：新本地工具 `gen.generate`（catalog spec `genapi_generate`，RiskConfirm，invokeLocal 模式同 save_as_folder/render.profile.* 先例）内部经 `KernelSendCommand` 发**既有**内核命令 `aigc_register_job` + `bridge_ingest_generated_asset`（GEN-MAC-BASELINE-1 实证面）；多产物=同一 `take_stack_id` 多候选 take（多候选基底对齐）。`gen.providers`（RiskDirect）列配置（含 key 在场性），**绝不回显凭据**（单测+真栈门双断言）。下载产物嗅探音频魔数（RIFF/ID3/OggS/fLaC/MThd）fail-closed，垃圾不进 ingest；跨域 presigned 下载不带 provider 凭据（单测断言）；落盘 `~/.vit/genapi-cache/<provider>/`（运行态不入源码树）。

### 2. Suno 反代封装（目标②）——可行性与风险如实入档

**可行性结论：技术面可行、已实现、真栈链路已实证；外源真账号腿=环境阻塞（无凭据），不冒充。**

- 中转站反代是**业界既有先例**：new-api（QuantumNous）等开源网关已把 Suno 网页端反代为 `/suno/submit/music`+`/suno/fetch/{id}` 的 API 面；gmpetrov/suno-api 一类直连会话 token（cookie）反代亦有先例。本卡 `suno_relay` 方言按 new-api 公开契约逐字段实现（submit 双模式字段、任务对象、status 枚举、`data[].audio_url`），auth 支持 bearer（中转站 key）与 cookie（直连会话 token 部署，`auth.style=cookie`）。
- **真栈实证边界（如实）**：本轮真栈 run 的"suno 面"是本地模拟器（按 new-api 契约逐字段仿真 submit/轮询/双候选下载+bearer 鉴权），证明的是**适配器链路**（请求构造/归一/轮询/下载/ingest/多候选切换）在真实内核+agent 栈上的正确性；**不是**对真实 Suno 中转站账号的端到端。环境中无任何外部 API key（§8 环境中断类如实申报），有凭据后用户填 config 即可跑同链路——无需改代码。
- **风险入档（决策侧/用户须知）**：①非官方接口——上游改版即断（反代站跟进有时差）；②账号风险——Suno 条款不允许 API 化网页端，中转站账号/封号风险自担；③条款/合规——商用需自评估；④中转站本身也是第三方（key 与计费经其手）。**天然降级路径=统一模板官方型**（Stability Stable Audio/MusicGPT/自托管 MusicGen，`async_poll`/`sync_*` 方言直接承载），配置切换即降级，零代码改动。

### 3. 统一化收口项（目标③）

**本卡不做**（卡面明示：GEN-CTX-INJECT-1 合入后另卡收口）。已留接口：`GenerationRequest` 为归一化载体，INJECT 卡产出 RenderGenerationPrompt 注入块后在此汇入。

## 二、验收对照

| 验收项 | 结果 | 证据 |
|---|---|---|
| 单测：配置驱动适配器（多 provider 形态） | ✅ | `agent/internal/genapi/genapi_test.go` 21 测试：五方言全覆盖（含 Stability 202/200 双语义、suno 双模式+双候选）、参数归一/映射覆盖/丢弃、auth 四式（bearer/header/query/cookie）、跨域凭据剥离、魔数 fail-closed、尺寸帽、env key 覆盖、ExampleConfig 可用性；`agent/internal/harness/gen_generate_test.go` 5 测试（另有既存 TestGenesisBusTopologyRendering 非本卡）：ingest 腿经 fakeKernel（命令名/job 前缀/bucket=generated/take 栈）、无 track_id 零内核写、config/auth 错误分类透出、providers 不泄密 |
| 单测：错误分类 | ✅ | 状态码矩阵（401/403/402/404/429/429+quota 词/500）+网络拒绝+轮询超时+非 JSON 解析+relay 信封拒+moderation 语义词，各类断言 class+retryable+http_status |
| 真栈端到端 ≥1 家 | ✅* | 权威 run 17/17 门 exit 0：用户 config（文件+env 凭据）→gen.providers→gen.generate(sync_binary)→产物落盘 RIFF→既有 register+ingest→clip/asset_ref/take 三元组→project.state 可寻址；suno_relay 同链路双候选落轨+switch_take 切换；请求形状证据 8 项（prompt/duration 归一、inspiration 模式字段纪律、mv←model、bearer←env、轮询至 SUCCESS、双候选下载）。***边界**：provider=本地自托管型模拟器（承载链路完整性）；外源真 provider 腿无凭据=环境阻塞（§8），已声明不冒充 |
| 全量 `go test ./... -count=1` 0 FAIL | ✅ | 93 包 ok、0 FAIL、exit 0（/tmp/genapi1_gotest_full.log；12 无测试包正常） |
| gofmt 净 | ✅ | `gofmt -l` 空 |
| 零内核改动 | ✅ | `git diff --stat` 对 VitApp/ 为空 |

## 三、探针迭代记录（AGENTS §8 失败分类）

- run1（8 门红）：**实现 bug 1 处**（sync_binary 误走 JSON 解析路径——单测先抓先修）+**脚本缺陷 2 处**（内核子命令误用 tool 名 `assets.register_job`，应为命令名 `aigc_register_job`/`bridge_ingest_generated_asset`；agent 工具错误回包=HTTP 400+JSON 信封，被误判传输失败）。修：命令名对齐+invoke_tool 改"有 JSON 信封即已应答"。
- run2（1 门红）：**脚本缺陷**（模拟器证据依赖 python atexit，SIGTERM 不触发）——改停机前经 `/stats` 拉。
- run3（1 门红）：**模拟器断言与契约不符**（tags 是 new-api custom 模式专属字段，inspiration 模式按契约不发）——断言改为"inspiration 模式无越界 tags 字段"（方言字段纪律的正确断言）。
- run4：**17/17 全绿 exit 0（权威）**。三轮红门均非栈功能失败（栈本体自 run1 起命令面全活）；迭代证据归档 `probe-iteration-history/`。

## 四、已知边界与后续

1. **外源 provider 端到端**：待用户提供任一真实凭据（中转站 Suno key 或 Stability/MusicGPT key），填 config 即可复跑同冒测（脚本已参数化，换 config 指向真 base_url 即同链路）。属环境项非代码项。
2. **mp3 入内核**：模拟器产物用 wav（基线实证基质）；suno 真产物为 mp3，内核 JUCE 解码面未实测——首跑真源时若 ingest 拒收，按 §8 记录后议转码卡（不本卡内 Silent修）。
3. gen.generate 长任务在单次 HTTP invoke 内同步完成（provider timeout 可配 0..3600s，suno 缺省 600s）；超长任务拆 submit/poll 两工具的形态留后续卡（当前 LLM 面与冒测面均够用）。
4. GEN-CTX-INJECT-1 合入后的注入块收口：`GenerationRequest` 已是汇入点。

## 五、工件索引

- 权威 run：`coord/runs/GEN-API-ADAPTER-1/run4-pass-20261011-232432/`（summary.json 17 门、http/ 24 份逐探针回执含确认卡、logs/ 三进程日志、config/ 用户配置原件、cache/ 下载产物、sim_evidence.json+sim_shape_verdict.txt 请求形状证据、run_meta+双二进制 sha256/mtime、gate_failures.txt 空）
- 探针迭代：`probe-iteration-history/`（run1/run2/run3 summary）
- 栈占用：登记 fa25ae47（占用）→ 释放随回执提交（23:24 末轮后 lsof 7878/5555/5556/5557=0、pgrep VitApp/vitagent=0 亲核）
- 端测覆盖边界（AGENTS §5）：本卡验证面=Mac 两件套真栈（内核+agent）命令/回执面+本地 provider 模拟器 HTTP 面；**未含** Godot 前端、webui 渲染与用户旅程面（本卡改动不涉该两面）。
