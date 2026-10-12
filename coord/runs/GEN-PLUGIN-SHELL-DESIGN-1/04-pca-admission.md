# 设计件四：PCA 接入面（可管可控准入语义）+ 落轨分工 + genapi 关系定位

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 卡面对应：设计五件之 4——"壳子=可管可控的准入语义；音频产物落轨走宿主渲染+ingest 的分工"；设计前提 4（"壳子接 PCA、可控即达标；未来厂商自家插件形态兼容（标准通道天然可达）"）。
- 交叉引用：docs/PCA_PLUGIN_LOAD_GATE_V1.md（历史契约）；docs/PROCESSOR_CONTROL_ATTESTATION_V1/V2.md（attestation 现行语义）；件一 §3（T1 参数面）/件二 §6（落轨双路径）。

## 1. PCA 现状事实（本卡勘察锚点）

- **加载门只在 agent 侧**：harness `agentPluginLoadCommand` 覆盖 `rack_add_node / rack.add_node / plugin.load_to_rack / instantiate_plugin / plugin.instantiate`（harness.go:1942-1949 亲验），在 Invoke 管线内核派发前执行（harness.go:930-934）；**内核侧无门**、DAW/UI 手动加载走原生 rack 服务不受限（harness.go:1951-1954 注释亲勘）。
- **准入=family 级**：processorattestation `QueryInstalledAdmission`/`QueryCurrentAdmission`（eligibility.go:68-85/130-166）；未知 family fail-closed（`unsupported_processor_family`，eligibility.go:125/165）。
- **family 词表现状=9 个**（static_eq/broadband_compressor/limiter/gate_expander/de_esser/transient_shaper/multiband_dynamics/spectral_dynamics/clipper——processorintent/intent.go:32-42 亲勘），**无 generator 类**；pluginsemantics 类型集亦无 generator（index.go:286-306）。
- **生成动作现状游离 PCA**：`gen.generate`（genapi_generate 本地工具）仅常规确认、不经 PCA/attestation——"生成准入"是空白面（勘察结论）。
- 认证通道三入口：语义选择/PCA 选择/认证 token（harness.go:1908/1921/1933）；full_project_access 自铸授权从 promoted 目录解析唯一身份（harness.go:1971-1996）。

## 2. 壳的 PCA 接入（v1 范围裁定）

**"可控即达标"的操作语义=三条**（前提 4 的验收化）：

1. **agent 侧装载过门**：壳作为 processor 走 PCA 加载门——新 family `generation_shell`（词表三处扩展：processorintent/intent.go、processorattestation v2 家族表（v2.go:15-23）、processorregistry/registry.go 家族注册），attestation 由本地认证链签发（自产二进制：`FingerprintPath` 对 .vst3 bundle 内容哈希可重复——fingerprint.go:18-71 先例；subject key=format+identifier，types.go:143-160）。经此，agent（含未来语义编排）把壳装上轨是**被准入、被审计**的动作。
2. **作业全审计**：壳的每次生成=job record（state/message/request_hash/source 全程）+回执（落轨结果/失败分类）——生成动作的准入语义由**作业审计面**承载（不是 PCA coverage：PCA 是处理器控制准入，生成是资产生产，两类语义不混编——与现状"生成链走资产面"一致，只把审计钉实）。
3. **密钥不进内核**：provider 凭据只存在 agent 进程（genapi config/env——件四 §3），壳/内核/GUI 零密钥材料。

**v1 明确不做**（边界声明）：
- 壳参数面的 typed apply 工具对（v2 家族的 inspect/apply 钉死模式，local_processor_certification.go:234-247 亲勘）：壳 v1 的控制动词=生成作业（走审计面），非参数写；`generation_shell` 家族 coverage 词表 v1=**空集**（load 准入即可，无 typed 控制承诺）。实现卡若发现 CertifyProcessor 硬性要求非空工具对→停止条件触发，上交换方案（给壳定义 `gen_shell.inspect` 只读工具或改走认证 token 通道）。
- `pluginsemantics` 新增 generator 语义类型：壳分类走 `synth/instrument`（IsInstrument）既有规则即可（index.go:276-312），不动分类词表。
- 手动 UI 装载：维持不经门（既有先例，不因壳改变）。

**厂商兼容路径**（前提 4 后半）：第三方"生成壳"实现同一 T1 参数面+T2 消息契约（件一）→ 经 PCA 同一 family 准入装载——标准通道（IConnectionPoint/参数/chunk 全是 VST3 原生）+ 标准 family 准入，无私有接口依赖。

## 3. genapi 适配层关系定位（任务点名，五条钉死）

1. **协议零实现**：壳不做任何 provider 协议（HTTP/轮询/下载/格式嗅探）；生成执行=agent runner 调 `genapi.Generate(ctx, provider, req)` **原样复用**（genapi.go:82-124 亲验，五方言 sync_binary/sync_json_url/sync_json_base64/async_poll/suno_relay 不加壳侧分支）。
2. **密钥零内核**：provider 凭据=agent 进程的 `~/.vit/genapi_providers.json`/`VIT_GENAPI_KEY_<ID>` env（config.go:16-27/263-272 亲验）；壳 GUI 只见 `Describe()` secret-free 投影（has_api_key 布尔，config.go:275-296）。
3. **目录投影**：provider 列表由 runner 经 `gen_shell.pending` 应答顺带刷新（件二 §4）——配置文件是唯一事实源，壳无配置面。
4. **请求映射**：壳请求→`GenerationRequest` 字段映射固定（件二 §2）；`prompt` 字段=agent 渲染终文填充（统一化单一渲染），适配器转换不删改自动块（统一化件三 §2 验收项沿用）。
5. **新后端零壳改动**：未来任何生成后端（含本地 SVS 服务——GEN-VOCAL-SYNTH-1 案 A 形态的天然后端）=genapi 加 provider 条目/方言，壳零改动即获得（产品演进面钉在 agent 侧）。

**与 GEN-CTX 消费链的关系**：壳=统一化件一 §3"生成插件"消费方的**产品形态**（件三 §1 形式化的共用模板引擎+快照+机检门仍是机制本体；壳把该消费方从"编排器代码路径"升级为"轨道上的用户直操作面"）——`gen_plugin` profile 的两个使用者（AIGC-DRUM-GEN 编排线[后置]与壳）共用同一渲染器，零第二实现。

## 4. 落轨分工（PCA 不管的事）

- **PCA 管装载**（§2），**不管产物**：音频产物落轨=宿主渲染/下载产物→ingest 资产链（件二 §6 fast/print 双路径），审计由 job record+GeneratedAssetService 回执承载——两类准入不混编（§2 条 2）。
- ingest 链零新增面：`aigc_register_job`/`bridge_ingest_generated_asset`/`switch_asset_take`/`set_async_ghost_state` 四命令既有（CommandDispatcher.cpp:2784-2803 注册亲验，四 handler 连续区）；GeneratedAssetService 全语义（媒体池入库/clip 落轨/vit_* 注记/take 栈/job 关联——GeneratedAssetService.cpp:160-308 勘）原样复用。
- **内核唯一新增命令**=`gen_shell.pending`（读命令，RiskDirect；件二 §4）——不触 get_project_state 披露块（GEN-CTX-DISCLOSE-1 域避让）、不触渲染面。
