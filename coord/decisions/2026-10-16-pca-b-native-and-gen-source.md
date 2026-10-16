# 决策记录：PCA 生成壳准入方案 B + 生成源战略（2026-10-16 深夜，用户裁定）

## 一、PCA-1 认证面方案：B——GenShell 作为原生打包插件获得永久准入

- **用户裁定（2026-10-16 深夜）**：GenShell 不是第三方音频效果器，是我们设计、与 Vit 打包在一起的原生"效果器"，不需要第三方适配流程——用户只需在 GenShell 里做 API 配置即可使用；准入按方案 B 直接升级为永久认证。
- **决策侧落地设计裁决（实现用户原则）**：永久准入**不经过一次性 token 介质**，改由**代码级注册事实驱动**——`processorregistry` 已登记 generation_shell 为原生 load_only 家族，实现形态=agent 启动时对 registry 声明的 native load_only 家族**幂等 seed 一条永久 v2 attestation**（绑定 GenShell.vst3 bundle 指纹），装载门/admission/audit 全部走现有查询路径，**三道现行门（chat 认证入口/token 通道/typed coverage）零改动**（token 通道保留其一次性授权语义不动）。
- 理由：用户原则="原生打包=开箱即用"。token 是运行时一次性授权介质，为它加签发能力反而把临时介质升格；注册行是经正常代码审查入库的代码级事实，证据强度更高且零人工流程。撤销语义：revoke 在运行时生效（同进程装载即拒）；重启 re-seed 属 native 家族会话级行为，卡面如实声明。
- 详见修订卡 GENSHELL-PCA-1（blocked→todo，方案入卡面）。

## 二、生成源战略：Suno 官网订阅网页反代专线 + 其他 API 型统一走 genshell/genapi

- **用户裁定（2026-10-16 深夜）**：
  1. MusicGen/本地模型不采用（MacBook Air 算力不合适——用户判断，决策侧撤回先前本地兜底建议）。
  2. **Suno 重新设计思路**：不走第三方中转 API，目标是把**官网订阅的 Suno 通过网页端直接反代出来**（用户持有 suno.com 订阅，本地反代服务持登录会话，把生成请求转发为网页端 API 调用并取回产物）。
  3. **其他可 API 配置的生成模型统一走当前这套 genshell 套壳**（genapi 适配器三 dialect 承载）。
- 决策侧注记：genapi `config.go` AuthConfig 已预留 cookie 会话认证形态（注释原文 "direct web-session reverse-proxy deployments use this"），底座支持此方向。风险如实：网页端接口非公开（断代风险）+自有订阅账号自动化存在风控/封号可能——先勘察（GEN-SUNO-WEB-DESIGN-1，池序 126）后实现，风险摸清前不写生产代码。
- GENAPI-REALVERIFY-1（125）解锁前提相应修订：真源=Suno 网页反代线落地后以真源续跑（或用户先启用任一 API 型源）；本地 MusicGen 路线作废。
