# 生成源候选调查 + 用户手测清单（2026-10-17，Mac 决策侧）

- 依据：用户 2026-10-17 指示——126 卡筛除付费手测内容分开汇总；点名测试 YUE2 / MiniMax（M3）/ HappyShrimp。
- 本文件 = ①候选模型调查结论（即时搜索，2026-10-17）+②用户手测汇总清单 +③排序修订。搜索证据链接见文末。

## 一、候选模型调查结论

| 候选 | 是什么 | 官方订阅/API 入口 | 对接 genshell/genapi | 结论 |
|---|---|---|---|---|
| **MiniMax Music 3.0**（用户记的"M3"=Music 3.0，注意 M3 是其 LLM 产品线） | MiniMax 最新全曲模型：一次生成作曲+编曲+演唱+制作；Music 2.5（2026-01）起官网与 API 平台同步上线 | ✅ **官方开放平台 API**（platform.minimax.io，Pay-as-You-Go 按次计费） | ✅ 正统 API 型——genapi 现成 dialect（async_poll/sync 类）直接承载，零适配风险 | **REALVERIFY-1 最快真源路径**：注册+小额充值+key 即可端到端 |
| **HappyShrimp**（阿里 ATH） | 一句话→完整歌曲（歌词+人声+编曲），v1.0 beta/v1.1 | 官网 Studio（happyshrimpmusic.com，免费试用）；API 是否上百炼**未确认**（待深查阿里云公告） | API 确认前无法接；无 API 则与 Suno 同属网页形态 | 免费可先测质量；API 入口=后续公开信息腿 |
| **YuE2**（m-a-p 开源 3B） | 开源全曲模型（歌词→歌，48kHz，社区评质量对标 Suno v5/v6） | 无官方 API——开源权重自部署（GitHub+HF）；第三方托管（WhiteFiber/yueai.ai 等）非官方 | 本地 3B 音频模型 MacBook Air 跑不动；第三方托管可作 sync_binary 接 | 自部署不作主路；第三方托管列为可选实验 |
| Suno（对照） | 全曲质量标杆 | 官方 API 不对个人订阅开放（独立付费层）；消费级=网页/App；非官方 API 已被主动关停（studio-api.suno.ai 503） | 主路=网页自动化/反代（GEN-SUNO-WEB-DESIGN-1）；SongSmith 先例在架 | 质量主源线，设计驱动，慢路 |

**排序修订（决策侧）**：REALVERIFY-1（真源端到端验证）的最快解锁=**MiniMax Music 3.0**（官方 API+小额付费即可），不再等 Suno 反代线。Suno 网页线独立推进（质量主源）。genapi 的 suno_relay dialect 保留（中转站生态仍活跃，可作 Suno 应急腿）。

## 二、用户手测清单（付费/手动项已从 126 卡筛出，一并执行）

按成本从低到高排序：

1. **HappyShrimp 免费试用**（零成本）：happyshrimpmusic.com 注册→用我们的典型用例（一句话生成伴奏/全曲）试 2-3 首→记质量体感；顺带看官网/设置里有无 API 入口或公告（有就截图记档）。
2. **MiniMax Music 3.0 小额付费**（~¥50-100 量级）：注册 platform.minimaxi.com（国内站）或 platform.minimax.io→开通音乐模型 API→小额充值→生成 API key→**key 自存**（交执行侧时只进 ~/.vit/genapi_providers.json，不入仓不入卡）→可先在平台自带 playground 生成 1-2 首确认质量。完成后告诉我 key 已就位，我派 REALVERIFY-1 续跑（执行侧代填配置+真源端到端）。
3. **Suno 订阅**（~$24/月，放最后——先确认前两个的质量再决定是否需要）：购买 Pro→登录 suno.com→手动生成 2-3 首体验→若继续，配合 131 卡真会话腿（勘察报告 01-recon.md §5 已有计划：Chromium 系浏览器登录后提取 `__client` 会话到 ~/.vit 用户级文件 0600+在场配合 ≤10 请求预算探测 billing/c-check/生成/轮询/下载）。执行侧勘察结论先行参考：主链路纯 HTTP 可走、验证码有条件性障碍但有人工逃生口、断代风险高可工程化缓解。
4. **（可选）YuE2 第三方托管试用**：yueai.ai / WhiteFiber 任一注册试用 1-2 首（质量对照用，不注册也行）。

每项做完把体感（质量/速度/价格）回我一句即可，我记录归档并更新选型决策。

## 三、卡面变更（同批提交）

- GEN-SUNO-WEB-DESIGN-1（池序已改 131，doing 中）：公开信息腿已完成（四节齐，主链路纯 HTTP 可走+边车对接推荐），真会话腿本就挂 [等待用户]——用户付费/在场项已归入本清单第 3 项，卡面无需再改（拆分意图已由执行侧形态+本清单共同达成）。
- 附带上交登记：执行侧发现 genapi call() 固定 UA 覆盖用户 Headers 的顺序缺陷（genapi 文件域，仅登记未改，AGENTS §11）——待决策侧开小修复卡。
- GENAPI-REALVERIFY-1（125）注记更新：解锁最快路径=MiniMax Music 3.0 key 就位。

## 搜索证据（2026-10-17）

- Suno 官方 API 不对个人开放/API 独立层：reddit.com/r/SaaS 官方与否讨论、sunor.cc/blog/suno-api-pricing-2026
- Suno 主动关停非官方 API：GitHub issues（studio-api.suno.ai 503）；ToS 风险：aimlapi.com/blog/the-suno-api-reality
- SongSmith 网页自动化先例（Chrome 扩展在架）：songsmith.studio、chromewebstore.google.com/detail/songsmith-ai
- YuE2：github.com/multimodal-art-projection/YuE、huggingface.co/m-a-p/YuE2-3B
- MiniMax Music 3.0/2.5 API：ir.minimax.io、minimaxi.com/news（music-01 起 API 在线）、platform.minimax.io 定价
- HappyShrimp：happyshrimpmusic.com、alibabacloud.com/blog（v1.0 beta 官宣）

## 四、使用途径与封装架构调查（2026-10-17 第二轮，决策侧即时搜索）

用户核心问题：各家使用途径能否统一成一种通用方式（API/网页）？→ **不能完全统一，但两层分界清晰**：

| 途径类型 | 覆盖 | 封装归属 |
|---|---|---|
| 聚合 API 型 | **fal.ai**（一个 key：MiniMax Music 1.5/2.0/2.5/2.6+ACE-Step+Sonauto V2+Lyria+ElevenLabs，$0.03-0.15/次，队列式 API）/ Replicate | **通用层零新增代码**：形状与 genapi async_poll dialect 对口，一个 provider 配置多模型 |
| 官方 API 型 | Stable Audio、Lyria（Gemini API）、ElevenLabs、MiniMax（见变数） | 通用层（参数化 dialect 承载） |
| 网页-only 型 | **Suno**（官方 API 不对个人+fal 无官方合作+非官方 API 被打击+WMG/BMG 行业授权路线）、HappyShrimp（beta 待确认） | **专用适配插件**：Suno 网页边车（131 卡）；HappyShrimp 视 API 归层 |

**MiniMax 澄清（用户疑问）**：API 真开放（开放平台 10 万+开发者，非自家平台专属，fal/Replicate 正规转售）；**变数=付费音乐 API 2026-08-20 起对新用户关闭**（存量可用；Music 3 开源上 HF 但 3B 级 Air 跑不动）→ 新注册需实测可开性，不可开走 fal 同模型。

**封装结论（套壳架构裁定方向）**：通用 provider 层（现有 genapi）覆盖聚合型+官方型=**大多数品牌不需单独适配**；专用适配仅 Suno 确定（+HappyShrimp 视情况）——用户"优先给 SUNO 和个别品牌做单独适配"直觉正确且范围更小。REALVERIFY-1 最快真源路径修正：**MiniMax 官方 → fal.ai**（注册+$10 充值即测多家，横向质量对比同账号完成）。摩擦点：fal 需外网（Mac 7890 代理可用）+外币支付；纯国内备选=MiniMax 中文站（若新用户可开）或中转站（suno_relay 已实现，非官方）。

**手测清单修正（第二节第 2 项）**：MiniMax 项加实测步骤——注册开放平台→确认音乐 API 新用户可开性；**不可开即转 fal.ai**（$10 充值，playground 横测 MiniMax Music 2.6/ACE-Step/Sonauto 等，一次覆盖大部分 API 型候选）。其余项不变。

搜索证据（第二轮）：platform.minimax.io/docs/api-reference/music-generation（8/20 新用户关闭条款+HF 开源 Music 3）；fal.ai/explore/text-to-music-apis（聚合阵容与定价）；TeamDay 2026-09 评（"Suno 无官方 API，第三方皆非官方包装，勿在其上建产品"）；Billboard（Suno×WMG）/BMG 2026-08 行业授权；fal.ai/learn 2026-03（fal 官方音乐模型阵容=MiniMax/ACE-Step/Sonauto，无 Suno）。

## 五、通道优先级终裁（2026-10-17 第三轮，覆盖 §四建议）

用户裁定**网页侧为主路线**（[decisions/2026-10-17-gen-web-first.md](../decisions/2026-10-17-gen-web-first.md)）——§四"API 为主、网页兜底"建议被推翻：必有网页=商业存续性最硬（API 说关就关，MiniMax 8/20 实证）；主流程解耦论证成立（Vit 机架输入→网页产出→下载导入，对面复杂度封在 profile 层）。架构=平台化边车引擎（通用流程）+薄站点 profile（五槽位：认证/提交/轮询/下载/风控）；API provider 降并行通道保留。三工程约束入设计件：profile 非零工作量/契约自检腿/低频拟人节流默认。
