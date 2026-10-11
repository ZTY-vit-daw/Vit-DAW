# 设计⑤：实施拆卡（达到可直接发卡程度）

LOUDNESS-WATCH-DESIGN-1 设计件 6/6。三张实施卡（A 配置面 / B 触发与探针接线 / C 告警双面）按 PROTOCOL §5 模板给全规格；另列一张内核缺口候选卡（不发，内核线排序归主管）。卡间依赖 A→B→C；A 无硬依赖可立即发卡。

## 发卡侧注意（三卡共用）

- 发卡会话=LOUDNESS-WATCH-DESIGN-1 执行流（GLM-5.3-Flash PC，2026-10-11）；验收负责人=GLM 主管决策流；派发确认建议填"已确认（依据=用户 2026-10-11 晚窗功能定义+本设计件）"——最终确认权在主管。
- 文件域互斥核对基线（2026-10-11 晚）：doing 卡 GEN-API-ADAPTER-1（agent 适配包+配置面+scripts 冒测，Mac）/ GEN-CTX-DISCLOSE-1（CommandDispatcher.cpp 披露块，Mac）——A/B/C 与二者代码域不相交；**A 与 GEN-API-ADAPTER-1 的"配置面"、B 与其"scripts 冒测"存在措辞重叠，领取 A/B 前须重读该卡实际文件域再确认**（发卡侧预检记录，非阻塞）。
- B 依赖 M1-4（todo，池序 67）与 PULL-PROBE-TIER-EXT-1（todo，池序 52）合入；领取 B 前核对两者 done。

---

## 卡 A：LOUDNESS-WATCH-CONFIG-1（配置面）

```
# LOUDNESS-WATCH-CONFIG-1：响度监听配置面（工程级开关+预设+自定阈值）
- 发卡：GLM-5.3-Flash PC 执行流（LOUDNESS-WATCH-DESIGN-1 拆出）/ 2026-10-11
- 派发确认：已确认（用户 2026-10-11 晚窗功能定义"手动开关+阈值用户自定"+设计件 02）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / 无硬依赖（可立即开工）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
```

- **目标**：新建 `agent/internal/loudnesswatch` 包——①`loudness_watch_config.v0` schema+校验（设计② §2 全项：trigger_mode 三态、periodic interval≥60s、builtin id 经 `rlm.LookupDeliveryProfile` fail-closed、custom 借 `rlm.ValidateDeliveryProfile` 校验、realtime 接受存储+unavailable 语义）；②纯函数比对器 `Compare(rows, config) Verdict`（verdict schema `loudness_watch.v0`、pass/breach/unmeasurable 三态、LoudnessBand nil-min 语义、grade 强制 approximate、peak 维度只产 breach/未见 breach）；③`loudness.watch.configure`/`loudness.watch.status` 两工具的 catalog spec（`tools/catalog.go` 按 `:1067` render_profile_bind spec 形态）与 chat invoke 处理（`chat/server.go` handleInvoke 面）；④配置进 `projectAgentRuntimeState` 快照/恢复（`server.go:164/198/7245/7256` 同款扩展）。
- **文件域**：`agent/internal/loudnesswatch/`（新包）；`agent/internal/chat/server.go`（runtime state 字段+快照/恢复+invoke 分发）；`agent/internal/tools/catalog.go`（两 spec 行）；单测（包内+`chat` 侧往返）。
- **并行与资源**：不需要真栈；纯单测卡；与 M1-4/B/C 文件域不相交。
- **验收标准**：`cd agent && go build ./... && go test ./internal/loudnesswatch/... ./internal/chat/... -count=1` 0 FAIL；单测覆盖清单=①旧记录无字段=缺省关+零行为变化 ②配置写入→快照→恢复→读回全等（含 custom 指针阈值）③坏 schema/坏枚举=关+dropped 计数 ④builtin 未知 id fail-closed ⑤custom band 序/跨窗序/TP>0 违规逐项拒绝 ⑥realtime 存储往返+unavailable 原因 ⑦Compare：超上限 breach/低于有下限带 breach/nil-min 带不查下限/unmeasurable（行缺失、ST/M 维度恒 unmeasurable）⑧peak 必要条件语义（sample_peak>TP 上限⇒breach；≤⇒"未见 breach"非 pass）⑨verdict 文案带"近似口径"限定；`gofmt` 净。
- **停止条件**：`projectAgentRuntimeState`/invoke 面与锚点（server.go:164/198/7245/7256）不符 → 上交。

---

## 卡 B：LOUDNESS-WATCH-TRIGGER-1（触发与探针接线）

```
# LOUDNESS-WATCH-TRIGGER-1：响度监听触发接线（处理后三钩子+账本计量）
- 发卡：GLM-5.3-Flash PC 执行流（LOUDNESS-WATCH-DESIGN-1 拆出）/ 2026-10-11
- 派发确认：待主管确认（依赖 M1-4、PULL-PROBE-TIER-EXT-1 合入后生效）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P2 / 1-1.5 天 / 依赖卡 A 合入 + M1-4 合入（池序 67）+ PULL-PROBE-TIER-EXT-1 合入（池序 52）
- 模型分级：L1 / GLM 执行会话（PC，真栈烟测）
```

- **目标**：设计① §2 三钩子接线——①T1：`chat/free_state_conclusion_bridge.go` `recordFreeStateSettlement` 成功持久化后（`:75` 后）触发 watch 检查（后台 goroutine，失败只 WARN，dedupe 键=project_uuid+project_revision+verdict_kind）；②T2：`harness/requestMixObservationFeaturesBackground`（`harness.go:4279`）特征快照新鲜 loudness_summary 行的搭车比对（零新增 bake）；③T3：`LatestProjectChange` 更新处（`chat/free_state_reasoning_loop.go:83/362-363`）触发检查（带即时性诚实边界）；④周期模式（默认关）：复用后台探针节奏，interval 来自配置；⑤单次触发 bake 上限 ≤4 clip 超限 unmeasurable_reason=scope_capped；⑥M1-4 D1 pack 披露 watch 状态（依赖其只读面）；⑦冒测：按 `scripts/fs_largeproject_smoke.ps1` 的 `-M1FiveThreeView` 门控腿模式（`scripts/fs_largeproject_smoke.ps1:80,126,135-136` 互斥守卫+门控 param 先例）新增 `-LoudnessWatch` 腿：settle→verdict 落盘断言+dedupe 断言+配置关=零行为断言。
- **文件域**：`agent/internal/chat/free_state_conclusion_bridge.go`；`agent/internal/chat/free_state_reasoning_loop.go`（钩子点，不碰 overlay 语义）；`agent/internal/harness/harness.go`（后台搭车挂点）；`agent/internal/loudnesswatch/`（接线层）；`scripts/fs_largeproject_smoke.ps1`（新增门控腿）；单测。
- **并行与资源**：真栈烟测按 `coord/resources/PC-RUNTIME-STACK.md` 占用流程；与卡 A/C 串行（A 先 C 后）；与 GEN-API-ADAPTER-1 的 scripts 域重读核对（发卡侧注意）。
- **验收标准**：①单测：三钩子命中/未命中、dedupe 同版本单发、配置关静默零成本、陈旧行补 bake 上限、watch 失败不回滚 settle（桥 WARN 语义保持）、verdict 后台化（settle 回执先于 verdict）；②`go test ./... -count=1` 0 FAIL+gofmt 净；③真栈烟测 `-LoudnessWatch` 腿 exit 0（工件新目录，回执含 run ID/HEAD/`d1_smoke_report.json`）；④E1 实验（06 性能件 §3）顺带产出：3 样本 settle 触发成本记录+复用占比，结论回填本卡回执。
- **停止条件**：`recordFreeStateSettlement` 六漏斗形态/`LatestProjectChange` 装配点与锚点不符；M1-4 pack 面与其卡面不符 → 上交。

---

## 卡 C：LOUDNESS-WATCH-ALERT-1（告警双面）

```
# LOUDNESS-WATCH-ALERT-1：响度监听告警双面（agent 证据面+用户呈现面）
- 发卡：GLM-5.3-Flash PC 执行流（LOUDNESS-WATCH-DESIGN-1 拆出）/ 2026-10-11
- 派发确认：待主管确认（依赖卡 B 合入后生效）
- 验收负责人：GLM 主管决策流
- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / 依赖卡 B 合入
- 模型分级：L1 / GLM 或 flash 执行会话（PC，真栈烟测+webui 测试）
```

- **目标**：设计③四披露位+双消费者呈现——①typed event：`loudness_watch_verdict` 进工具/settle 回执 `typed_events`（`harness.go:3537/3838` 模式）；②catalog 条目 `project.risks.loudness_watch`（derived/cheap，`mixboard/catalog.go:265-266` 模式）+摘录位（`:204-217` capRows 形态），与 status 工具同源存储；③mix_report 三视图增"响度 watch"段（`chat/mix_report_three_view.go` M1-5 形态：最近 verdict 行+breach 计数+无 verdict 诚实缺省"未运行"）；④chat 会话事件行（`chat/audition_events.go` 事件形态）→webui 渲染（webui 侧改 `observePresentationLayered` 面+`src/types.ts` typed_events 类型，vitest 断言）；⑤`loudness.watch.status` 全量披露（历史 verdict+unmeasurable）。
- **文件域**：`agent/internal/harness/harness.go`（typed event 载荷）；`agent/internal/mixboard/catalog.go`；`agent/internal/chat/mix_report_three_view.go`+`mix_report_three_view_test.go`；`agent/internal/chat/audition_events.go`；`agent/internal/loudnesswatch/`（披露层）；`agent/webui/src/`（observe 呈现+类型+测试）。
- **并行与资源**：真栈烟测按 PC-RUNTIME-STACK 占用；与 B 串行。
- **验收标准**：①单测+vitest：catalog 条目只读性、三视图行有/无 verdict 双分支、typed event 形态、webui 渲染行字段完整、文案带 grade 限定词、unmeasurable 不进 chat 事件；②`go test ./...` 0 FAIL+`cd agent/webui && npm run test` 0 FAIL+`npm run build` 过；③真栈烟测：B 卡 `-LoudnessWatch` 腿扩展断言"verdict 行 webui 可见"（或同脚本新腿）exit 0；④**渲染面端测边界显式声明**（AGENTS §5）：E2E-WEBUI-1/旅程烟测层级现状+本卡覆盖边界，交决策侧裁定；⑤E2 实验（06 性能件 §3）如随本卡跑则回执记录。
- **停止条件**：三视图/typed_events/audition_events 锚点形态不符 → 上交；渲染面覆盖边界被裁定不足 → 补测卡或降范围重交。

---

## 内核缺口候选卡（不发卡，登记给主管）

```
# KERNEL-LOUDNESS-BS1770-1（候选）：内核响度口径补齐（K 加权门控 integrated+ST/M 窗+true peak）
- 依据：设计④ §3 缺口 1-4+§4 规格（文件域 L3AcousticAnalyzer.cpp/AudioFeatureTypes.h、
  probeaudio/suite.go:53,106-185 合成信号 fixture、-20 dBFS 正弦 ≤0.1 LU 断言、TP inter-sample 检出）
- 排序归主管（内核线；与 D/E 母带实现后置裁定 2026-10-04(2) 的关系由主管裁量）
- 完成后 watch 侧零改动生效（卡 C 预留 grade=bs1770 分支）
```

## 卡间依赖与排序图

```
PULL-PROBE-TIER-EXT-1(池52) ─┐
MIXLAYER-M1-4(池67) ─────────┤
                             ▼
LOUDNESS-WATCH-CONFIG-1（A，立即可发）──→ LOUDNESS-WATCH-TRIGGER-1（B）──→ LOUDNESS-WATCH-ALERT-1（C）
                                                            │
                        KERNEL-LOUDNESS-BS1770-1（候选，独立）─┘ grade 升级自动生效
```

- 用户手测义务：无新增（2026-10-09 裁定：共享协作收口不加用户手测节点；真栈验证全部走冒测脚本腿）。
