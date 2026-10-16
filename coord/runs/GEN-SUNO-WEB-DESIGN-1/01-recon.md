# GEN-SUNO-WEB-DESIGN-1 腿一勘察报告（01-recon）

- 日期：2026-10-16 深夜（Mac 执行侧，公开信息腿）
- 状态：**公开信息部分完成**；真会话验证（登录态探测 ≤10 次真实请求）标注 `[等待用户]`，见 §5
- 方法：社区项目源码考古（两仓库亲读全文）+ 公开搜索情报；**本轮零请求发往 suno.com**（所有材料来自 GitHub 公开仓与公开网页）
- 主要证据源（亲读，非转述）：
  - A = [gcui-art/suno-api](https://github.com/gcui-art/suno-api)（⭐3.3k，最后 push 2026-03-06，Next.js 重写版）——`src/lib/SunoApi.ts` 869 行全文亲读
  - B = [theslumdogemillionaire/suno-client](https://github.com/theslumdogemillionaire/suno-client)（npm `suno-client`，281 测试，最后 push **2026-10-13**，实测锚点新至 2026-10-13）——`docs/STATUS.md`（23.5KB）、`docs/API.md`（64KB/1534 行）、`CHANGELOG.md` 全文亲读
  - C = 公开搜索（ToS 原文片段、封号/断代情报、官方 API 存在性核查）

---

## §1 suno.com 网页 API 契约（社区考古版，2026-10 时点）

### 1.0 基础设施与总形态

| 项 | 值（证据 B，2026-08~10 实测） |
|---|---|
| Studio API base | `https://studio-api-prod.suno.com`（连字符形态；gcui-art 2026-03 版用点形态 `studio-api.prod.suno.com`，B §7.5 记为未验证别名） |
| 认证域 | `https://auth.suno.com`（Clerk 独立域；旧 `clerk.suno.com` 仅残留在 Turnstile PATCH 兼容代码） |
| 传输 | 普通 HTTPS + JSON；**无 Cloudflare JS 挑战阻断**（B 用 Node/axios 默认 TLS 直连成功，明确记录"transport 层是 Node、JA3/JA4 能看穿，但能工作"；curl-impersonate 仅在需要更强伪装时考虑） |
| 路由细节 | **尾斜杠承载语义且不一致**（如 `/api/billing/info/` 带斜杠、`/api/c/check` 不带）；不可做 URL 归一化 |

### 1.1 认证链（三步，全无密码介入）

```
浏览器登录 suno.com（人工一次）
  └─ Clerk `__client` cookie（实测有效期约一年，非一周；死会话通常是别处登录轮换，非过期）
       ├─ ① GET  auth.suno.com/v1/client?__clerk_api_version=2025-11-10&_clerk_js_version=5.117.0
       │     headers: authorization=<__client 原值，无 Bearer 前缀> + cookie + origin/referer
       │     → response.last_active_session_id（session id）
       ├─ ② POST auth.suno.com/v1/client/sessions/{sid}/tokens（同参数；空 body 但仍设 form content-type）
       │     → 顶层 jwt 字段（注意与 ① 的 response.* 嵌套不对称）
       └─ ③ Studio API 调用：authorization=Bearer <jwt> + browser-token + device-id 等头
            **studio API 全程不带 Cookie**（仅 Clerk 两调用用 cookie）
```

关键规则（B §1/§2 实测）：
- **JWT 30 分钟即钝化**：签发 1h，但 ~30 分钟就被服务端以 `Token validation failed` 拒绝（早于 exp）；刷新阈值=剩余<30min 即重铸；每次刷新后持久化 sid+jwt
- **每请求必带头**（小写头名）：`authorization`、`device-id`（稳定随机 UUID；全零 UUID 是最易识别的取值）、`browser-token`（=base64(`{"timestamp":<ms>}`) 再包 `{"token":"..."}`，逐请求重造）、`origin: https://suno.com`、`referer: https://suno.com/`；UA 用真实浏览器串；**不要发 `accept: application/json`**（web 应用不发，发了是破绽）
- 401→刷新 JWT 单次重试；`Token validation failed` 同时意味着"JWT 钝化"和"验证码 token 被拒"两种情况，靠"刷一次再失败=验证码问题"区分
- device_id 来源：cookie `ajs_anonymous_id`（`%22`→`"`、去引号，不做通用 percent-decode），否则随机 UUID v4

### 1.2 三核心端点（生成提交/任务轮询/产物下载）

**生成提交 `POST /api/generate/v2-web/`**（旧 `/api/generate/v2/` 已退役——访问它返回的 `Token validation failed` 与 JWT 钝化同串，会诱发无意义刷新循环）：
- 22 键固定 body（省略关键字段摘要）：`token`(hCaptcha 解或 null)、`generation_type:"TEXT"`、`title`(**必须字符串，null 422**——2026-08-01 起无数客户端因此全灭)、`tags`/`negative_tags`、`mv`(模型键)、`prompt`、`make_instrumental`、`metadata{create_mode: custom|inspiration|cover|remaster, create_session_token(UUID), web_client_pathname:"/create", control_sliders?}`、`transaction_uuid`(每请求新 UUID v4，**服务端去重键，401 重试时不重造**)、continue/cover/persona 各占位 null
- 灵感模式（无歌词）：`create_mode:"inspiration"` + 描述文本放 `prompt`（`gpt_description_prompt` 字段已删除）
- **服务端只强制 `prompt` 一个字段**（2026-09-10 用故意非法 `mv` 探针实测，422 先于 token 校验、不扣点数）
- 响应 `{clips:[...], status}`；**HTTP 200 ≠ 成功**：`status=="error"`(大小写不敏感)或 `clips==[]` 都是失败（点数可能已被报价）
- 计费：每次 10 credits，返回 2 clips
- 模型键按账号动态（Pro 账号实测只有 v6 键、默认 `chirp-hawk`；常量 `chirp-fenix` 在该账号 403）——**必须从 `GET /api/billing/info/` 的 `models[].can_use/is_default_model` 动态解析**，403=计划拒绝、401=会话死

**任务轮询 `GET /api/feed/?ids=<id1>,<id2>`**：
- **一次最多 2 个 id**（≥4 个静默只回前 2 个——轮询器会永远等不存在的完成）
- 响应是**裸 JSON 数组**（非 `{clips:[...]}`；与 `POST /api/feed/v3` 库分页的包装形状相反）
- clip 状态机：终态=`complete`/`error`（大小写敏感精确匹配），非终态一律当 pending（集合开放，勿枚举）；失败原因在 `metadata.error_message`（审核拒绝不可重试，同 prompt 必再拒）
- 轮询策略（B §4.3）：先取后睡、退避 5→10→15s 封顶、完成度按**请求的 id 集**判定（缺 id=pending）、超时 600s；端到端实测 145~173s complete
- 长轮询跨 JWT 钝化窗：每个 feed 调用独立包 401 单次重试

**产物下载（2026-09-10 大改，三件事同时发生）**：
- `clip.audio_url` 现为**哨兵值** `https://studio-api-prod.suno.com/api/forbidden`（回溯生效，50/50 存量 clip 全中）——所有依赖 audio_url 真值的旧客户端变下载期 403
- `media_urls` 是**加密**流（web 播放器路径：CDN 取密文 + `POST /api/mango/rights` 匿名取 AES-GCM 包裹的 key/iv/glt 解密；不动下载配额，但这是播放器 DRM 路径，**不建议产品化采用**）
- 正路：`GET /api/download/clip/{id}?format=mp3|wav|m4a|mp4` → `{ok,status,download_url}`（预签名 S3 URL；wav/m4a/mp4 先回 `processing` 需轮询；flac/ogg/opus 拒绝）；mp3 实测 44.8s/48kHz 立体声/内嵌封面 APIC
- **解锁计量制**：`POST /api/download/authorize {item_id,item_type:"clip"}` 花 1 次下载配额（Pro 实测 `current_period_downloads_limit: 20`/期）；锁定态 GET 返回 `not_authorized` 且**不扣配额**（可免费探测跳过）；解锁按 clip 永久、解锁后再取免费；CDN 下载**不带任何 studio 头**（预签名 URL），连接 20s/读 60s/总 600s 独立超时

**辅助端点**（本卡相关性高的）：`GET /api/billing/info/`（会话活性 canonical 探针+模型目录+下载配额）、`POST /api/c/check {"ctype":"generation"}`（验证码预检，`required` 字段缺失必须 fail-closed 当 true）、`POST /api/generate/lyrics/`+`GET .../{id}`（免费歌词生成）。

### 1.3 "网页会话可直用" vs "需要浏览器上下文"——分类结论

| 环节 | 直用普通 HTTP（cookie+JWT+拟浏览器头） | 需要真浏览器 |
|---|---|---|
| 认证/铸 JWT | ✅（B 全程 Node 直连） | 不需要 |
| 生成提交 | ✅ **当且仅当** `/api/c/check` 返回 `required:false` | ⚠️ 验证码门开启时：hCaptcha 需浏览器渲染 + 解题（gcui-art 用 rebrowser-playwright+ghost-cursor+**付费 2Captcha**；B 的立场=不解，fail-closed 报 `captcha_required`，人工逃生口=**在 suno.com UI 里手动生成一首歌，服务端清门**，实测~40 分钟也可能自愈） |
| 轮询 | ✅ | 不需要 |
| 下载 | ✅（预签名 URL 裸 CDN） | 不需要 |
| TLS 指纹 | Node/Go 默认 TLS ≠ Chrome（JA3/JA4 可识别）但**实测未被阻断**（2026-10-13 锚点） | 仅当未来 Cloudflare 升级阻断才需要 curl-impersonate/真浏览器 |

**结论：停止条件（"cookie 不够、必须完整浏览器上下文"）未触发**——主链路纯 HTTP 可走；验证码门是条件性障碍且有人工逃生口，非结构性阻断。

---

## §2 社区情报：项目版图、断代史与风控证据

### 2.1 项目版图（2026-10 时点）

| 项目 | 状态 | 形态与要点 |
|---|---|---|
| gcui-art/suno-api（A） | 活跃至 2026-03-06，⭐3.3k，100 open issues | Next.js 服务；`SUNO_COOKIE`（DevTools 手工拷贝 Cookie 头）+ `TWOCAPTCHA_KEY`（**付费**，README 明言"Suno 不断要求 hCaptcha，免费手段无法解"）；内嵌 rebrowser-playwright + ghost-cursor 解验证码；Android 客户端伪装头（`x-suno-client: Android prerelease-4nt180t 1.0.42`）+ 随机 Mac UA（"Mac 系统通常触发更少 CAPTCHA"）；自己暴露 OpenAI 兼容面 |
| theslumdogemillionaire/suno-client（B） | **最新**（push 2026-10-13），npm 包 | TS 库+CLI；不解验证码（fail-closed+人工逃生口）；`login --from-browser` 用 node:sqlite 直接从本机 Chrome/Brave/Edge/Chromium cookie 库提取会话（含 `ajs_anonymous_id` 稳定 device id；Safari 不在此列）；canary 免费契约探针+响应漂移基线检测；诚实文档文化（每条实测带日期） |
| ggozad/suno-api | **仓库已 404**（消亡） | 2024 代 `__session` cookie + studio-api.suno.ai 路线的知名项目 |
| SunoApi/SunoApi.org | **仓库已 404**（消亡） | 账号池 token 管理路线 |
| 商业中转站（sunoapi.org/Kie/AIMusicAPI/AceDataCloud 等） | 活跃 | 用户已裁定不走（2026-10-16 决策§二） |

### 2.2 断代史（时间线，全部有实测锚点）

- **2024 代**：`__session` cookie（Firebase/Clerk JWT cookie）+ `studio-api.suno.ai` + `/api/generate/v2/`——整代死亡；基础设施级证据：第三方工具经该域收到 Cloudflare `503 Service Suspended`（OmniRoute issue #14224）
- **Clerk `__client` 代**：认证迁 `auth.suno.com` 三步链；base 迁 `studio-api(-prod|.prod).suno.com`
- **2026-04-07**：`v2-web` 创建路径捕获
- **2026-08-01**：`title: null` 开始 422——"最完整的开源客户端断了两个星期，134 个测试没发现"（B 立项动机）；Rust 参考客户端 HEAD 至今未修
- **2026-08-17/18**：`v2` 正式退役（含诱骗性错误串）；`feed/v3` 库分页出现；`captcha_version` 1→2（服务端灰度）；Free/Pro 分层语义实测（403=计划、401=会话死）
- **2026-09-10**：**下载三连变**——audio_url 哨兵化（回溯生效）+ media_urls 加密化 + 下载计量化（20/期）；`__client` 有效期修正为约一年
- **2026-10-13**：`/api/mango/rights` 播放路径文档化（最新实测锚点）

**断代频率评估：高**。约 6 周内三族破坏性变更（创建路径/字段类型/下载体系），且哨兵化是回溯性的（存量产物同样失效）。缓解手段（B 已验证）：免费 canary（422 先于 token 校验、不扣点、验证码门开启时仍有效）+ 响应形状漂移基线 + 严格错误分类。

### 2.3 风控证据（如实分级）

- **验证码门=行为信誉驱动**：B 实测同一账号 40 分钟内 `required` false→true→false 自行翻转（服务端灰度，非客户端行为定罪）；但"30 分钟 4 次创建"是明确嫌疑画像（Free 账号实测）；gcui-art 开发期"探测爆发把测试账号打到验证码门"。**门开启的创建被拒发生在扣点之前**（30 点前后不动）
- **封号先例**：存在（r/SunoAI 有无预警封号个案），但**未找到"付费账号低频自动化→封号"的专门实证**；最接近的定性是 B 文档原话："A ban lands on a paid account that presumably holds the commercial rights to your tracks. Decide that is acceptable before building on this."（付费账号封号损失=商业权+曲库，决策前置）
- **未发现"近期批量封自动化账号"证据**——卡面停止条件第二支未触发
- **条款**：[suno.com/terms-of-service](https://suno.com/terms-of-service) 明文禁止 "data mining, robots, scraping, or similar data gathering or extraction methods"——本线路在条款文义范围内，属用户自有账号自担风险（用户 2026-10-16 已裁定方向；本报告如实记录边界）
- **官方 API**：截至 2026-10 **无公开官方 API**（2026-01 Make.com 社区+Reddit 深挖双确认；GitHub 搜索无官方 SDK）——降级路径只剩中转站（已裁定不走）
- 平台画像利好：Mac 触发更少验证码（gcui-art 实测经验）；B 证明不解验证码也能运营（fail-closed+人工逃生口）

---

## §3 与 genapi 底座对接评估（代码亲读）

### 3.1 现底座能力（agent/internal/genapi，commit fe86bfea 亲读）

- `AuthConfig`：单值凭据附加（bearer/header/query/**cookie 单名=单值**，`cookie_name` 默认 session；注释明言 "direct web-session reverse-proxy deployments use this"）
- `ProviderConfig.Headers`：任意附加头——**但 `call()` 中固定 UA `vit-daw-genapi/1` 在 Headers 循环之后设置，会覆盖用户 UA 头**（config 可配 UA 也无效；对 suno 风控是硬伤，也是后续任何卡的已知缺陷）
- 可配 `generate_path/poll_path/poll_method/task_id_fields/status_field/done_statuses/fail_statuses/url_fields/base64_fields`（含 `data[].x` 数组路径语法）
- `suno_relay` dialect=中转站契约（`POST /suno/submit/music`→`{"code":"success","data":"<id>"}`、`GET /suno/fetch/{id}`→`data[].audio_url`），**非** suno.com 网页契约

### 3.2 网页契约 vs 底座表达力——逐项差距

| 网页契约要求 | 底座可表达？ |
|---|---|
| Clerk 三步认证（cookie→sid→JWT），JWT <30min 重铸、401 单次重试 | ❌ 认证是有状态多调用协议，静态凭据附加表达不了 |
| 逐请求 browser-token 重造 + device-id + 精确五头 + 禁 accept 头 | ❌（Headers 可带静态头，browser-token 是逐请求动态的） |
| 创建返回 2 clips → 轮询需双 id 逗号拼接且**每请求≤2 id** | ❌ 单 taskID 插值模型（`/fetch/{task_id}`） |
| 轮询响应为**裸数组**、完成度按请求 id 集判定、缺 id=pending | ❌（对象路径+status_field 模型） |
| audio_url 为哨兵 → 真下载走 `/api/download/clip/{id}` + 解锁决策 + 预签名 URL 裸 CDN | ❌（url_fields 直接抓 audio_url=必撞 403 哨兵） |
| 200-with-error 语义、`Token validation failed` 双义、路由级 404 判漂移 | 部分（classifyHTTP 有基础，语义分支缺失） |
| 尾斜杠承载语义、`?ids=` 查询构造 | ⚠️（joinURL 归一化风险，需实测） |

**评估结论：网页契约无法用现有 dialect+config 直接表达**（差距是结构性的：认证协议、多 clip 轮询、下载三段式），"新增 suno_web dialect 直连"路线要把 Clerk 会话机、验证码预检、下载计量决策全部写进 agent 生产码——风险面与断代维护成本耦合进主仓。

**推荐形态：换 transport——本地反代边车（sidecar）服务暴露中转站形状的契约，genapi `suno_relay` dialect 零改动消费**：
- 边车持有全部网页会话机（Clerk 链、JWT 刷新、browser-token、验证码 fail-closed、节流拟人化、下载解锁策略）
- 对上暴露 `POST /submit/music`→task_id、`GET /fetch/{task_id}`→`{status, data:[{audio_url,...}]}` 形状（audio_url 由边车改写为本地下载路由或直接填预签名 download_url）——与 `suno_relay` 的 taskIDFields/statusField/urlFields 逐字段对齐
- 好处：① agent 零生产改动（config 加一条 localhost provider 即接入，REALVERIFY-1 续跑同链路）；② 断代维护收敛在边车单点（对照 B 的 canary 手法）；③ 会话凭据只存在边车的用户级文件（0600），不进 agent 配置面
- 具体服务形态（独立进程 vs agent 内 provider）、会话文件布局、节流参数、WAV 计量策略：**归腿二 02-design.md**（待真会话腿证据后定稿）

### 3.3 已发现的底座缺陷（无论哪条路线都该修，另立卡）

`call()` 固定 UA 覆盖用户 Headers 中 UA 的顺序缺陷（genapi.go L160-167：Headers 循环先、固定 UA 后）——对任何"拟浏览器头"场景都是硬伤。属 genapi 文件域，不在本卡文件域内，**仅登记不改**（AGENTS §11）。

---

## §4 风险评估（按 GEN-API-ADAPTER-1 ruling 风险四条口径更新）

| # | 风险项 | 量级判断 | 依据与缓解 |
|---|---|---|---|
| 1 | **非官方接口断代** | **高**（结构性） | 6 周三族破坏性变更+回溯性哨兵化（§2.2）。缓解：契约快照锁定+免费 canary 探针（422 先于 token、不扣点、门开启时仍有效）+严格错误分类（schema_drift 单列）+fail-closed；断代发生=边车单点修，agent 零感知 |
| 2 | **自有订阅账号风控/封号** | **中**（行为相关，非定罪） | 证据：验证码门随行为画像开启（30min 4 create 嫌疑）、无预警封号个案存在、无"低频付费自动化→封号"专门实证（§2.3）。缓解清单：单账号单并发、低频+间隔随机化、绝不自动重试 429/403、无探测循环/健康检查轮询、验证码门 fail-closed 上报+人工逃生口（UI 手动生成一首）、下载解锁仅显式决策（B 先例：库从不自动调 authorize）、Mac 平台天然少验证码 |
| 3 | **条款合规** | **明确越界文义，用户自担**（已裁定方向） | ToS 明文禁 robots/scraping（§2.3）。边界如实：个人研究用途+自有账号+低频；不逃避检测（不解密 DRM 路径、不做指纹伪装升级、不碰账号池）；产物权利按订阅条款归用户 |
| 4 | **中转站第三方** | **本线路消除** | 直连自有会话，无第三方见凭据/产物/流量（对比 ruling 时点的中转站风险：整项不适用） |

**净结论**：本线路用 #1（高但可工程化缓解）+#2（中，行为自律可控）+#3（文义越界但用户已裁定）换掉 #4（第三方信任）与中转站费用；主链路无浏览器依赖（停止条件未触发）；最大单点不确定性=验证码门行为学（真会话腿重点取证项）。

---

## §5 真会话腿计划（`[等待用户]`）

**需要用户配合（一次性，约 2 分钟）**：
1. 在 Mac 本机浏览器（Chromium 系：Chrome/Brave/Edge/Chromium 任一，Safari 不支持 cookie 库直读）登录 suno.com 并保持订阅有效
2. 授权执行侧从该浏览器 cookie 库提取 `__client`（+`ajs_anonymous_id`）到 `~/.vit/suno_web_session.json`（0600，**不入仓不入卡面不入工件**；备选=用户从 DevTools 手工拷贝 Cookie 头粘给一次性导入命令）
3. 在场配合一次探测运行（≤10 次真实请求预算，见下）

**探测预算（严格 ≤10 次真实请求，全部计数落盘）**：
| # | 请求 | 目的 |
|---|---|---|
| 1 | `GET /api/billing/info/` | 会话活性+计划/模型目录/下载配额基线 |
| 2 | `POST /api/c/check` | 验证码门状态（重点取证项） |
| 3 | `POST /api/generate/v2-web/`（真实生成，10 credits/2 clips） | 创建契约实证 |
| 4-8 | `GET /api/feed/?ids=` ×5 @30s | 轮询契约+终态语义实证 |
| 9 | `GET /api/download/clip/{id}?format=mp3` | 下载路径实证（锁定态先探测不扣配额；解锁与否=**用户在场时显式决策**，每期仅 20 次） |
| 10 | 备用（wav processing 轮询或 401 刷新验证） | — |

若 #2 返回 `required:true`：停止后续生成类请求，仅做免费探测（billing/锁定态 download GET），验证码门行为学如实记录——这是设计腿节流参数的直接输入。

**探针脚本不入仓**（留本 run 目录），key/cookie 材料零入仓零入工件（卡面端测边界条款）。

---

## §6 本轮边界声明

- 本报告全部结论来自公开信息（社区仓库源码亲读+公开搜索）；**"实测"字样均指证据源 B/A 的实测锚点，非本会话亲测**——真会话腿（§5）完成后回填本仓验证栏
- 停止条件两支均未触发（§1.3/§2.3）
- 零生产代码改动；本轮零请求发往 suno.com
- 02-design.md 待真会话腿证据后定稿（服务形态/节流参数/计量策略三处依赖实测输入）
