# FIXTURE-WE-HYGIENE-1：smoke fixture 波形包络遗留卫生（293 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 早窗（依据=[293 取证报告](../../reports/2026-11-07-we-mismatch-forensics.md)+[293 裁定](../done/2026-11-07-WE-MISMATCH-FORENSICS-1.md)——`semantic_processor_project_smoke_v1_80085263a651cf20` fixture 携带 2026-09-05 strip_silence smoke 遗留：`.vit_agent/vitproj_b3f28381…/mixboard/mixboard_feature_snapshot.json` WE 槽 `_1055` 行+twe 末行 (1007,1055)+`.vit_derived/.../acoustic_package_status.json` (1007,1055) 声学包，与 9/6 工程态（数字 id 全集 1001-1036 无 1055）**内在不一致**；读侧桥脱落时裸槽浮出→D1 waveform 面 settle 相位诚实翻转（设计内披露，物化链无罪））
- 派发确认：已确认（主管发卡）；验收负责人：GLM 主管决策流
- 池序 294；目标仓库=主仓 fixture（smoke 工件域）；P2 / 0.5 天 / 依赖=293 工件在库；L1-L2 / PC；零栈

## 目标

1. **卫生处置**：剔除/重置 fixture 的 9/5 遗留面（WE 槽 `_1055` 行、twe (1007,1055) 行、(1007,1055) 声学包条目），使 fixture 与其工程态自洽；**先复制到独立工作区操作+验证，再按权威 fixture 更新流程换入**（§10 防污染纪律：不直接改权威 fixture，前后核对哈希）；
2. 验证：卫生后的 fixture 在 293 同型读路径下 WE 视图稳定 ready（无裸槽浮出路径）——离线验证即可（293 工件轨迹复放/读侧函数离线跑）；如需一次真栈回归（266 D1 门组 settle 相位波形稳定）记候栈；
3. 顺带盘点：同型 fixture 是否还有其他日期遗留面（只记录）。

## 约束

- 文件域=fixture 副本工作区+报告；**源码零改动**；265/266/293 run 工件只读；
- sealed fixture 纪律：期望值/轨名不硬编码进代码或日志。

## 停止条件

剔除操作破坏 fixture 其他面板一致性（如 twe 规范行/声学包其他条目引用断裂）→停下取证上交，转重建流程评估。

## 领取/回执/验收

- 领取：2026-11-07 / origin/main=`66313d86` / owner=GLM-5.3（ZCode 执行子代理，PC） / 分支 `port/fixture-we-hygiene-1`（报告）+ `coord/294-claim`（卡状态） / worktree `D:/Vit_DAW_wt_294hyg`（实现）+ `D:/Vit_DAW_wt_294coord`（协调）/ 领取基线：HEAD=`66313d86`，status/diff --stat 均空（新 worktree 自 origin/main 检出）
- 回执（2026-11-07，执行侧自验完成待验收）：
  - **fixture 定位**：`D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20`（temp/ gitignore 域，不在版本控制；265/266/AF-D1 run 工件 `public_source_project` 亲证此为权威位）。`fixture_manifest.json` 只覆盖 cases stems（54 文件 sha256），不含 `.vit_agent`/`.vit_derived` 运行态——卫生编辑不触 manifest 面，换入后 54 stems 哈希复验全过。
  - **剔除清单（卡面三处，副本工作区外科+deep-equal 双验）**：①WE 槽 `kernel_prepared_waveform_envelope_1055`（t1007/c1055，9/5 telemetry）整槽；②twe `[18]` (1007,1055)（19→18 行，三代×6 规范行完整）；③声学包 `packages[6]` (1007,1055)（7→6 包）。**同族发现只记录不扩剔**：band_energy/loudness/stereo 三数组各含 `[18]` (1007,1055) 行（`kernel_l3_offline_analyzer`，15:59:32.766/.767+08 与声学包同源同刻）——不在卡面枚举且无读面暴露（数组行被归一化静默过滤），全文在工件 `removed_elements/siblings_record_only_1007_1055.json`，**交验收裁定是否补剔**。
  - **哈希账**：mixboard `5c0c97d5…`→`be05aafd…`；acoustic `e43d159c…`→`22416df0…`；换入三重门=复制时哈希=换入前重核+换入后字节 cmp 逐位一致+manifest 复验；前态备份与被剔元素 JSON 全文在 `coord/runs/FIXTURE-WE-HYGIENE-1/`（可逆）。
  - **离线验证**：`go test -overlay …/overlay.json ./internal/mixboard -run TestWEHygieneReadPathScenarios -count=1 -v` → **exit 0**（源码树零接触，overlay 注入；worktree 全程 clean）。四情景：v1 控制（卫生前+lr 原位）=ready `_1011` 复现 265 ground truth；v2 控制（卫生前+lr 换写 266 真实 probe_1027）=missing+`project_material_identity_mismatch` 于 `_1055` 复现 266 obs_171506 翻转（rig 保真度成立）；v3（卫生后+lr 原位）=ready `_1011` 稳态保持；v4（卫生后+lr 换写）=missing 无任何 _1055 身份——**裸槽浮出路径消灭**。v3/v4 在权威位换入后字节上复跑同过。工程态输入=265 run 工件捕获的 kernel get_project_state（只读复用）。
  - **盘点结论**：本 fixture 其余 1055 全在历史层（observations×4+l3_feature_log×18+.vit_history 会话态×2；`spv1_p01.vit` 9/6 工程态零 1055；两 `.bak` 各 1 处合法历史）——按只读纪律未动；同型 `…_02be6193fbc17bed` 状态层 clip id 全在规范 1001-1036 内，**零同型遗留**。
  - **报告与工件**：报告 [coord/reports/2026-11-07-fixture-we-hygiene.md](../../reports/2026-11-07-fixture-we-hygiene.md)；工件 `coord/runs/FIXTURE-WE-HYGIENE-1/`——均在分支 **port/fixture-we-hygiene-1** commit `87a8ec25`（基线 66313d86，12 文件，源码零改动红线亲证）。
  - **端测边界声明（零栈）**：验证边界=读侧函数离线复放（双控制精确复现 265/266 ground truth）+结构/哈希/manifest 静态核验；未跑真实栈回归——266 D1 门 A 回归门维持 274/293 backlog 记法，随后续真栈 run（296 D1D3-REGRESS-STACK-1）在卫生后 fixture 上验证。零栈零端口占用，无栈登记需求（卡面零栈口径）。
  - 过程纪律自述：领取推送一次被拒（origin/main 前进至 6f2f1ec5，277 栈登记），按 §2.1.3 自新 main 建 `coord/294-claim-reapply` 重新应用后推成（`8fb5e0e4`）；未用任何 reset/force 类命令。
- 验收：（主管回填）
