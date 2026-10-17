# 2026-10-17 晚窗 批量一裁定（K1 验收 / LOUDNESS-B 补证腿 / GENSHELL-PCA-1 PC 复核）

- 验收会话：GLM 主管决策流（PC，ZCode）；裁定来源：用户「任务卡已完成，可以检查」。
- 基线：origin/main=337d62a3；本批合入后 main=f4926543（两笔 no-ff merge）。

## 一、KERNEL-SENDROUTE-IMPL-1（K1，127）——**终验 pass，合入 f4926543 前置 merge**

- 决策侧独立复验（全链亲做）：
  1. **diff 亲读**：SendRouteService.{h,cpp} 1283 行全读——五命令齐；双 bus 三明治（send 偶/return 奇、最小未占用偶数对分配器、悬空 busNum 经 occupiedBusNumbers 全量 aux 扫描保留）；防环闸 1 四类拒绝（FX 作源拒/自送拒/含 AuxSend 轨拒作目标/wouldCreateCycle 可达性闭环保底）；闸 2 构造性链序（SendOut@监测边界+SendIn 紧随其后、FX 侧 AuxReturn 头部+湿 SendOut 末 rack 后、normaliseFxChain 幂等复位）；FX 主推 -96dB+拆解复位 0dB（防静音轨陷阱）；渲染收尾三行与 handleSetFolderRoutingBus 逐字一致（「对齐该模式」申报属实）；auto_create 单事务（beginTransaction 只开一次=undo 原子）、按名幂等复用、重名序号。CommandDispatcher 注册面照 :2303 先例、renderAllowlist 仅放行 list 读命令、rack 变更后钩子限定三条命令+ok 才触发且有界。
  2. **独立构建/测试**：worktree D:/Vit_DAW_wt_sendroute1_build Release 构建 **exit 0**；`ctest -C Release` **11/11 全过**（含 VitSendRouteTests #7）；VitSendRouteTests 直跑五腿数值精确——control 1.0000×/full 2.0001×/single_return 1.5001×/mute_T0 归零/loop_order 有界（≤2.5×R）——**VERDICT PASS failures=0 exit 0**，与探针底座三态可分逐位一致。
  3. **真栈烟测工件亲核**：run3=sendroute_20261017_204856 七腿全 pass——定标 R≈0.1995/connect auto_create+list 读回/防环四拒/幂等/湿渲染 **ratio440=ratio880=2.000008**（湿回注双目标特征值）/单次 undo 全撤/disconnect 幅度**逐位回基线**（0.19946631634214346 与 setup 完全一致）。
  4. **run1/2 败因亲读**（PC-RUNTIME-STACK 释放记录）：run1=16bit 线性区削波+预存 2x 增益致比值非线性（夹具）、run2=工作副本残留 FX 轨+clear_project 只清 clip 不清轨（夹具确定性，服务幂等语义反而正确）——两败不同签名、均为脚本夹具非产品，§8 未触止损，定性采信。
- 偏差三项采信：①侧链走 setSidechainSourceID（Plugin.cpp "N. " 显示名前缀+裸名静默 reset 实证；按 ID 直设同一原生状态面，rename 健壮）；②FX 轨不预建 rack instance（空 rack 卫生修剪+rack_add_node 实体化+归一钩子复位，行为与 §1.4 等价，烟测端到端证明）；③两处构建接线为机械必然。
- 边界三条采信（如实登记）：①dispatcher 归一钩子路径（rack 变更→链序复位）无自动化断言——**K2/K3 真栈腿必经面**（rack 装载到 FX 轨），届时暴露；②侧链验证到状态层（sidechainSourceID 读写回读），侧链音频流未做离线渲染断言；③「新工程旧内核 unknown-plugin」为记录性验收（Tests 组 C 持久化形态断言），旧内核二进制实测未执行。
- **域外发现处置**：stock 内核离线渲染逐声道恒定 ~2x 增益（主检出复现+最小复现在 PC-RUNTIME-STACK 释放记录，归 VitProductionCoordinator/te::Renderer 域）——**另立卡 KERNEL-RENDERGAIN-1（池序 132，P2 取证先行，未确认待用户）**；比值断言免疫，不挡本卡。
- 池面：128 K2（agent 语义）依赖打勾可领。

## 二、LOUDNESS-B-TRIGGER（90）——补证腿**返工裁定（案①）**+实现**阶段一合入 5e9d2f65**

- run2=环境中断定性采信：冷启动 Waves VST3 插件表扫描绑 5555 晚于 20s UI 门（内核日志数百条 class_selection/component_create 实证；当天首次拉起冷盘）；不占止损。脚本清理漏 kernel pid 已由 owner 手动拆净复核=0。
- run3=腿前提失配定性采信（取证件每环有工件锚点）：NL 轮纯文本作答→T1/T3 未触发→钩子 nil（注册点=三钩子自身）→observe 无 loudness_watch 键（**nil-hook 零新增键=设计行为真栈实证**）→脚本 Test-Path 空路径异常。§8 不烧剩余轮次正确：同 fixture 同话术 10+ 历史样本零 settle，模型分支稳定，非随机可期——按 AGENTS §11 前提失配上交合规。
- **run3 已证面入账（不重跑）**：impl 二进制真栈拉起（285 工具）+61 轨全链+LW configure/status HTTP 面（4.6 两门，R128 digest 在卷）+nil-hook 诚实退化。
- **裁定=案①腿重锚**：LW 真栈证明挪到能确定性驱动 settle 的腿（run_free_state_d1_smoke 先例；D1-S1 线经 109 七腿修复链后连续 PASS×2 已稳定）。修订腿内容：①D1 settle 后接 LW observe/verdict 落盘/dedupe/配置关静默断言+E1 提取（extract_e1.py 已备）；②脚本空 verdicts_path 守卫+Read-LwVerdictStore 空路径不烧 deadline；③fs_largeproject_smoke 调用面透传 UiPortWaitSeconds（≥90s，TIM-SMOKE 冷启动同族教训）。文件域扩展授权：run_free_state_d1_smoke.ps1（原卡域外，本裁定授权入腿）。
- **兜底**：若 D1 锚定腿仍不能确定性 settle（settle 本身不稳定），回决策侧按案②收口（configure/status+nil-hook+单测背书，E1 缺失如实登记）——不在执行侧自行降口径。
- E1 数据提取仍为终验前置。卡保持 doing；修订腿派发**待用户确认**。
- 阶段一合入说明（5e9d2f65）：impl 64b3ccfd 已于 2026-10-16 晚窗阶段一采信（95 包双跑+钉死项亲读在卡面）；本次合入目的=§12「领取重叠文件域新卡前先落 commit」——K2(128) 文件域含 chat/server.go 等，不合入将形成跨分支叠层。合入树组合态（+PLUGGRABBER 词表，文件集不相交）全量测试本轮补跑背书（结果见卡面终验节追加）。

## 三、GENSHELL-PCA-1（99，Mac 线）——**PC 主管复核留档：复核通过**；最终验收=Mac 决策侧先行裁定（裁定件未见，缺口如实登记）

- PC 复核亲做：
  1. diff 亲读（a198e1c3，6 文件 +535）：native_seed.go——typed 家族 fail-closed（V2CoverageAxes 非空即拒）、幂等短路（同 subjectKey+family+指纹 promoted 零写入）、指纹绑定（bundle 变=新签发走 store 既有 supersede）、签发走普通零覆盖 v2 语义（PromoteCurrent+IssueSpecV2）；registry.go——NativeSeed 仅限词表空集家族+字段完整性注册校验、List 防御性拷贝；chat/server.go seedNativeProcessorAdmissions——仅 Start() 进程级一次（测试/嵌入式构造不触发）、registry/store 不可用 WARN 不阻断启动；三道现行门零触碰亲核（diff 无 processor_certification_entry.go/authorizeCertificationTokenLoad/validateIssueSpecV2 面）。
  2. PC 侧独立复跑：复核 worktree @ a198e1c3 `go test ./... -count=1`=**95 包 0 FAIL**（PC 未部署 GenShell.vst3 场景=seed WARN 跳过路径兼容性一并覆盖）。
  3. Mac 真栈烟测工件亲读（genshell_pca_mac_20261016-231931/pca_report.json）：**12 ok/0 red verdict=all_green**——A seed（store promoted+指纹+identifier 三匹配+log seeded=true）/B agent 路径装载放行（agent_action_id 留痕）/C 同进程 revoke→pca_load_gate 拒（错误串逐字）/D UI 路径回归（直接内核命令 ok）/E 重启 re-seed 新记录+装载恢复（新 ID≠revoked ID）。隔离纪律（store/工程/VSP 全指 run 目录）声明在案。
- 边界采信：revoke=会话级语义（重启 re-seed 恢复）为 2026-10-16 用户裁定既定行为；identifier 由 GenShell 固定 FUID 派生（三轮实证恒定），FUID 变更须同步 registry 声明行；PC 部署后同声明自动生效。
- **合入归 Mac 线自理**（前腿 4d779236 亦未入 main，Mac 决策侧裁定后一并处理）；PC 不代合。
