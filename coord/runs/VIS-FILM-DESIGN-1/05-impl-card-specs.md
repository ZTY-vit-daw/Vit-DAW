# VIS-FILM-DESIGN-1 设计件五：实施卡规格草案（v1 实施线 8 张）

- 归属：VIS-FILM-DESIGN-1 设计基线；按 PROTOCOL §5 模板起草，**发卡人/派发确认留待主管填**，其余字段已达可直接发卡程度
- 设计依据：设计件一至四（同目录 01–04）；文件域互斥已核对（见各卡"并行与资源"）
- 编号规则：VIS-FILM-<批次>-<序号>；领取顺序建议=P0 →（A1∥B1∥B2∥C1）→ B4 → C3 → D1 → D2

---

## 卡 1：VIS-FILM-P0-ROUTE-ENABLE（image_understanding 路由启用+读图回路验证）

- 发卡：（主管填）/ 派发确认：（待主管确认；启用运行时配置需用户知情）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 用户对"启用 image_understanding 路由"点头（第零验证项实测已证明端点收图正确，见 probe/）
- 目标：①`~/.vit/config.json` 的 `multimodalRoutes.image_understanding.enabled` 改 `true`（仅此一键，无代码改动）；②在真实 agent 进程内经 `llm.Client.CompleteImageUnderstanding` 读一张 ≥1280×720 的真实截图并回写描述（验证大图+真实 agent 进程路径，设计探针只测了 16×16 与 go test 进程）；③读图质量记录（能否数清镜头元素/文字），不足则升级路由模型为决策项上报。
- 文件域：`~/.vit/config.json`（用户机器配置，非仓库文件）；验证工件落 `coord/runs/VIS-FILM-P0/`
- 并行与资源：真栈烟测腿 1 次（agent 进程在位即可，不需内核/Godot）；PC-RUNTIME-STACK 登记
- 验收标准：P0 run 目录含 agent 原始日志+读图请求/响应工件+描述文本质量评注；agent 进程读图调用零 panic；`image_understanding_route_unavailable` 不再出现
- 停止条件：路由开启后端点拒收大图（4xx/5xx）→ 记录原始响应，升级为主管决策（换模型/换 provider），不自行改 BaseURL

## 卡 2：VIS-FILM-A1-ENVELOPE（音频特征面：整曲渲染封装+能量包络分析器）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / ~1 周 / 无（与 B/C 线并行）
- 目标：新包 `agent/internal/film/`：①`Render` 封装——发 `start_render{file_path, bit_depth:24, use_master_plugins:true}` → `harness.WaitRender` → sha256（复用 `agent/internal/chat/free_state_d1_render.go:99` 等待模式）；②WAV 解码（24bit/32f·48k）→ 能量包络（hop 50ms/窗 100ms，RMS+peak dBFS+6 频段，频段口径对齐 `VitProductionCoordinator.cpp:170-178`）→ `energy_envelope.json`（schema `vit.film.energy_envelope.v1`，设计件一 §3.2）；③v1 能量法段落切分（双阈值+最小段长 8s）落 `segments`；④源 JSON 落 `.vit_derived/<uuid>/film/`（落盘模式照 `agent/internal/acousticpackage/status.go:136-157`）。
- 文件域：`agent/internal/film/**`（全新包+测试）；**不改** chat/harness 既有文件（WaitRender 已在位）
- 并行与资源：无真栈需求（WAV 用 fixture 单测；渲染链路集成测并入 B4）
- 验收标准：`cd agent && go build ./... && go test ./internal/film -count=1` 退出码 0；fixture WAV→JSON 黄金值比对（含 24bit/32f 两格式、静音段、削波段）；schema 往返测试；能量值与 `analyseL2ProbeFile` 同口径交叉核对（同一 fixture 对拍，容差 ±0.5dB）
- 停止条件：发现 start_render 产物路径/等待契约与 free_state_d1_render.go 模式不符（命令面变更）→ 停止并上交证据

## 卡 3：VIS-FILM-B1-FILM-CAPABILITY（film 写作能力+FilmFilePort+封堵）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / ~1 周 / 设计件一/二（无代码依赖；与 A1/B2/C1 并行）
- 目标：①`orchestration/builtins.go` 加 `film.writing.v1` 声明（设计件二 §1.1 字段）；②`orchestrationruntime/runtime.go` 常量+`StartFilmChatSession`；③`chat/capability_owner.go:24-28` 白名单+关键词簇（成片/AMV/film/VitReel）；④新文件 `chat/film_runtime.go`（模板 `semantic_eq_runtime.go:177`：会话/Proposal/授权骨架）；⑤新文件 `executionports/film_file.go`——FilmFilePort：写入限定 staging `.vit_agent/<uuid>/film-staging/<提案id>/`，Verify pass 后同租约内原子 rename 进 `<工程>/film/`（模式 `projectworkspace/derived.go:51-65`），持久化基线走 ProjectHistory；⑥封堵：`workspace/workspace.go:513` `ensureTextWritablePath` 拒写 `<工程>/film/**`。
- 文件域：上列 6 处；`agent/internal/film/**` 归 A1 卡（B1 只引用不修改）
- 并行与资源：无真栈需求（单测 mock port）
- 验收标准：`go build ./... && go test ./internal/orchestration ./internal/orchestrationruntime ./internal/executionports ./internal/workspace -count=1` 退出码 0；新增测试：能力注册可 Resolve、staging→verify-pass→rename 挂载、verify-fail 不 rename、`workspace_write_file` 直写 film/ 被拒
- 停止条件：发现 MutationPort/写租约契约与 staticbalance_vsp 模板有未记载的前提（如租约粒度限制）→ 停止上报

## 卡 4：VIS-FILM-B2-FILM-VERIFIER（确定性校验器+DEPENDENCY-BOM 建档）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / ~1 周 / 与 B1 并行开发（接线合入待 B1 领取后协调 `chat/film_runtime.go` 互斥文件）
- 目标：①新脚本 `scripts/film_verify.mjs`：输入 staging 路径+采样配置，输出 `verification_report.json`+退出码；检查 C1-C5（设计件二 §2.1：双打乱序重画像素比对/console 报错/字体+网络封禁/文字裁切/数据面一致性）；playwright-core 多候选自发现机制从 `scripts/webui_rendered_dom_smoke.mjs:200-227` 泛化为共享 lib（`scripts/lib/playwright_discovery.mjs` 新文件，原脚本改 import——**该文件与 C1 卡互斥**）；channel 回退链移植（bundled→msedge→chrome，先例 `scripts/e2e_chat_chain_layer_b.py:28-58`）；②新文件 `agent/internal/executionverifiers/film.go`：子进程调 film_verify.mjs，消费 report（模板 `executionverifiers/staticbalance.go:44`，Acoustic=nil）；③新建 `coord/resources/DEPENDENCY-BOM.md`（首版内容=设计件三 §2.3 表）。
- 文件域：`scripts/film_verify.mjs`、`scripts/lib/playwright_discovery.mjs`（新）、`scripts/webui_rendered_dom_smoke.mjs`（仅 import 改造）、`agent/internal/executionverifiers/film.go`（新+测试）、`coord/resources/DEPENDENCY-BOM.md`（新）；`chat/film_runtime.go` 接线行与 B1 互斥（顺序：B1 先建文件，B2 后接线）
- 并行与资源：无真栈需求（fixture 镜头页本地自测：一个确定性页+一个故意违例页各一）
- 验收标准：`node scripts/film_verify.mjs <确定性fixture>` 退出码 0 且 report 全绿；`<违例fixture>`（含 Math.random 画面/网络请求/文字裁切）退出码非 0 且 report 逐类命中；`go test ./internal/executionverifiers -count=1` 退出码 0；webui_rendered_dom_smoke.mjs 改造后原断言组回归仍退出码 0
- 停止条件：playwright-core 全候选目录不可用且 channel 回退链全失败（本机无 Chromium 系浏览器）→ 记环境证据上报，不自行下载（CDN stall 教训）

## 卡 5：VIS-FILM-B4-E2E（能力端到端：chat 写镜头页→过校验→挂载+读图迭代）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / 2-3 天 / A1+B1+B2+P0 全部 done
- 目标：真实栈在位（内核+渲染可用）跑通全链：工程内发起 film capability→agent 生成 1-2 个测试镜头页（数据来自 A1 真实渲染的 energy_envelope）→FilmVerifier 全绿→rename 挂载 `<工程>/film/`→artifact 登记；**读图迭代回路**：agent 对自己生成的帧截图发 CompleteImageUnderstanding，据反馈修改页面再过校验，至少一轮。
- 文件域：零生产代码（缺陷修复除外，修复须另列 diff 归属）；run 工件落 `coord/runs/VIS-FILM-B4/`
- 并行与资源：真栈烟测腿（PC-RUNTIME-STACK 登记；内核+agent，Godot 可不在位）
- 验收标准：run 目录含 agent 原始日志、verification_report（全绿）、挂载前后 `<工程>/film/` 快照、读图请求/响应工件、迭代前后页面 diff；全程零 panic
- 停止条件：链路断在 A1/B1/B2 任一环（非本卡代码）→ 记断点证据退回对应卡

## 卡 6：VIS-FILM-C1-PLAYER（webui 播放器+沙箱三层）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / ~1 周 / 设计件一/三契约（可 fixture 页先行，与 B 线并行）
- 目标：①`main.tsx` 加 `?surface=film` 分支（先例 :7-12）；②新文件 `webui/src/film/FilmPlayerView.tsx`：film.json+镜头页+WAV 经 `artifactFileURL` 加载（`lib/api.ts:272`），`<audio>` 主时钟+rAF 帧驱动+相邻镜头 iframe 预载（设计件三 §1.2）；③iframe `sandbox="allow-scripts"` 无 allow-same-origin；镜头页生成模板注入 CSP meta（校验器 C3 已断言）；④`webui_rendered_dom_smoke.mjs` 加 film 断言组（iframe 存在/sandbox 属性/frame 调用计数/audio currentTime 推进）——**与 B2 卡互斥该文件**（B2 先做 lib 泛化，C1 再加断言组）。
- 文件域：`agent/webui/src/main.tsx`（分支）、`agent/webui/src/film/**`（新）、`scripts/webui_rendered_dom_smoke.mjs`（断言组，互斥条款见上）；不碰 App.tsx 主面（v1 入口=URL 直入+artifact 预览卡动作位，后者若需动 App.tsx 则拆最小 diff 单列）
- 并行与资源：DOM 烟测腿 1 次（run_webui_rendered_dom_smoke.ps1 隔离泊位）
- 验收标准：`cd agent/webui && npm run test && npm run build` 退出码 0（新组件 jsdom 测试：时钟驱动/镜头切换/沙箱属性）；`run_webui_rendered_dom_smoke.ps1` 含 film 组退出码 0；手测指引按 AGENTS §5 Godot 前端途径另行提供（用户在位时）
- 停止条件：若 vite proxy/同源面导致 iframe 加载被浏览器策略拦（实测发现）→ 记录原始错误上报，不自行改代理配置

## 卡 7：VIS-FILM-C3-EXPORT（MP4 导出+真栈烟测）

- 发卡：（主管填）/ 派发确认：（待主管确认）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1 / ~1 周 / film.json 契约（设计已锁，可与 C1 并行开发）；烟测腿需 C1 的 fixture 成片
- 目标：①新脚本 `scripts/film_export.mjs`：逐镜头 headless Chromium 逐帧 `frame(t)` 截图→ffmpeg h264+整曲 WAV mux→MP4；ffprobe 校验时长/帧数/音轨（设计件三 §2.1）；ffmpeg 探测=PATH 优先+WinGet 候选路径+版本断言；②新脚本 `scripts/run_film_export_smoke.ps1`：真实栈在位经 agent HTTP 面发起导出→等 MP4→ffprobe 断言→退出码 0；③`coord/resources/DEPENDENCY-BOM.md` 补 ffmpeg 条目（与 B2 互斥该文件，B2 先建）。
- 文件域：`scripts/film_export.mjs`（新）、`scripts/run_film_export_smoke.ps1`（新）、`coord/resources/DEPENDENCY-BOM.md`（追加行，互斥条款）
- 并行与资源：导出烟测腿（PC-RUNTIME-STACK 登记，与 L1/L2 烟测排队）
- 验收标准：本地 fixture 成片导出 MP4 成功（ffprobe 时长 ±1 帧/帧数=duration×fps/音轨在）；`run_film_export_smoke.ps1` 退出码 0；产物不入 git（exports/ 在 .gitignore 追加，该行变更归本卡）
- 停止条件：ffmpeg 探测全候选失败（WinGet 目录漂移）→ 按环境中断记录上报，不算功能失败

## 卡 8：VIS-FILM-D1-DEMO-FILM（demo 工程成片）+ D2（展演素材，12 月评估点后）

- 发卡：（主管填）/ 派发确认：（待主管确认；D2 另受 DEFENSE-SHOW-1 评估点裁定）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P1（D1）/ P2（D2，12 月上）/ 全部前卡 done
- 目标：D1：单 demo 工程 3-5 镜头成片——能量驱动切镜（intro/verse/chorus 各至少一镜头）、能力链全程产出、webui 全片播放无中断、MP4 导出成功；工件落 `coord/runs/VIS-FILM-D1/`。D2：按 demo-capture SOP 采集展演素材（最佳轮次实录+导出 MP4 兜底，live/录播明示边界）。
- 文件域：零生产代码；demo 工程状态与 run 工件
- 并行与资源：D1 真栈烟测腿 1 次；D2 录制在位（用户在场）
- 验收标准：D1=评估点"可判定 v1"硬判据全中（设计件四 §4）；D2=素材命名/证据链符合 demo-capture SOP
- 停止条件：镜头质量连续 3 轮迭代不达标 → 记录轮次证据，降级预案（v1 判定延期/展演移出）交主管

---

## 发卡顺序与池序建议（主管复核用）

- 依赖图：P0→B4；A1→B4；B1→B2 接线→B4；C1 契约→C3 烟测腿；全部→D1→D2。
- 立即可发（互斥文件域核对过）：P0、A1、B1、C1 四张并行；B2 待 B1 建文件后发（或同批发但卡面注明互斥顺序）；C3 可同批发（互斥仅 DEPENDENCY-BOM 追加行）。
- 互斥文件清单（先到先改，后到者 rebase 协调）：`chat/film_runtime.go`（B1 建/B2 接线）、`scripts/webui_rendered_dom_smoke.mjs`（B2 泛化 import/C1 加断言组）、`coord/resources/DEPENDENCY-BOM.md`（B2 建/C3 追加）。
