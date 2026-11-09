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
| 271 | CLOSURE-ROUTE-ADAPT-1 | doing（实现轻验 pass；验证腿候便车搭后续 run） |
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
| 326 | FIX-RESTARTAGENT-ENV-1 | todo 可领（PENDING 分流②：脚本重启 agent env 透传；真栈轻腿可搭 282 同窗） |
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

C5 设计（E 前置=主总线处理）→ E 批次 → F rubric；E2E-WEBUI-1/JOURNEY-1 渲染面/旅程烟测债；154/155 卫生项；AF 体检下轮抽查（261 其余三腿）。

## 二、三线（乐局/编曲/生成）——按 [11 月三线执行排序](../reports/2026-11-05-nov-three-line-execution-plan.md)（领卡顺序权威）

- **首波收口**：276/277/278/279/280/281 全终验合入（**278 整卡 pass：chat 唱歌 exit 0=注册链③ 收口**，生成线对话式唱歌全链打通）；注册链④（282 实现腿轻验 pass 2026-11-08：引导段落 ruleset shared 段+四面注册+域外 4 行追认；真栈轻腿候栈）；
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
| 330 | FIX-PLAN-STALE-STATUS-1 | todo 可领（327 已合入解锁——plan 面 stale 伪造 completed 同型修） |
| 331 | VST3-SIGNAL-HYGIENE-1 | todo P3（309 S1 卫生：setup/per-call 信令一致化——**不占展演前窗口**） |
| 332 | AI-LAUNCHER-DESIGN-1 | done 自验候验收（九节设计报告已产出——三态一窗/对话流两案 A 主 B 退/反向导航零内核改动/拆卡 6 卡 8.75 人日中值；候主管验收+用户终审后拆实施卡） |
| 333 | LEGAME-GENSCOPE-1 | done 已验收 pass 合入 82f42846（手测 v2 旅程面收口：只走旅程 A，零生成前置；边界声明含 AI-first 衔接） |
