# GEN-SUNO-WEB-DESIGN-1：Suno 官网订阅网页反代勘察+设计（两腿）

- 发卡：Mac 决策侧 / 2026-10-16 深夜（依据=用户裁定"把官网订阅的 Suno 通过网页端直接反代出来"，见 [decisions/2026-10-16-pca-b-native-and-gen-source.md](../../decisions/2026-10-16-pca-b-native-and-gen-source.md) §二）
- 派发确认：已确认（用户 2026-10-16 深夜方向裁定）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 126；目标仓库=Mac 本机
- 优先级 / 预估 / 依赖：P1 / 1 天（勘察 0.5+设计 0.5）/ 无（不挡 GENSHELL 主线，与 AGENT-1 并行）
- 模型分级：L2 / GLM 执行会话（涉网页契约抓取与风险评估判断）
- **[等待用户:提供 suno.com 订阅账号会话配合（勘察腿需要——浏览器登录后导出 cookie/或本地登录配合一次）；用户不在场时先做公开信息腿（接口形态社区情报/开源 suno-api 类项目契约考古），真会话腿留用户轮]**

## 腿一：勘察（报告入 coord/runs/GEN-SUNO-WEB-DESIGN-1/）

1. **suno.com 网页 API 契约**：生成提交/任务轮询/产物下载三端点的真实形态（路径、headers、认证载体 cookie/session token/CSRF、参数 schema、产物 URL 时效）；区分"网页会话可直用"与"需要浏览器上下文（指纹/JS 挑战）"两类。
2. **社区情报**：开源 suno 反代/客户端项目的契约版本与失效史（接口断代频率、最近可用版本），风控报告（自动化检测/封号先例）。
3. **与 genapi 底座对接评估**：AuthConfig cookie 会话形态（config.go 已预留 "direct web-session reverse-proxy deployments"）是否够用；suno_relay dialect 的请求/轮询结构与网页 API 的匹配度——复用换 transport vs 新增 dialect（如 suno_web）两案权衡。
4. **风险评估如实上呈**：自有订阅账号自动化的风控/封号风险、非公开接口断代风险、条款合规边界——按 GEN-API-ADAPTER-1 ruling 风险四条口径对照更新。

## 腿二：设计件（同一 run 目录，编号 01-recon.md / 02-design.md）

反代服务设计：本地服务进程形态（独立小服务 vs agent 内 provider）、会话管理（cookie 存储/过期刷新/登录失效 fail-closed 上报）、请求模式拟人化节流（低频+间隔随机化，降风控暴露）、产物抓取与 WAV 魔数校验、与 ~/.vit/genapi_providers.json 的配置映射、REALVERIFY-1 续跑衔接路径。

## 文件域

coord/runs/GEN-SUNO-WEB-DESIGN-1/**（报告与设计件）；勘察探针脚本不入仓不入生产（留 run 目录）。**零生产代码**——腿二设计通过后另立实现卡。

## 验收标准

腿一四节齐+腿二设计件（含推荐 dialect 对接形态与服务形态）+风险评估节（含封号风险量级判断与缓解清单）+需要用户配合项清单。

## 停止条件

网页 API 需要完整浏览器上下文（指纹/JS 挑战过不去，cookie 不够）→ 上交换方案（中转站/官方 API/内嵌浏览器路径另议）；风控证据强（近期批量封自动化账号）→ 上呈用户裁定是否继续。

## 端测边界

零生产代码零外呼承诺不适用于本卡——勘察腿允许向 suno.com 发真实请求（用户自己的订阅，低频 ≤10 次探测）；产物不入库；key/cookie 材料只进用户级本地文件，不入仓不入卡面不入工件。

- 领取：（2026-10-16 23:05 +0800 / origin/main=ca86b226d8af0bc85dc3390aa49028a3c9f3697a2 / GLM-5.3 L2 Mac 执行侧 ZCode 会话（本卡会话，用户口令指定领取）/ 分支=port/gen-suno-web-design-1（worktree /Users/timozty/Documents/Vit-DAW-gensunoweb1）+ 协调 worktree Vit-DAW-gensunoweb1-coord（coord/gensunoweb1-claim）/ 领取提交=本提交；领取时核对：卡在 todo 派发已确认（用户 2026-10-16 深夜裁定）、依赖无、doing 空、文件域零冲突、真栈零占用（纯勘察+设计，无栈需求）。本轮=勘察腿公开信息部分先行（用户不在场），真会话腿 [等待用户]）
- 回执：（报告链接 / 风险评估结论 / 用户配合项状态 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
