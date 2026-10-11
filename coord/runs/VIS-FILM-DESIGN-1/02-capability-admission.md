# VIS-FILM-DESIGN-1 设计件二：能力面与准入（film 写作 capability + look 式校验器）

- 归属：VIS-FILM-DESIGN-1 设计基线四件之二
- 勘察底座：能力 v1 全链先例（B2 静态平衡/Agent 语义 EQ）+ 执行验证器体系，锚点见文内
- 定位：比 openfilm 更严——openfilm 的 agent 直接写文件无准入；vit 的 film 写作走 capabilityruntime，**过确定性校验器才能挂载**

## 1. film 写作 capability（声明/路由/执行）

### 1.1 声明（orchestration 层）

挂载点：`agent/internal/orchestration/builtins.go`（`DefaultRegistry()` 现有 6 能力只放声明）。

```go
{
  ID:             "film.writing.v1",
  Family:         "project_specialist_film_writing",
  Description:    "Write VitReel film shots (self-contained frame(t) pages) into the project film directory",
  Effects:        []string{"file_mutation"},
  RiskCeiling:    "bounded_reversible",
  ManifestHash:   "<发布时计算>",
  VerificationRef: "film.writing.verification.v1",
}
```

### 1.2 路由与会话（chat 层，照 Agent 语义 EQ 模板）

- 能力 ID 常量 + `StartFilmChatSession`：`agent/internal/orchestrationruntime/runtime.go`（常量区 :21-26、入口先例 :105）。
- 白名单：`agent/internal/chat/capability_owner.go:24-28` 加 `film.writing.v1`；关键词推断 `inferProjectAwareCapability`（:44）加"成片/AMV/film/VitReel"簇。
- 会话骨架：新文件 `agent/internal/chat/film_runtime.go`，模板=`agent/internal/chat/semantic_eq_runtime.go:177`（横向能力模板，最贴近"新能力"形态）。
- 上下文包：`agent/internal/capabilitycontext/context_manifest.go` 加 per-capability manifest（CapabilityID+Guidance：纯度规则 P1-P5+自包含约束+数据内联规范，Guidance 文本直接引用设计件一 §1——**契约即能力指引**）。

### 1.3 执行（executionruntime 层）

复用能力 v1 骨架（`agent/internal/executionruntime/coordinator.go:49-218`）：Proposal→授权（对话审批复用 `capabilityinteraction/resolver.go:20` 的 approve/reject/revise/narrow）→ 每工程写租约（:84-92）→ `Preflight` → 逐 Action `Apply` → `Verifier.Verify`（:187-204）→ pass 才 `completed`。

新写两个小件：

**FilmFilePort**（实现 `MutationPort`，模板 `agent/internal/executionports/staticbalance_vsp.go:28`）：
- 写入目标限定：staging 区 `<工程>/.vit_agent/<uuid>/film-staging/<提案id>/`；
- Apply=把 LLM 生成的镜头页/manifest 写入 staging；
- Verify pass 后在同一写租约内原子 rename 进 `<工程>/film/`（暂存+rename 模板：`agent/internal/projectworkspace/derived.go:51-65`）——**"过校验后才对外可见"由此实现**；
- 持久化基线走 `executionports.ProjectHistory`（回滚能力）。

**FilmVerifier**（实现 `Verifier`，模板 `agent/internal/executionverifiers/staticbalance.go:44`，`Acoustic` 置 nil）：见 §2。

## 2. look 式确定性校验器（准入门）

校验对象=一次提案的全部镜头页+film.json。采样=每镜头均匀采样 + 段落边界时刻加采（v1 经验值：每镜头 ≥8 帧+全片 ≥40 帧）。

### 2.1 检查项（四类，openfilm look 对齐+vit 加项）

| # | 检查 | 方法 | 判定 |
|---|---|---|---|
| C1 | 非确定性帧 | 每采样帧按**两种打乱顺序各画一遍**（pass1 正序+随机序，pass2 相同帧集不同随机序），像素级比对（容差：逐像素差 ≤2/255，防抗锯齿抖动） | 任一帧超容差 → fail，报帧号+两版截图 |
| C2 | 页面报错 | 收集 console error/exception、`window.film.frame` 缺失/抛错、页面 crash | 任一 → fail |
| C3 | 字体/资源加载失败 | `document.fonts.status`、失败资源请求清单（CSP 拦截的网络请求在本检查里单列——P3 违例） | 字体失败→fail；网络请求→fail（P3） |
| C4 | 文字裁切 | DOM 扫描文本节点 bounding box vs viewport（openfilm look 同款），含 `overflow:hidden` 容器边缘 | 裁切→fail，报元素选择器+截图 |
| C5（vit 加项） | 数据面一致性 | film.json 时长/镜头区间 vs energy_envelope 时长；`data_fingerprint` 校验前后一致（防两遍间数据被换） | 不一致→fail/`data_changed` 中止 |

输出：`verification_report.json`（逐帧结果+失败类+截图路径）落 artifacts（`agent/internal/artifacts/store.go:30` 工件库，JSONL manifest 已带 scope）。

### 2.2 执行环境选型（裁定）

**Node playwright-core + 浏览器多候选自发现/回退链**，理由：
- 与 E2E-WEBUI-1 渲染面门同栈：`scripts/webui_rendered_dom_smoke.mjs:200-227` 已有 playwright-core 多候选目录自发现机制，直接泛化复用，不养第二套浏览器驱动栈；
- openfilm look.mjs/render.mjs（357/157 行）同语言可直搬改造；
- 浏览器获取策略移植 Python 侧已验证的 channel 回退链（bundled-chromium→msedge→chrome，`scripts/e2e_chat_chain_layer_b.py:28-58`，注释实证本网络 playwright CDN 下载会 stall）；
- npx 缓存已有 playwright-core 1.61.1（恰为 openfilm 锁定版本）。

备选（记录不采纳）：Python playwright 1.62（已装，e2e 先例多）——不采纳主因=与渲染面门双栈；Go chromedp/rod——新依赖无先例。

Go↔Node 接线：Go 侧 FilmVerifier 以子进程调 `scripts/film_verify.mjs`（参数=staging 路径+采样配置，输出=report JSON 路径+退出码），模式同 start_render 的命令分发等待（harness 分发先例 `agent/internal/harness/harness.go:1731`）。

### 2.3 校验器只保底线不保审美

复述风险边界（卡面 §风险）：校验器保"不断/不糊/不裁切/确定"，不保"好看"。质量迭代靠 agent 读图回路（第零验证项，见 README §2）+展演排练最佳轮次实录兜底（与 DEFENSE-SHOW-1 保底逻辑同构，live/录播明示）。

## 3. 封堵绕行路径

`workspace_write_file` 工具（`agent/internal/tools/catalog.go:888`，RiskConfirm）是通用写文件面，**无能力级校验**——若放行写 `<工程>/film/` 即绕过准入。封堵条款（归实施卡 B1）：
- `agent/internal/workspace/workspace.go:513` `ensureTextWritablePath` 拒写 `<工程>/film/**`（同拒写 `.vit/.trackionedit` 的既有模式）；
- staging 区同样仅 FilmFilePort 可写（经能力写租约）。

## 4. 音画对位第二 verifier（v2 设计预留，v1 不实现）

**FilmAVAlignmentVerifier**——评分器，非准入门：
- 输入：film.json 的 `shots[].evidence`（切镜决策的证据回指：DOM 动态事件/MOM 关系/RLM 电平）+ energy_envelope segments + 镜头表；
- 检查（示例，v2 细化）：切镜点落在段落边界 ±0.3s 的比例；chorus 入点画面能量（该镜头平均饱和度/运动量代理）与 RLM 电平跃变的方向一致性；副歌/主歌镜头时长分布与段落结构的相关性；
- 输出：对位评分+失配清单，作为 Proposal 附件交 LLM 迭代与用户审阅——**质量反馈环，不是 pass/fail 硬门**（与第一 verifier 的准入门定位严格区分：门保底线，评分器给改进方向）。
- 结构性超越点注记：openfilm 的 agent 对音乐盲切；vit 的 agent 拿观察投影证据切镜——这是 VitReel 对 openfilm 的核心差异主张（论文叙事可用）。

## 5. 与现有能力体系的关系

- film capability 是**首个 Effects=["file_mutation"] 的能力**（现有 6 能力全是混音参数域）——RiskCeiling 定 `bounded_reversible`（写租约+staging+ProjectHistory 回滚支撑）；若主管认为文件域风险需更强口径，升 `review_required`，实施卡 B1 发卡时定。
- 与生成线（GEN）关系：film 写作能力消费生成线产物（工程内生成素材进混音→渲染→数据面），不直接调生成命令；`ask vit` 单入口下，用户说"给这首做成片"→capability_owner 路由到 film capability→能力内可嵌套发起渲染（复用 start_render 工具面）。
- 就绪门：`agent/internal/capabilityruntime/readiness.go:45`（Readiness/CanProceed）——film capability 的 readiness 条件=工程已开+渲染可用（内核在线）+（读图迭代时）image_understanding 路由已启用。
