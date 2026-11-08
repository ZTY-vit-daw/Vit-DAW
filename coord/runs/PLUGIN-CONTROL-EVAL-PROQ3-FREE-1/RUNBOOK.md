# PLUGIN-CONTROL-EVAL-PROQ3-FREE-1 跑前口径（§8 成文，跑前冻结）

- 卡：coord/cards/doing/2026-11-08-PLUGIN-CONTROL-EVAL-PROQ3-FREE-1.md（领取 2026-11-08 22:56 @ origin/main=1883021b→领取提交 d95dc537；worktree=D:/Vit_DAW_wt_320proq3 干净基线）
- 本卡=T7 自由调控 Pro-Q 3 终审（用户裁定 2026-11-08 深夜）：**早期开发第一个碰壁处**——模型自行选择调控但必须自报频段/频率/Q/增益/类型；**终态保留供用户 GUI 亲验**。判定权=主管+用户 GUI 终审（本 run 只产证据，不出最终判定）。
- 复用件：318 装置模式（`coord/runs/PLUGIN-CONTROL-EVAL-BAREFREE-1/`，main 在库）——冻结 runner trio **只读 import**（transport/param diff/budget/confirmation/telemetry 解析复用，不改动）；311 布位先例（`scripts/m2_handtest_prep.py`——%APPDATA%\Vit-DAW\projects 持久布位+recent 启动页同源面+reload 后取 id 纪律）。

## 一、装置（本目录四件）

- `proq3_free_cases.py`：T7 单案定义（**六案例体系外新 case 文件**）——任务话术卡面原文逐字冻结；Pro-Q 3 registry 预核行（identifier=VST3-Pro-Q 3-a1aba170-ba5f7ab2，FE-LANE-VIEW-TUNING-1 BridgeReplies 亲读）；三轮工程名「实验·ProQ3 裸控 r1/r2/r3」；真实音频素材=B6 渲染 20s 立体声 48k/24bit（只读源，import 复制入工程域）；per-round/segment caps 与冻结 runner 同值；失败分类=冻结 plan 全集+新增类。
- `proq3_free_driver.py`：driver（selftest/device/round/consolidate 四子命令）。**agent 源码零改动**（卡面约束）。
- `proq3_free_orchestrator.ps1`：栈编排（318 编排复刻+T7 段构成）。
- 本 RUNBOOK：跑前冻结。

## 二、run 配置（唯一差异面=任务本身）

- env：`VIT_PLUGIN_CONTROL_EVAL_SEAM=1` + `VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1`（BAREFREE 配置全开；无检索缝、无提示缝；均在任何进程启动前置入）。模型面=产品默认 ~/.vit/config.json（deepseek-flash）；env 模型 override 在场即拒跑。
- 内核=wt_228 VitApp.exe SHA256 F9E3DA0A…（300-318 同一二进制只读复用，hash 不符=环境中断）；agent=本 worktree 现场构建（main 1883021b+d95dc537 协调笔，源码零改动；SHA256 入报告）。

## 三、段构成（单一栈窗口顺序执行）

1. **净空 preflight**（编排 phase 0）：六 TCP+两 UDP+栈进程全零。
2. **env freeze**（phase 1）：VIT_PROJECT_XML=git-clean 副本+遥测/渲染/载荷捕获路径+两缝 env+模型面记录；布位目标已存在即拒跑（重跑卫生，不覆盖）。
3. **起栈**（phase 2）：dev_agent_smoke -StartKernel -RestartAgent（无 UI；用户 GUI 终审是**事后另启**的用户旅程，不在本 run 断言面）。
4. **device 腿（零 LLM）**（phase 4）：五门——①registry 亲证（plugin_search 解析到冻结 identifier；**失败=卡面停止条件**，exit 4 取证上交）；②布位 r1/r2/r3（new_project→save_as→add_track→clip.import_audio[真实音频]→rack_add_node[Pro-Q 3]→save→open→reload 核对单轨单插件单剪辑）；③初始态等价（三副本 fixture digest 全等）；④known-op 裸写亲证（agent invoke 面 set_plugin_param 必须落地+独立回读变化+复原；**失败=缝未在 Pro-Q 3 面生效=装置前提失效**，exit 4）；⑤save/open 持久化门（改动值过 save→open 仍在+复原后 digest 回初始=终态可保留给用户 GUI；**失败=交付前提失效**，exit 4）。任一门失败→不进付费轮。
5. **三轮 T7**（phase 5，付费）：每轮=open(rN)→workspace 激活等待→初始 readback→新会话+冻结话术→确认面 approve/澄清追问 ≤3/预算帽内循环→终答全文=自报告→post readback→mutation diff（vs 初始态）→边界核对（仍单轨单插件）→save→open→reopen readback（持久化=after 与 reopen diff 为空）。**无 teardown——终态即用户 GUI 亲验物**。
6. **consolidate**（phase 6，离线）：三轮并列+机械 claim 抽取（Hz/kHz/dB/Q/滤波器类型词）+漏报项如实列+语料旗标+对账表草稿（reconciliation_draft.md）。

## 四、§8 声明（运行次数/预算/失败分类/止损——跑前冻结）

- **运行次数**：单 run 无重跑（run_id=启动时间戳，新目录不覆盖）。任务轮恰好 3（r1/r2/r3 各一，每轮新会话+独立工程副本）；device 腿零 LLM；失败轮不重跑按 taxonomy 记录；环境中断原始退出证据在卷才可排除，补跑=新 run_id；环境重建上限 1 次/批（预冻结，装置脚本缺陷类在内，312/318 先例）。
- **每轮上限**（超限停轮记 model_budget_exhausted:*）：LLM 调用 12/轮；单调用输入 40k/输出 8k；工具调用 30/轮；轮墙钟 20 分钟。
- **段级上限**（轮间检查，超限停段）：合计 ≤36 调用/≤1,440,000 输入/≤288,000 输出 tokens。
- **失败分类**：冻结 plan taxonomy 全集 + 本卡新增：`plugin_load_failure`（Pro-Q 3 不在 registry/无法装载=停止条件，取证上交，换插件须用户裁定）；`guard_blocked_write`（318 修正类：守卫拦截不误标 value_encoding_error；缝开态再现守卫拦截=先查缝失效，缝外新守卫面=锚点上交不扩修——**319 harness 第三层缝未合入 main**，T1 型 broad-mix 判定若拦 EQ 写即按此类如实记录）；`seam_leakage`（typed 执行/检索执行等装置面泄露→停段隔离证据）；**`self_report_mismatch`（本卡新类，诚实度观察项——自报与 readback 实际不符）**；`persistence_failure`（终态未过 save/open）。行为标签分列；环境中断（栈占用/端口/进程/网络，LLM 60s 超时按 296 模式记单轮环境类）。
- **止损线**：①device 任一门失败=exit 4 停段取证（不进付费轮）；②连续两轮同型环境类失败→止损取证上交；③同代码版本同确定性断点两连败→禁原样重跑先取证；④typed 执行/检索执行→立即停批隔离证据。止损只停投入，不改写未发生结果。
- **计费**：用户 DeepSeek 账户（deepseek-flash）；预算 ≈¥1 上界（3 轮×≤12 调用≈36 调用/≤0.6M in 量级；318 参考价 108 调用/1.58M in≈¥2-3）；per-round tokens 逐轮入成绩表；cost 字段=missing（价目未核不事后折算）。
- **退出码**：0=3 轮全评估+consolidate 全绿（≠任务成功≠终审通过——判定权在主管+用户 GUI）；1=driver/编排层失败或 consolidate 缺轮；2=环境中断/栈占用；3=段预算/止损停止；4=装置验证失败（registry/known-op/持久化——含卡面停止条件）。
- **评价独立面**：T7 无目标值判据（开放自选）；证据面=readback diff+自报告全文并列呈现，**机械抽取不承载语义对号**——「模型说动了什么/实际动了什么」最终对号归主管+用户 GUI 终审。评价器期望（若有）从未进入被测模型上下文。

## 五、栈协议与工件

- 真栈占用按 PROTOCOL §2.2 全链：净空亲核→占用登记推 main→≤60s 即时复核→启栈→run→owner 亲拆 TEARDOWN_ALL_ZERO 亲核→释放登记（run ID）。**排队 319 之后——登记前重读 coord/resources/PC-RUNTIME-STACK.md 顶部，等待不抢。**
- 工件根=coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/（本目录）：proq3_free_<stamp>/（run 目录：run_report.json 含 HEAD/分支/内核与 agent SHA256/两缝 env 记录/model env 掩码/teardown 亲核；device/ 子目录含 registry_row.json+placement_r*.json+initial_readback_r*.json+known_op_*.json+persistence_probe.json+device_report.json；rounds/rN/ 含 round_record.json+readback_before/after/reopen.json+self_report.md+workspace_activation.json；logs/ 含 telemetry+prompt render+payload capture+agent log 副本；reconciliation_draft.md+proq3_free_summary.json）。
- 工程交付物（用户 GUI 终审物）：`%APPDATA%\Vit-DAW\projects\实验·ProQ3 裸控 r1.vit` / `r2.vit` / `r3.vit`——**run 后保留不删**（用户数据域持久位，311 先例；§10 副本制零权威污染——素材源只读、工程为新建副本）。
- 遥测/载荷捕获 per-run（env 重定向）；密钥不入工件（model env 记录键名掩码）。全部工件随 port/plugin-control-eval-proq3-free-1 分支入库，不合入 main（归主管）。

## 六、装置已知边界（预声明）

1. **GUI 最后一米不在断言面**：本 run 断言 agent HTTP/ZMQ 面（readback=参数表证据）；用户从 Godot 启动页打开工程→机架→Pro-Q 3 GUI 肉眼比对=用户终审旅程，**无 E2E-WEBUI/JOURNEY 覆盖主张**（实验腿不适用渲染面门槛，AGENTS §5 边界如实声明）。recent 启动页同源面=kernel get_recent_projects（m2 先例）——本 run 布位后 device 腿亲证 recent 首项（见 placement 核对），用户侧可见性以此 API 面为准。
2. **319 未合入的 harness 第三层守卫**：本 run 基线 main=1883021b（318 缝在，319 harness 缝不在）。若 T7 请求被判 broad-mix 且 EQ 写入撞 `broadMixObserveFirstWriteGuard`（harness.go:2590），按 guard_blocked_write 如实分类+锚点上交（318 T1 先例），**不扩修不重跑**；三轮同拦=数据点完整上交（T7 答案=「守卫面拦截」）。
3. **机械 claim 抽取的语义边界**：regex 抽取 Hz/kHz/dB/Q/类型词仅为草稿原料；模型若用非单位表述（「中频」「低切」无数字）抽取不全——自报告全文永远并列在卷，对号以全文为准。
4. **known-op probe 参数面**：probe 选名含 gain/freq 的连续参（偏好 Output Gain）；Pro-Q 3 面 ~348 行（FE 在案），MIN_PARAM_ROWS=100 为下界断言。
5. **单轮=单样本**；3 轮=3 独立副本，非稳定性主张。
6. **布位工程的 recent 面与用户工程共存**：%APPDATA%\Vit-DAW\projects 已有「手测·小星星乐局.vit」（311 布位，用户手测物）——本卡布位**不触碰**既有工程，仅新增三副本；run 后保留。
7. **音频素材源**：B6 渲染 wav 只读引用（artifacts 运行时工件域，非权威 fixture）；clip.import_audio 后工程自带媒体副本，素材源文件本 run 不改不删。

## 七、终章批（第三批）跑前口径（§8 增补冻结，2026-11-09 凌晨，主管终章指令）

- **解锁依据**：321 RACK-PARAM-PERSIST-FIX-1 终验 pass 合入 main（01579d7b）——rack 内插件参数持久化修复亲证（probe 3/3：0.7→save→open→0.699999988 persisted，对照本卡未修基线三次全回 0.5）。第二批的持久化门拦截条件解除。
- **内核**：wt_321persist 构建产物 VitApp.exe SHA256=950CF07B7B735749C2DEF5E9447B7260F1272694DC5B83A865790B8E24CD30F5（编排 hash 断言换新；wt_228 F9E3DA0A=未修基线弃用）。
- **agent env 透传**：两缝 env 在任何进程启动前置入编排进程（Start-Process 继承制，run3 known-op 落地=透传实证）；321 域外记录（-RestartAgent 链 env 语义）以**编排 phase 2b psutil 活体亲证**覆盖——agent 进程 env 两键非 "1" = exit 4 不进付费轮。
- **运行次数/预算**：本批单 run 无重跑（新 run 目录）；任务轮恰好 3；预算 ≈¥1（≤36 调用/≤1.44M in/≤288k out，deepseek-flash 用户账户）；失败分类维持 §四全集（self_report_mismatch 观察类维持）；**付费轮后装置/环境失败=取证上交不重跑**；结果三态如实记录（写入落地与否/自报一致性/守卫拦截类）。
- **交付物**：三轮终态工程保留在 %APPDATA%\Vit-DAW\projects（实验·ProQ3 裸控 r1/r2/r3.vit，不删——用户 GUI 终审物）+三轮自报告全文+对账表草稿（reconciliation_draft.md）——用户从 Godot 启动页打开→机架→Pro-Q 3 GUI 肉眼比对频段/频率/Q/增益/类型。
