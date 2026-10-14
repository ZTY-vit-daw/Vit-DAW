# M2-TIMEFX-1：reverb/delay 语义控制面（时域效果器控制·语义侧）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=用户三线同步：M2 时域=混响/延迟类效果器；MIX-CAP-RECON §2.B/§3 C3 行）
- 派发确认：已确认（用户 2026-10-11 晚窗同步）
- 验收负责人：GLM 主管决策流
- 池序 71；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / **MIXLAYER-M1-3 合入后领取**（C3 观察侧与控制侧同域，先观察后控制）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- **定义钉死（用户裁定）**：时域效果器=**混响、延迟**类；动态族（compressor/de-esser/transient/limiter）属 C2 已完成，不入本卡。

## 目标

reverb/delay 两族按 C2 动态族现役模式接入语义控制面：

1. **域表扩展**：d1s1_domains.go 四族模式（SemanticIntentCoverage 轴）复制出 reverb/delay 两族（语义意图→插件参数映射，如 reverb=decay/pre-dry/wet、delay=time/feedback——词表设计执行侧定，风格参照 attestation coverage 现有轴）。
2. **参数抓取/inspector 面**：MIXLAYER 勘察结论"plugin_grabber 无 reverb/delay inspector"——领取时 grep 实际落点（勘察名词可能非代码符号名），把 reverb/delay 参数快照纳入抓取面。
3. **写回链接入**：既有 set_plugin_param / plugin.set_params_batch（readback+base_revision）通道直接复用，零新协议。
4. **自由态可达**：话术/意图路由进自由态循环（fastpath 注册清单完整性契约模式），非编排独占。

## 文件域

chat/capability seam + 域表文件（d1s1_domains.go 同级）+ 对应单测；与 M1-3 领取记录核对文件域。

## 验收标准

- 单测：域表两族映射+越界 fail-closed；「reverb/delay 语义控制在自由态循环内可达」冒测断言。
- 真栈烟测（AGENTS §5，扩展现有 ps1 体系）：reverb 或 delay 一族语义话术→确认→写参→回读一致，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

域表/写回链锚点与卡面不符（上游已变）→ 锚点上交。

- 领取：2026-10-12 PC 晚窗 / origin/main=813cbf5850225d3edc0f70ba609a6f137d54a27e / owner=GLM-5.3 执行流（ZCode PC 会话，M2 时域线）/ 分支=port/m2-timefx-1 / worktree=D:/Vit_DAW_wt_m2_timefx1 / 领取提交=91c7b012
- 回执：实现 commits **5de7c450+caf76fa2**@port/m2-timefx-1（已推远端）；栈占用 a7fccf41 → 释放（本批）。`go test ./... -count=1` **GOTEST_EXIT=0（93 包 ok 零 FAIL）**；gofmt 净（新文件全 LF 对齐净；触碰的 CRLF 既有文件对齐复核净，M1-2/M1-3 先例口径）。
  - **① 域表两行（验收 1）**：`experiment/d1s1_domains.go` reverb/delay 行——PluginBound 单通道、Journal=set_plugin_param、ActionIDSuffix _rev/_dly、CapabilityID static_mix.reverb.v0/static_mix.delay.v0、ObservationViewIDs=mix.multitrack_relationship（族专用成对差分视图=M2-TIMEFX-2 域，行内注释钉界）。**量纲执行侧实测定**：reverb=decay_pct ±8pp（Pro-R 表面 Decay Rate 显示百分比，2026-10-12 内核探测亲测；上限<10 由 sealed 测试 TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound 的「非 dB 域拒绝 dB 累计量纲」不变量钉住）；delay=feedback_pct ±8pp（Timeless 3 Feedback 显示 %）。TargetSemantics=**delta_physical**（新标记=原 delta-on-current 机的原生单位形态；port 同机双标记接受，既有 delta_db 行字节不变）；非 dB 行 multi-round fail-closed（pan 先例）+NaN/Inf 拒绝（越界 fail-closed 单测锁）。**SemanticIntentCoverage=单主轴**（reverb=decay_time、delay=feedback_amount——FAM 全族惯例；词表全集 decay/pre_delay/wet 与 time/feedback/wet 落在 PCA v2 族轴表+白名单条目，行内注释说明单证覆盖不变量）。
  - **② 抓取/inspector 面（验收 2）**：勘察名词实测落点=plugingrabber 每族 topology 识别器+chat 侧族工具。新增 `reverb_topology.go`/`delay_topology.go`（tail/timing/feedback/character/output 分节；可证核心=decay 证据/echo-time 证据；delay 面进 reverb 识别器→unsupported_delay 边界、reverb 面进 delay 识别器→unsupported_reverb 边界、wet-only→unresolved、动态族→各自 unsupported 边界）+四工具 plugin_grabber.inspect_reverb/.apply_reverb_controls/.inspect_delay/.apply_delay_controls（control_ref/restore_ref 编解码+原子事务+全 preimage 回滚，limiter 同款）——工具注册五点齐（catalog alias+argspec+spec / server.go HTTP / goalrunner 执行器路由+工具清单 / 认证工具对映射）。**真栈实证**：inspect 两工具对真实 Pro-R(89 参)/Timeless 3(1011 参) 表面全通过（classification=predelay_decay_reverb/feedback_delay，词表 id 亲测=decay'1'/predelay'74'/mix'6'、time'0'/feedback'85'/mix'160'，control_ref 全挂；工件 inspect_real_check.json）。
  - **③ 写回链（验收 3）**：零新协议兑现——执行走 executeD1StaticEQ 通用 PluginBound 链→resolveD1PluginParamWhitelistBinding（新增两分支+attestation 读取器）→StaticEQVSPPort normalized_batch_v1（plugin.set_params_batch readback+base_revision）。白名单 schema v6 增 reverb/delay 两 section（ReverbPlugin{decay/pre_delay/wet id}/DelayPlugin{time/feedback/wet id}，v5/v6 双形兼容、DisallowUnknownFields 校验、Validate*Admission 走 v2 库）；机器本地白名单已落盘（备份 .bak_20261012_m2timefx）。
  - **④ PCA 闸同步**：processorattestation v2 增 FamilyReverb/FamilyDelay+coverage 轴表+role→axis 映射；processorintent 家族面；processorregistry 两族定义（ObservationViews 暂绑 mix.multitrack_relationship+注释钉 M2-TIMEFX-2 界）；semanticDynamicSpecForFamily 两 spec；processorAttestationFamily/推荐需求标签/策略 prompt 枚举/认证族清单+工具对——「先开枪后装安全」红线不出现。**真栈发证**：pcactl 认证线（in-agent job）两族各两轮全 completed——apply/readback/restore/全参数快照核对/临时轨清理（jobs 86c8ae2a/c471a907/1980b350/595dd30b）；最终 attestation **promoted**：Pro-R=decay_time+output_normalization+wet_balance（rev79 pca2_16b17b51c4984c6）、Timeless 3=feedback_amount+spatial_mode+wet_balance（pca2_19d2d7871c22c5c）。**认证机两处泛化**：可逆目标选择器增测量曲线样本源（连续无 domain 控件如 decay 曲线 50-200% 可入证）+可逆证明上限 2→3 distinct-axis（时域词表 3 轴 vs 单证覆盖不变量；动态族行为不变——其表面可逆轴数不变）。
  - **⑤ 自由态可达（验收 4）**：域表=自由态实验面本体（RECON §1.5），prompt 域规则/入口/确认分发全表驱动自动生效；单测=注册完整性契约（M1-3 模式，m2_timefx_registration_test.go：八点同 ID——协议词表/域表/intent 家族/D2-SEMINT1 冻结 intent 可 Decode/PCA v2 族/披露面/终端措辞/processorType 映射+四工具路由）+resolver PCA fail-closed（未提升/未配置双边界）。**真栈 NL 实证（run 105611）**：reverb 话术→自由态循环→needs_experiment→**admission reverb/reverb_decay_adjust/decay_pct=-4 准入成功**（域选择门通过；工件 run105611_admission_reverb.json）。
  - **⑥ 冒测扩展（验收 5）**：run_free_state_d1_smoke.ps1 PromptFlavor/ExpectDomain ValidateSet +free_state_d1_smoke.py PROMPT_FLAVORS（四要素同构话术）+ADMITTED_DOMAIN_KINDS 两行——复跑即 -PromptFlavor reverb -ExpectDomain reverb。
  - **⑦ 真栈 NL 全链腿——环境阻塞如实申报**：四轮真栈 run（105611=准入成功后 registry 门暴露→修；110302=family_not_registered 修复后推荐门暴露→修；110817=冻结轴覆盖门暴露→单主轴+发证泛化修复；111841=**API 额度耗尽**「用户剩余额度 0.013072 < 预扣 0.026238」首个 LLM 调用即失败=环境中断，原始证据 run111841_api_quota_block.json reply 字段）。**已证**：话术→准入（105611）+写参→回读一致（认证线两族 typed apply/readback/restore 全过，走同一 set_plugin_param 通道）+全部门修复后链路通达（110817 轮已推进到推荐覆盖门，修复后无再跑机会——额度中断）；**未证**：修复后全链单 run exit 0。**[等待用户：API 额度恢复后复跑 `run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` 一轮即收]**（§8 声明：非模型随机分支，原始日志在卷；不得把认证线+准入轮拼写成全链完成）。
  - **⑧ 文件域如实声明**：卡面「chat/capability seam+域表同级+单测」实际触碰面=experiment(域表/量纲)+experimentplugins+processorattestation/processorintent/processorregistry/processorauthority+executionports(delta_physical 标记)+chat seam 全族注册点+plugingrabber 两 topology+tools catalog+scripts 两文件+cmd/pcactl——全部为「C2 族现役模式」复制的固有注册面（M1-3 先例：seam 入口必改点如实披露）；内核零改动（复用主检出 VitApp.exe B6565DCF85D1DA86 同版二进制）。
  - **⑨ 事故如实记录**：①首两轮烟测 agent 二进制落后于修复（-SkipBuild 用既有 bin）——第三轮起每轮重建后复跑；②105611 轮 interventions=0 的失败归因三轮递进定位（registry→推荐族映射→轴覆盖）各自修复后才见额度中断——三轮均为功能性门暴露而非凑通过，无断言弱化（sealed 测试仅按新现实扩清单：registry 计数 9→11+注释/adapter 矩阵 reverb/delay 迁入 cases/措辞镜像测试增原生单位分支/delay 上限 10→8 由 sealed 不变量驱动）。
- 验收：**pass（一项补证挂账）** / 2026-10-12 晚窗（[rulings/2026-10-12-EVENING-BATCH1-rulings.md](../../rulings/2026-10-12-EVENING-BATCH1-rulings.md)；决策侧合并态亲跑 build 0+95 包 0 FAIL+gofmt blob 净+工件亲证；额度中断腿挂账复跑一轮即收；impl 双 merge 合入 main；M2-TIMEFX-2 解锁）
- **挂账销账（2026-10-14 批量六）**：API 额度复跑挂账（run 111841）**已销**——109 rerun6 双 exit 0（run12/13，deepseek-flash 思考档同基线）超额达成（[rulings/2026-10-14-BATCH6-rulings.md](../../rulings/2026-10-14-BATCH6-rulings.md)②）。
