# 共享卡池统一索引（POOL-INDEX）

- 维护：GLM 主管决策流（主线）/ 2026-11-06 建立（用户指令「按三线执行计划整理任务池」）
- **权威规则**：领取凭证=**卡文件名（卡 ID）**；数字池序仅排序辅助。编号空间（2026-11-07 早窗主管裁定修订）：主线（混音/自由态/A-F/仪器）**258–275 封块**（269–274 为 11-07 凌晨连续发卡已成事实追认，275=B3-ABLATION 本次启用；原「267–275 留空」缓冲规则作废——消歧目的已被 11-07 凌晨发卡突破，回改成本>收益，[映射表](../reports/2026-11-05-nov-three-line-execution-plan.md)在案可查旧号）；三线（乐局/编曲/生成）**276–291**（原 260–275，2026-11-06 撞号协调改号）；**292 起两线共用递增序列，发卡时以本索引登记先后为准**。
- 状态口径：todo=可领取（含挂起待触发）；doing=执行中（worktree+分支登记）；done=执行侧自验完成待主管验收；archive=裁定退出执行的历史记录（不再领取）。

## 一、主线（混音/自由态/A-F/仪器）——GLM 主管决策流管辖

### A-F 进度计量（终局裁定口径：A=已证可拆 / H=harness 形态已测 / F=固定层推测）

**A=1（B4，2026-11-06 [264 裁定](../rulings/2026-11-06-B4-ABLATION-1-pass.md)）｜ C4=H（263 维持）｜ B1-B3/C1-C3/D1=F（B3 已拆未过维持 F——275 判定型负结果，263 组 3 同型）｜ D2/D3=判据薄层已点亮（265/266，族本体未建）｜ C5/D4/E/F=not_built**

### 活跃/待领

| 池序 | 卡 ID | 状态 |
|---|---|---|
| 262 | MIXTICK-CONFIRM-CONTRACT-1 | done 已验收 pass 收口（G6 全链闭合） |
| 263 | C4-ABLATION-1 | done 已验收（维持 H，词表=入口识别薄层） |
| 264 | B4-ABLATION-1 | done 已验收（**A 态确认**） |
| 265 | D2-STEREO-THIN-1 | done 已验收 pass 收口（exit 0+1-mutation 主链首通） |
| 266 | D3-REF-ALIGN-THIN-1 | done 已验收 pass 终验收口（exit 0+D3 点亮） |
| 267 | MSGLOOP-FEEDBACK-LOSS-1 | **done 取证型 pass**（前提推翻+重定性；累积弹回史挂 262 验证腿读数） |
| 268 | D1GATE-SCRIPT-HYGIENE-1 | done 已验收 pass（主管亲跑复验） |
| 269 | EXPADMIT-EVIDENCE-RULE-1 | done 已验收 pass 收口（契约链整程真栈走通） |
| 270 | SMOKE-ENC-HYGIENE-2 | done 已验收 pass（主管亲跑） |
| 271 | CLOSURE-ROUTE-ADAPT-1 | blocked（11-17预算8轮实证，目标工具未达/9调用超8上限；终验未过，停止重跑，352不解锁） |
| 272 | OBSFACE-SELECTIVE-MISS-1 | done 取证型 pass（前提改写=agent 侧接线错位非 kernel；转卡 274） |
| 273 | ARRLOOP-AUTHBRIDGE-1 | doing（11-07 早窗重发；零栈段在派，验证腿候栈第 3 位） |
| 274 | D1D3-DIGEST-WIRING-1 | done 已验收 pass 合入（相邻两处转卡 292/293） |
| 275 | B3-ABLATION-1 | done 已验收 pass 收口（判定型负结果：B3 维持 F 不判 A；263 组 3 同型，B4=A 族内对照素材） |
| 292 | MIXBOARD-TRACKKEY-1 | done 已验收 pass 合入（四处同型候选记录→PD） |
| 293 | WE-MISMATCH-FORENSICS-1 | done 已验收 pass（取证型：设计内披露+物化链无罪+前提证伪；转卡 294；加固挂 PD-6） |
| 294 | FIXTURE-WE-HYGIENE-1 | done 已验收 pass（三处剔除+离线四情景验证；三休眠行记录接受） |
| 295 | CAPABILITYPACK-KEYADVERT-1 | done 已验收 pass 合入（A 案宣传面修正） |
| 296 | D1D3-REGRESS-STACK-1 | done 终验收口（实质 pass：274/292 substance 闭合三轮三型亲证；D1-settle/D3 活栈挂账搭便车；LLM 超时=环境模式记录） |
| 297 | KERNEL-OFFRENDER-FORENSICS-1 | done 已验收 pass（候选②fail-silent 守卫唯一不可排除；转卡 301 复现；PD-7=by-design 钉死） |
| 300 | PLUGIN-CONTROL-EVAL-RUNNER-1 | done 已验收 pass（**P 基线 5/6 落地**；T4=纪律非能力；typed 承重声明→305 缝卡；守卫误拒扩 302 域+ms 反演转卡 304） |
| 304 | DELAY-APPLY-MS-FIX-1 | done 已验收 pass 合入（分段反演+物理回读诚实降级） |
| 305 | PLUGIN-CONTROL-EVAL-SEAM-1 | done 已验收 pass 合入（四缝+A 臂三面亲证；S2 残留=A 口径声明项） |
| 307 | PLUGIN-CONTROL-EVAL-S1A-1 | done 已验收 pass（A=P=5/6 同构；两装置发现→S5/S2b 开卡 308；T4 疑似规则缺陷批次前修） |
| 308 | PLUGIN-CONTROL-EVAL-SEAM2-1 | done 已验收 pass 合入（S5 三出口裁族+S2b 渲染剥离+三态字节不变；control_shell=键名残留接受；310 解锁） |
| 310 | PLUGIN-CONTROL-EVAL-APURE-1 | done 终验 pass 收口（**A-pure 3/6 vs A 5/6 vs P 5/6：写入类=typed 承重，裸面 T1/T3/T4 全败/T2 裸写唯一过/读观察等价**；B 段 go 候手测后；开放录音棚=适配层须 typed 等价面） |
| 311 | M2-HANDTEST-PREP-1 | done 终验 pass 合入（exit 0 手测放行：工程布位亲证+VIT_EDU_LEVELS_DIR/YuE2 前置入清单 §〇；282 离线兑现；GUI 最后一米=用户本体） |
| 312 | PLUGIN-CONTROL-EVAL-S1B-1 | done 终验 pass 合入（B 零检索执行——主动性>可见性） |
| 317 | PLUGIN-CONTROL-EVAL-BR1-1 | done 终验 pass 合入（态三：提示在场仍零发起；**深夜修订：9/9 失败轮=守卫阻断非编码错误**，[结论档](../rulings/2026-11-08-plugin-control-eval-line-conclusion.md) ⓪ 节） |
| 318 | PLUGIN-CONTROL-EVAL-BAREFREE-1 | done 终验 pass 合入（**T3 3/3=真裸写能力证实**；T4=真实编码边界；T1 第三层守卫→319 补完） |
| 319 | PLUGIN-CONTROL-EVAL-BAREFREE-T1-1 | done 终验 pass 合入（**T1 3/3——barefree 线四案全清**：守卫许可下 T1/T2/T3 全 3/3，唯 T4=真实编码边界） |
| 320 | PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 | done 终验 pass 收口（六批纪律链+三轮诚实对账：r1 提案链被断连/r2 LLM 超时/r3 能力边界拒调[频段测量真实]；产品面零 rogue 裸写；GUI 终审物交付；「实际调好 EQ」演示候 322 裁定） |
| 321 | RACK-PARAM-PERSIST-FIX-1 | done 终验 pass 合入（probe 3/3 存活亲证 0.7→reopen 0.7；320 解锁） |
| 322 | PROQ3-CERTIFY-AND-T7-1 | done 终验 pass 合入（**r1/r3 裸写逐项吻合+S5 零 typed 铁证+persisted×3**——Pro-Q 3 认证落库+derive 带锚修复；早期首碰壁目标闭环；GUI 终审物交付） |
| 323 | EQ-RECEIPT-VENDOR-EXPAND-1 | done 终验 pass 合入（vendor-scoped 最保守+双门干跑亲证——322 解锁） |
| 324 | BRIDGE-POC-1 | **todo 候用户确认**（控制面改革方向去风险：display probe 反演双锚验证 T4/r3；零栈优先零 LLM；改革主线排展演后——2026-11-09 用户定向） |
| 325 | FIX-CAPROUTE-RETRIEVAL-1 | done 已验收 pass 合入 a36df77d（**负结论**：sem 族三披露形态全可达零缺口+317 line18 实调亲证；防回归锚测试随卡；PENDING①条目关闭） |
| 326 | FIX-RESTARTAGENT-ENV-1 | done 已验收 pass 合入（取证翻案型：重启链本就全量透传零丢失；真实丢失面=复用路径冻结+会话缺 env（有意设计）；修复收敛=契约注释+断言工具；321 run1 归因消解；真栈轻腿 exit 0+§10 哈希亲证） |
| 327 | FIX-GOV-INTERACTION-TTL-1 | done 已验收 pass 合入 c8bb576e（勘察翻案：无 TTL，丢失=落错工作区+restore 替换+罐头伪造 completed；保活+两态回执活体 A/B 证成；park 腿降便车腿；330 解锁） |
| 328 | FIX-FREEFLOAT-EQ-ANCHOR-1 | done 已验收 pass 合入 a670c67a（TBTECH 落盘 static_eq 50+类缺口登记+322 计数随卡修复；主管亲测三 PASS） |
| 313 | LEGAME-UX-DESIGN-1 | done 终验 pass 合入（设计 v1 落档：L1 四件+三段式 onboarding+HUD+七卡清单；三裁定点[L1-d 案别/onboarding/HUD]候用户） |
| 314 | LEGAME-DIR-1 | done 终验 pass 合入（runA 零 env 产品级装载实证）
| 315 | LEGAME-AUTOINTRO-1 | done 终验 pass 合入（runB 白话开场+「接着上局 1/7」恢复播报亲证）
| 316 | LEGAME-FRESH-1 | done 终验 pass 合入（relay 幂等+用户工程哈希零触碰+runC 新工程语义） |
| 306 | GEN-VOCALSEP-1 | done 已验收 pass（三 vocals stem 候用户试听终审；集成腿候后续卡） |
| 301 | KERNEL-OFFRENDER-REPRO-1 | done 已验收 pass（**候选②排除+定性 WaveShell 插件兼容**；M3 证活渲染管线；转卡 309） |
| 298 | VSTI-PARAM-PROBE-1 | done 已验收 pass（两件内核面控制实证零 mismatch=准入链②证据到手；agent 面守卫误拦转卡 302） |
| 303 | GEN-CHAT-ROUTING-1 | done 已验收 pass 合入（token 边界+生成域扩表+b1/b2/b3 同面顺带修；278 末腿放行） |
| 302 | VSTI-WRITEGUARD-1 | done 已验收 pass 合入（单点族豁免+未知族 fail-closed 维持） |
| 299 | PLUGIN-CONTROL-EVAL-PREP-1 | done 已验收 pass（PREP 收口：模型=deepseek-flash 双证+A/B 四锚点污染定位；RUNNER 不含缝可先行，缝+模型矩阵候用户裁定） |
| 276 | EDU-SYMBOL-CORE-1 | done 已验收 pass 整卡收口（黄金三条真栈命中） |
| 277 | ARR-SRCLOAD-1 | done 已验收 pass 收口（装载链四环节实证；渲染静音=域外内核缺陷转卡 297；**注册链③解锁**） |
| 279 | EDU-LEVEL-CONTENT-1 | done 已验收 pass 整卡（首个乐局卡定稿+319 项复算） |
| 280 | EDU-LEVEL-RUNTIME-A | done 已验收 pass（runtime_slots+语义定案两裁定） |
| 281 | EDU-LEVEL-RUNTIME-B | done 已验收 pass 收口（smokerun3 双 0 全相绿；整链合入 a7c23dfe；**乐局线 276→279→280→281 全贯通**） |
| 239 | M3-LLM-TIMEOUT-ADAPT-1 | done 已验收 pass |

### 未成卡 backlog

C5 设计（E 前置=主总线处理）→ E 批次 → F rubric；E2E-WEBUI-1/JOURNEY-1 渲染面/旅程烟测债；154/155 卫生项；AF 体检下轮抽查（261 其余三腿）；**lease 门 owner 参数化**（408 runtime 发现：ps1 lease 门硬编码 owner 串与资源记录格式耦合，曾致预检假拒+现场格式修正绕行，窄修卡候 409 后）。

## 二、三线（乐局/编曲/生成）——按 [11 月三线执行排序](../reports/2026-11-05-nov-three-line-execution-plan.md)（领卡顺序权威）

- **首波收口**：276/277/278/279/280/281 全终验合入（**278 整卡 pass：chat 唱歌 exit 0=注册链③ 收口**，生成线对话式唱歌全链打通）；注册链④（282 实现腿轻验 pass 2026-11-08：引导段落 ruleset shared 段+四面注册+域外 4 行追认；真栈轻腿候栈）；
- **乐局线后置（2026-11-09 用户裁定）**：手测里程碑=AI-first 启动面（[裁定档](../decisions/2026-11-09-handtest-milestone-launcher-ruling.md)）；乐局线挂起候编曲/生成落地——283 注册链⑤暂缓、EDU 待发卡挂起；**编曲线优先级上调**（273 验证腿+ARR 待发卡）；生成线次之。
- 注册链串行：276→277→278→282→283；其余依赖见计划文档卡序表；
- 外部依赖（用户动作）：~~裁定 15 问询通用化~~（已裁 2026-11-08：方案 a 通用段）/Audiveris+music21 部署（286 前）/Suno key（288 前，可选）；
- 真栈窗口与主线按日协调（§9 一次一个 owner，登记 coord/resources/PC-RUNTIME-STACK.md）。

## 三、挂起待触发（留 todo，不可立即领）

| 卡 | 触发条件 |
|---|---|
| ARTIST-AUDITION-1（62） | 生成线首批落地+编曲线前置就绪（用户 2026-10-11 裁定后置） |
| VIS-FILM A1/B1/B2/C1（82-84/86） | Mac 生成线（ViTape）排期，链内并行 |
| VIS-FILM B4（85）/C3（87）/D1（88） | 链内前置（B1+B2 / C1 / B4+C3）合入后；D1 另挂 12 月评估点 |
| GENSHELL-AUDITION/GUI（98/100） | GENSHELL-AGENT-1（Mac 线） |
| FE-MOJIBAKE-CLEANUP-1（108） | Mac 线 P3 卫生（10 分钟随时可做） |
| VMT-1 | 就业预习线优先，不占主线窗口 |
| UX-AB-AUDITION-RESIDUAL-1（181） | P3 待命，触发后按 P1 处理 |

## 四、归档（退出执行的历史裁定记录）

- 2026-09-26-DEFENSE-SHOW-1：展演收敛到教学轨迹（录播），节目单整体退出（2026-11-05 用户裁定），本卡转历史裁定记录——2026-11-06 自 todo 移入 archive。

## 维护规则

1. 各线发卡后在本索引登记行；卡状态流转（领取/完成/验收）由执行侧回执+主管验收更新；
2. 新卡编号遵守编号空间裁定（主线 258–275 封块已满/三线 276–291 已分配；**292 起两线共用递增**，发卡登记本索引认领池序）；
3. 挂起卡触发条件达成时由管辖线主管移入各自活跃段。
| 309 | WAVESHELL-OFFRENDER-FIX-1 | done 已验收 pass 合入 317fce56（**M1=RT-SILENT 判别**：GR 四 harness 全静音→归属装载/license/壳层转 329；S1 运行时确证非 GR 成因转 331；修法 B warn_only+形态 3 落地；C 不转正） |
| 329 | WAVESHELL-LOADING-FORENSICS-1 | doing（**取证腿验收 pass 2026-11-09**：三代归并+闭源壳 license 状态机+本机授权链物证——H1=授权环境主嫌疑上报用户；判别腿 D1-D4 候栈候用户定向） |
| 330 | FIX-PLAN-STALE-STATUS-1 | done 已验收 pass 合入 14a105a3（plan 面两态回执+chat 投影第二伪造面一并修；红转绿在卷；334 清扫卡转出） |
| 331 | VST3-SIGNAL-HYGIENE-1 | todo P3（309 S1 卫生：setup/per-call 信令一致化——**不占展演前窗口**） |
| 332 | AI-LAUNCHER-DESIGN-1 | done 终验收口（2026-11-09主管与用户终审pass，实施链已另卡落地；本行2026-11-17消除旧状态） |
| 333 | LEGAME-GENSCOPE-1 | done 已验收 pass 合入 82f42846（手测 v2 旅程面收口：只走旅程 A，零生成前置；边界声明含 AI-first 衔接） |
| 334 | FIX-GOALSTATUS-FORGERY-SWEEP-1 | todo 可领 P3（goal_status 伪造面全树普查+同型修——327/330 缺陷族收尾；随手清型） |
| 340 | LAUNCHER-GATE-FIX-1 | done 已验收 pass 合入（**门修复+腿1/2 双 exit 0——手测 v2 启动面就绪**；launcher 路由首次真栈走通+338 桥接段证成） |
| 341 | LAUNCHER-FE-LAYOUT-FIX-1 | **撤销归档**（2026-11-14 晚窗主管裁定：348 落地后修缮对象转 legacy 次面；后继=用户手测 v3 React 路径，排 347 后；[依据](../rulings/2026-11-14-UI-BRIDGE-1-pass.md)） |
| 342 | RECENTS-HYGIENE-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-RECENTS-HYGIENE-1-pass.md)：10 条 fixture 清出零残留+Settings.xml 零漂移亲证；泄漏锚点=VitHeadlessService 三处无条件写；隔离双轨——b 案已入 AGENTS §10 装置纪律 / a 案=351 新卡；recents 重建=用户裁 b 留空） |
| 335 | LAUNCHER-CHAT-FE-1 | done 已验收 pass（零栈段——探针 38/38 亲测+红线零触碰；渲染观感候 339+用户 GUI） |
| 336 | LAUNCHER-NAV-FE-1 | done 已验收 pass（探针 71/71 亲测+红线零触碰+前端 main 已合并——用户手测入口即刻可见） |
| 337 | LAUNCHER-CHAT-BE-1 | done 已验收 pass 合入（软动词分级+三分支+authority 三红线；契约增量对齐进 336/339） |
| 338 | LAUNCHER-BRIDGE-BE-1 | done 已验收 pass 合入（context_flow 三态+volume_switch+桥接两分支 315 承接零重写；跨工程路由登记候选） |
| 339 | LAUNCHER-SMOKE-1 | done 已验收 pass（装置+腿3 降级 exit 0+greeting 真栈全契约；**腿1/2 结构性门关取证→340 修复卡**：337 uuid 判据真栈不可达双层证据） |

### 主窗口 v5.1 UI 实施（2026-11-14 发卡，343–349）

| 池序 | 卡 ID | 状态 |
|---|---|---|
| 343 | UI-HOST-1 | done 已验收 pass 合入（**v5.1 链头落地**：autoload 承载结构成立——同 HWND/URL 跨场景存活真栈 exit 0 + legacy 回退；[裁定](../rulings/2026-11-14-UI-HOST-1-pass.md)；前端 caaef19/主仓精选合入；R0 §4 ①②③取证达成，DPI/多屏留边界） |
| 344 | UI-FOUNDATION-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-FOUNDATION-1-pass.md)：主管亲跑 test:render+build 复验 exit 0 + 28 文件全在授权域 + 设计哈希回指；**345 解锁，test:render=后续 UI 卡渲染面验收底座**） |
| 345 | UI-LAUNCHER-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-LAUNCHER-1-pass.md)：主管亲跑 498 测+双渲染烟测 PASS；App.tsx 7 行分发最小改动；open/new 留 348——**346 解锁，348 前置全齐**） |
| 346 | UI-PROJECT-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-PROJECT-1-pass.md)：529 测+三渲染面主管亲跑；选择≠应用红线单测在卷；宏快照/A-B agent 协议缺口登记候另卡） |
| 347 | UI-LOADING-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-LOADING-1-pass.md)：主管亲跑 560 测+四渲染面；无假倒计时红线全核——**v5.1 链全通：343→349 七卡单晚落地，React 启动面端到端可用（flag 门控），下一步=用户手测 v3**；live 装载分步事件候手测后裁定） |
| 348 | UI-BRIDGE-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-BRIDGE-1-pass.md)：真栈 exit 0 桥链全通——React→directive→治理三键→open→dock 同 HWND 承接→回执 React 真收；前端 380166e；零摩擦 finding 转 352；341 撤销；godot-cpp 转 353） |
| 349 | UI-OS-1 | done 已验收 pass 合入（[裁定](../rulings/2026-11-14-UI-OS-1-pass.md)：真栈 71 断言 exit 0——关闭≠收拢/前台门控通知/同 HWND 召回；WinRT toast 按钮与通知触发链边界登记留 348/另卡；前端 128477b 已推） |
| 350 | R0-CHAT-WINDOW-RECON-1 | done 已验收 pass 收口（取证型：设计侧预勘察收口为交付物，[裁定](../rulings/2026-11-14-R0-CHAT-WINDOW-RECON-1-pass.md)；真栈未知项移交 343） |
| 351 | REC-ISOLATION-CONFIGDIR-1 | todo 可领 P3（342 裁定 a 案：env VIT_DAW_USER_CONFIG_DIR 装置配置目录隔离，fail-open；真栈腿候 PC 栈空闲；**不占展演前主线**） |
| 352 | LAUNCHER-ZEROFRICTION-1 | doing（**取证腿 pass+方案裁定采 A1**（launcher 卷门控不物化）——根因链=聊天落盘对话节点→物化 Unsaved 草稿→path hint→三键，主管亲证锚点；实施腿仍候271终验（11-17未过）后另领） |
| 353 | ENV-GODOTCPP-RESTORE-1 | done 已验收 pass 合入（双位置全空取证+根因高置信=拆除穿透 junction（归因 open，mtime 证明早于主管 force remove）+4.2.2 恢复+scons 双变体 exit 0 亲证；§3 rmdir 实测形式补遗；B 位不恢复裁定） |

- 推荐起手：**343 ∥ 344**（Godot 宿主 / WebUI 基座，文件域不相交）；345 在 344 合入后；346 在 345 后；347 在 346 后；348 等 343+345；349 等 343。
- **341 排序约束（主管 2026-11-14 晚窗）**：341（LAUNCHER-FE-LAYOUT-FIX-1）与 343 文件域重叠（前端仓 `app/startup/start_page*`），**排在 343 合入之后**才可领取；v5.1 链落地后 341 的价值面收缩（启动面主位将转 React launcher），届时由主管重新裁定 341 范围（可能降级为 legacy fallback 最小修缮或撤销）。
- 发卡：主设计会话（lead）/ 2026-11-14；派发确认=已确认（用户「把新更新的 UI 设计任务放进任务池」）；主管可调整排序与编号。
- R0 勘察卡已由主管确认收口（池序 350，done 归档；报告 `coord/runs/R0-CHAT-WINDOW-RECON-1/REPORT.md`）。
| 354 | UI-ENTRY-1 | done 已验收 pass 合入（P0（**手测 v3 返工主卡**：单窗入口——启动态 React 启动台独占+Godot 主窗隐藏+进工程展开+回退；surface 同窗不重载切换；[裁定](../rulings/2026-11-14-UI-ENTRY-1-pass.md)——**手测 v4 放行**，方案 B 当日闭环） |
| 355 | UI-ENTRY-2 | done 已验收 pass 合入（[裁定](../rulings/2026-11-15-UI-ENTRY-2-pass.md)：关窗三进程退出+失败链零 Godot 回退+splash 压制；降级采纳=关工程仅退出（内核 close 卡候选）；前端 83390f5——**手测 v5 放行**） |（手测 v4 反馈：关窗即退出+无 Godot 回退+工程态往返单窗+splash 压制；354 语义修正，start_page 代码保留仅退场门面） |

### VitShell 宿主线（2026-11-15 设计定稿发卡，356–360）

| 池序 | 卡 ID | 状态 |
|---|---|---|
| 356 | SHELL-1 | done 已验收 pass 合入（**主管亲跑 PASS**：VitShell→agent→React 真数据→关窗全树退→Vit 系进程零残留；extension 零 diff；「API 连不上」=agent cwd 探测链已固化；前端 b578525——**SHELL-2 解锁**）（宿主骨架：WebView2 主窗+agent 进程树+装载态+退出清理；零 Godot 启动；含「API 连不上」真实环境验收；**过夜可跑**） |
| 357 | SHELL-2 | done 已验收 pass 合入（**主管亲跑全旅程 PASS**：抽屉按钮→零摩擦首开→直启 DAW→同窗切换→页内三键二次开→全树净退+S10 还原；两真缺陷修复采纳；执行侧直推 main 偏差记录——**SHELL-3 解锁**）（启动态桥：抽屉手动按钮+页内治理确认（跟随发起方）+Godot 直启 DAW+关 DAW 回主窗；排 SHELL-1 合入后） |
| 358 | SHELL-3 | done 已验收 pass 合入（**主腿 3/3 轮 PASS**+失败注入三腿+legacy 腿；手测 v6 放行包在卷——**VitShell 架构全线贯通，候用户手测 v6**；裸双击需 --dev-root 开后续卡候选）（全链真栈烟测+手测 v6 放行；排 SHELL-2 后） |
| 359 | UI-VISUAL-FIDELITY-1 | done 技术面 pass 合入（[终验=用户目检 v6]；裁切双侧根因：webui 高度链已修+宿主 DPI 移交 SHELL-1；12 项修复+四分辨率烟测段；8 缺口上交清单在卷）（launcher/工程面视觉保真返工对照定稿稿+窗口裁切多分辨率修复；与 SHELL-1 并行，SHELL-2 前合入） |
| 360 | SHELL-4 | done 已验收 pass 合入（主管亲核 diff 栅栏+run 工件 3/3 轮+复跑 go/vitest 573+栈释放亲证；主仓精选 1600f9dc/4c02a287→a6c99f71、前端 2ede560→d05043e；**手测 v6「新建/列表/问候」复测腿候用户**；遗留=新建名不进内核身份→**363 内核域卡**）（[用户裁定：内核随窗就绪](../decisions/2026-11-15-kernel-ready-at-shell-start-ruling.md)——启动台内核在线+新建/列表/对话真数据+内核生命周期归属主窗；执行细节见卡面回执） |
| 361 | UI-SPRITE-MOUNT-1 | done 已验收 pass 合入（主管复跑三门+负对照亲证；实现 59842aa8→main 1ea32119；真实栈图标可见性候 SHELL-4/用户目检）（**实现 commit 59842aa8 @ port/ui-sprite-1 已推**：main.tsx 挂载 DesignSprite+导入 sprite.css；render-smoke 图标计数/非空断言×6 面+真实入口 APPENTRY 段；三门 exit 0（vitest 570/570）+负对照拦截实证；工件 coord/runs/UI-SPRITE-MOUNT-1/；真实栈图标可见性候用户目检；**DesignSprite 全局未挂载→全应用 sprite 图标渲染为空**（发送键黑方块/回复中无指示）已修；双图标体系清单在卷卡面） |
| 362 | UI-MSG-SHADOWBOX-1 | done 已验收 pass 合入（主管复跑三门+两面 computed 亲核+负对照采纳；实现 65c659bf→main 3a3d2973/合并 52641eb1；真实栈候用户目检）（[用户裁定：用户消息阴影框标识](../decisions/2026-11-15-user-message-shadowbox-ruling.md)——两面 1px 灰边+柔影+右对齐，assistant 无框；SPEC-V5 §8.1/§11 修订在卷） |
| 363 | KERNEL-NEWPROJ-NAME-1 | todo（SHELL-4 遗留边界承接：内核 new_project 命令面接受工程名——新建《X》名实相符+空白草稿治理豁免裁定；内核 C++ 域；**候排期**） |
| 364 | SHELL-5 | done 已验收 pass 合入（主管亲核零绑定/按钮直建/意图自动执行 3/3 轮+三门复跑；实现三提交→main 10f97b12；attempt2 产品缺陷自修复链采纳；用户复测腿就绪）——细节见卡面回执 |
| 365 | UI-REC-TAPE-1 | done 已验收 pass 合入（主管复跑三门+**磁带造型目检亲证**（外壳+窗口+双带轮 20px 一眼可辨）+收起守卫采纳；实现 62fd380c→main b2eec1dd；氛围色真实名散布辨识度候用户目检）（磁带 cassette 头像 TapeBadge+§7.3/§7.3.1 v5.1.1 修订+§12 收起回归守卫，细节见卡面回执） |
| 366 | UI-REC-TAPECARD-1 | **终验 PASS（用户目检 2026-11-17 凌晨「磁带卡成立」）**——整卡闭环 |
| 367 | SHELL-6 | done 已验收 pass 合入（工程面意图执行+治理/确认卡全 surface 渲染；L3b 3/3 亲证+vitest 598；实现→main 4ce8862a；**不变式=任何确认必有可点交互面**；用户复测腿就绪） |
| 368 | UI-CONVERGE-RECON-1 | done 已验收 pass（技术面）——报告七节+分阶段卡草案在卷（阶段 1 五卡 5.5–8.5 天/阶段 2 七卡/关键路径 CONVERGE-SHELL-1）；**分阶段方案候用户过目发卡**；Godot 重试导航收编点纳入 ENTRY-1 |

### 展演能力线（2026-11-15 深夜进度驱动重排，[裁定](../decisions/2026-11-15-progress-driven-ruling.md)：无日期锚/无工期，按依赖就绪度并行）

| 池序 | 卡 ID | 状态 |
|---|---|---|
| 369 | G0-SCORE-SPIKE-1 | done 已验收 pass——**选型定案：VexFlow 5+自研 beat-grid**（双 spike exit 0 亲核；Verovio=材料显示可选件候裁定；内核无 set_time_signature→375 承接）；主开发卡草案→374 |
| 370 | G0-LEGAME-CONTRACT-1 | done 已验收 pass（技术面）——[docs/LEGAME_DESIGN_CONTRACT_V0.md](../../docs/LEGAME_DESIGN_CONTRACT_V0.md) 34 节+覆盖 119/119；**R1-R6 候用户裁定（主管整理带建议裁定单）**；W01-A/B/C 实施卡候裁定回填后发 |
| 371 | G0-STACKGATE-1 | done 已验收 pass 合入（[docs/STACKGATE_DG1_DG3_V1.md](../../docs/STACKGATE_DG1_DG3_V1.md)=现行关卡契约——DG1 三能力关/DG2 旅程 J1-J10/DG3 渲染手测 M1-M4+卡面引用模板；锚点抽核真实；CURRENT-STATE 索引登记随 gate） |
| 372 | GEN-YUE-WIRE-1 | done 已验收 pass 合入（**H6 六项真栈证成 W0-W8**：MIDI→方言/prompt 映射/纯器乐/请求卫生/后台 job/引擎托管；C1 逐音 16/16+C8 双版 WAV；W9=已知概率边界裁定在卡；**M1 用户听核 2026-11-16：可听性 PASS/纯器乐+v2 完整性 OPEN→381 取证**） |
| 373 | SRC-TEMPLATE-1 | done 已验收 pass 合入（**DG1-B 真栈 B1-B7 全绿**：melody-drum-v1 模板（2/4/80/C+Surge 血统+鼓 groove）+coursetemplate 包；save→.vit 语义发现入卷；**M2 用户听核 2026-11-16：机械面 PASS/音色域否决→换音源新裁定候用户**（lesson_project.vit）） |
| 374 | SCORE-WORKBENCH-1 | done 已验收 pass 合入（**M1-M5 全过**：模型/推导规则 v1/编辑全集（本地缓冲+显式 apply）/内核真写回读/随谱游标 fail-honest/DG1-A 十五断言真栈；vitest 668 主管复跑；A7/A8→377、D1-D3→378 承接；**M4 终验 PASS 2026-11-17（交互无问题）**；谱面产品语义升级→392 承接〔裁定档 2026-11-17〕） |
| 375 | KERNEL-TIMESIG-1 | done 已验收 pass 合入（set_time_signature+创建 2/4 应用+§11 全过亲证（ctest 15/15+T1-T5 真栈）；实现→main aea3c37a/456b96ef；**374 M3 解锁**；legacy.command 语义发现入卷） |
| 376 | UI-SMOKE-AUTOASSERT-1 | done 已验收 pass（渲染断言对齐 SHELL-5 自动执行态——主管复跑七段全 PASS 亲证；374 M5 阻塞解除；验收缺口归因闭环） |
| 377 | SCORE-DG1A-COMPLETE-1 | done 已验收 pass 合入（**DG1-A 全断言闭环**：A7 探针非静音+A8 差分 18334，十九断言真栈 exit 0；三门 668/668；attempt1-5 分记；**D4 域外锚点→382 承接**〔非 120bpm 谱面装载崩溃〕；栈协议全合规） |
| 378 | KERNEL-SAVE-FORENSICS-1 | done 已验收 pass 合入（**D1/D2 五腿证伪不重现**〔374 环境窗部署漂移归因〕；**D3 归因修正+最小修复**=effective_project_path 三层投影——374 归因被证伪；真栈 29/29 exit 0+栈协议全合规；config 事故已修复主管亲核零残留） |
| 379 | WIDGET-KIT-1 | done 已验收 pass 合入 fc85c016（AudioPlayer+ComparePair+卡挂载；三红线一正一负断言；**698/698 主管复跑**+render 9 段；App.tsx 零触碰；真实内核预览链路留真栈卡） |
| 380 | MATERIAL-TRANSCRIBE-1 | 部分完成（批1五条transcribed_unverified；批2-4十五条候续派，保留uncertain/未听核边界） |
| 381 | YUE-VOCAL-FORENSICS-1 | done 已验收 pass 合入 1001a216+补腿 731a7d9f→main（C8 双版时长比例窗 [0.6,1.5] 干跑 0；**人声定性后被 390 查证修正**：官方有 instrumental 工作流，差距在本地封装） |
| 390 | YUE-INSTRUMENTAL-PATH-1 | doing 零栈两腿 pass 合入（**模型件齐+接入=纯封装层**：S1/S2/S4/S6 官方语义对齐+干跑断言+go 回归 0；**真栈生成+用户听核候栈**——人声抑制效果无先例，fallback 待命） |
| 391 | SHELL-WIN-BEHAVIOR-1 | **done 用户手测 PASS**（效果非常好；416 后续项已修；对话流列表另立 417-419；[终验](../rulings/2026-11-18-SHELL-WIN-BEHAVIOR-1-handtest-pass.md)） |
| 392 | SCORE-TRACK-BIND-1 | 父卡（397绑定→398跟随→399小条；400草稿契约先行，父卡不可重复领；谱面轨道绑定+双模式：轨道选择器+GUI 跟随+clip 小条+草稿 clip 资产不落位；374 痛点承接；裁定档 §二） |
| 393 | CHAT-STREAM-LIST-1 | **done superseded**（需求升级为 417/418/419 常驻左栏+对话流模型；未执行实现；[决定档](../decisions/2026-11-18-entry-rail-conversation-model.md)） |
| —— | music_score 对话流控件 | 登记阶段 2 承接（裁定档 §二：374 内核包装成卡内挂载组件；候 388/389 收口后随阶段 2 发卡） |
| 382 | SCORE-ENTRY-REACH-1 | done 已验收 pass 合入（**D4 归因翻案**：内核命令面正确〔377 归因撤销〕，真因=373 模板 MIDICLIP 拍数值被 tracktion loader 当秒读→**模板资产修正并入 384 腿 D**；ErrorBoundary+「打开工作台」入口+J1-J3 旅程断言；702/702 主管复跑+真栈 28/28；**用户 M4 重测就绪**〔VitShell 进工程→打开工作台〕） |
| 383 | LAUNCHER-RECENTS-CLICK-1 | done 已验收 pass 合入（**磁带卡可点=用户刚需落地**：点击→治理→打开全链+移走/损坏降级+置灰；706/706 port 分支主管亲证+真栈 RC1-RC3 exit 0；LLM 上界 7=attempt1 装置轮如实记；**用户终验 PASS 2026-11-17「可点击加载工程，没问题」**） |
| 384 | SRC-PRESET-AGENT-1 | doing 腿A+E pass 合入（**instruments 语义面落地**：GM 128 全表+乐器词→候选（钢琴→Grand Rhapsody 实证）+Surge 624 预设索引+四态降级；**state 编码三层全解**=JUCE base64 双证+fxp→state 剥头即得；腿 D 闸门已判 **binary_309_identical=内核亲存路径**〔fxp 离线合成证伪；Global Volume 不入 patch 序列化〕；腿 D pass 合入=模板拍数修正+音色换血+state 固化〔真栈 12/12，**Waves 全系 RT-SILENT=会响的钢琴本机不存在**——与 329 授权取证呼应〕；**余腿 C=M2 重测+点音色演示候用户在场，技术面收口**） |
| 291 | ARR-PRESET-1 | todo **提序候选**（384 勘察证成：运行时预设切换=本卡内核域（258 报告命令面草案）；用户底线的「预设级」一段需此卡——排期待主管与用户定，真栈队列 377→378→382→383 后） |
| 385 | T9-PRE-1A-1 | done 已验收 pass 合入 8720d5ee（App.tsx 11857→9448/七模块 2861 行/172/172 逐字亲证/**668=668 主管复跑**+build+render 8 段；386 解锁候 379） |
| 386 | T9-PRE-1B-1 | done 已验收 pass 合入 dd367207（App.tsx **9448→2335 行**/17 域模块；698=698 主管复跑+render 9 段；过程事故 2 起披露在卷无权威丢弃；**387 解锁**） |
| 387 | CONVERGE-SHELL-1 | done 技术面 pass 合入（**366 磁带卡落地**：磁带盒整卡结构主管截图亲核+渲染断言+真栈 ui_entry2 四腿；**366 用户目检已于2026-11-17终验PASS**；ui_entry_smoke 腿1 前提失效+data-tape-badge 断言对齐→389 承接） |
| 388 | CONVERGE-ENTRY-1 | done 主管PASS合入be43a2a2（Codex /root/entry388 / Sol-high；收敛阶段1卡4：工程管理挂载+进出语义；**387 合入后解锁**；Godot 收编点 A 另微改卡） |
| 389 | CONVERGE-CLEANUP-1 | done 主管技术PASS合入036b7eaf（收敛阶段1卡5：旧面退役+阶段出口入口已按用户11-17更新为VitShell.cmd；**388 合入后解锁**） |
| 394 | SHELL-WIN-NATIVE-1 | blocked（E2窗口仍iconic，待403恢复语义取证；391-A，Sol/xhigh；/root/window391；前端仓VitShell域，零栈先行） |
| 395 | SHELL-WIN-PRESENTATION-1 | blocked待复核（既有722测/31渲染与组合11报告保留；409已提供新窗口基线，须对齐协议并复核组合真栈，未自动领取或最终验收；391-B，真栈串行） |
| 396 | GODOT-RETRY-LAUNCHER-1 | doing（/root/retry396，Luna/high；收编点A独立微卡，前端仓gd域） |

- 2026-11-17协调说明：384重复todo快照保留至archive，doing为权威；390物理归位doing，既有owner未变。当前PC主管Codex /root，子代理自主执行授权见decisions/2026-11-17-autonomous-subagent-execution.md。

| 397 | SCORE-BINDING-SAFE-1 | **done PASS**（零栈 717+集成 733；真栈 attempt2 33/33 exit 0，run_20261118_202521；收口 e882267e→main aa3e9b47；S2/S3/S4 边界；[裁定](../rulings/2026-11-18-SCORE-BINDING-SAFE-1-pass.md)） |
| 398 | SCORE-GUI-FOLLOW-1 | todo（依397，GUI真实选择装配/反向回读） |
| 399 | SCORE-CLIP-STRIP-1 | todo（依397/398，clip时间小条） |
| 400 | SCORE-DRAFT-CONTRACT-1 | todo（只读契约，工程包草稿库前提未成立） |
| 401 | LAUNCHER-RECENTS-READ-1 | done 主管PASS合入be43a2a2（388必要只读接缝，同owner独立提交；GET不经LLM/不物化工作区） |
| 402 | SHELL-WORKSPACE-READY-1 | blocked（最终验证ownerentry388，native bootstrap恢复未过；394取证必要接缝：生产装配+首份工程快照回执，配对失败/超时） |

| 403 | WIN32-NOACTIVATE-RESTORE-FORENSICS-1 | done 主管取证PASS（自窗6/6+404补独立Godot2/2；非产品修复，普通大窗geometry/z→405） |
| 404 | GODOT-MAIN-HWND-OBSERVATION-1 | done 主管取证PASS（精确主HWND+23纯关联/14门禁反例+实际Godot两条件0；测试装置合入，不含失败产品候选） |
| 405 | GODOT-NOACTIVATE-GEOMETRY-1 | blocked（R2首ordinary尺寸正确但engine4/紧邻门失败；试验具体化纠正→407，原FAIL保留） |
| 406 | WINDOW-TRANSITION-OBSERVER-1 | doing（Sol/high /root/restore403；测试专用自进程观察DLL+纯无窗策略，供405因果geometry证据；不改生产、不授启窗） |
| 407 | ORDINARY-LARGE-WINDOW-1 | blocked（R2尺寸/mode0/noFG过但rawdownprefix证child在guard前；目标自线程z事务→408） |
| 408 | TARGET-Z-TRANSACTION-1 | done 取证PASS收口（11-17晚GLM亲证：z事务机制全绿——1821/1821三phase配对+guard_after 5/5锚定已核reference+16DIP+窗口期零FG/focus；runtime_01停机期焦点违约=产品窗外known behavior〔用户裁定A〕；工件9873eaf1；external腿转409） |
| 409 | SHELL-WIN-ZPRODUCT-1 | done 已验收合入（11-18 Codex主管；shell06/external07真栈均exit0，首show在下/16DIP/消息区间零激活焦点/无后纠正；前端bc8daf6、主83bd5e77；仅当前Windows开发入口，391仍待395） |

### 窗口线收口（2026-11-18 晚，PC 主管交接后；用户批准方案 A）

| 池序 | 卡 ID | 状态 |
|---|---|---|
| 410 | SHELL-WIN-PRODUCER-1 | **done 实现面 PASS**（主管复跑 build+ctest 3/3；4d450b9→前端 main 3bceb3b；[裁定](../rulings/2026-11-18-SHELL-WIN-PRODUCER-1-pass.md)；真实链路待 414） |
| 411 | SHELL-READY-SEAM-1 | **done 实现面 PASS**（主管复跑 import+unit runner exit 0；4784f9b→前端 main cf33195b；[裁定](../rulings/2026-11-18-SHELL-READY-SEAM-1-pass.md)；真实装配待 414） |
| 412 | SHELL-WIN-PRES-MERGE-1 | **done 实现面 PASS**（主管独立复跑 722 测/build/31 渲染/全段 render；c44c9039→main 668078fd；[裁定](../rulings/2026-11-18-SHELL-WIN-PRES-MERGE-1-pass.md)；真实端侧待 414） |
| 413 | SHELL-ENTRY-REFRESH-1 | doing（lead/entry413；先隔离扩展 DLL 预构建，410/411 合入后主管放行刷新最终 VitShell/webui/入口产物；零栈） |
| 414 | SHELL-WIN-COMBO-1 | **done PASS**（attempt03 hands-off：409 分类器 PASS + 395 呈现 PASS + chain_detail=workspace_ready；零 LLM/卫生还原；[裁定](../rulings/2026-11-18-SHELL-WIN-COMBO-1-pass.md)；run_20261118_190253） |
| 415 | SHELL-WIN-STAGE-PARSE-FIX-1 | **done PASS**（主管复跑 ctest 3/3；真实链 workspace_ready 生效；8c2c2b44→前端 main 8c3801f；[裁定](../rulings/2026-11-18-SHELL-WIN-STAGE-PARSE-FIX-1-pass.md)） |
| 416 | SHELL-RING-SPIN-FIX-1 | **done PASS**（加载卡状态环轴心居中/去 translateY 覆盖；主管复跑 722 测+35 渲染+全段 render；2160104f→主仓 main 88f4f163；[裁定](../rulings/2026-11-18-SHELL-RING-SPIN-FIX-1-pass.md)） |
| 417 | SHELL-ENTRY-RAIL-1 | **done PASS**（B1 壳层+常驻左栏；主管复跑 722 测+50 渲染检查+全段 render；d231d67e→main f529e3f2；[裁定](../rulings/2026-11-18-SHELL-ENTRY-RAIL-1-pass.md)） |
| 418 | CONV-RAIL-1 | **done PASS**（B2a 对话流树/只读预览/继续/新对话/footer/持久化；主管 735 测+73 渲染检查+全段 render；9b1b2d58→main c2f7b2c2；[裁定](../rulings/2026-11-18-CONV-RAIL-1-pass.md)） |
| 419 | CONV-RAIL-CROSS-1 | todo 候 418（B2b：跨工作树只读浏览+【打开此工作树并继续此对话】+组合工具单次重载+真栈） |
| 420 | MATERIAL-TRANSCRIBE-2 | **done PASS**（15/15→transcribed_unverified；22=20 unverified+2 checked；主管复跑 validate/final_check 0；44ad301c→main 25b58b8a；[裁定](../rulings/2026-11-18-MATERIAL-TRANSCRIBE-2-pass.md)） |
| 419 | CONV-OPEN-TOOL-1 | **done PASS**（agent 组合工具 version.conversation_open：跨树 autosave+单 reload+turn 守卫+失败披露；e6184e1d/4554e8e8→main 8d9b8f4e；[裁定](../rulings/2026-11-18-CONV-OPEN-TOOL-1-pass.md)） |

- 目标链：410/411/412 并发零栈 → 413 重建 → 414 单次真栈组合 → 391 手测放行。409 RULING 不重开；395/402 原 blocked 卡待本批收口后归档/收口。
- 交接记录：[decisions/2026-11-18-pc-supervisor-handoff-planA.md](../decisions/2026-11-18-pc-supervisor-handoff-planA.md)。














