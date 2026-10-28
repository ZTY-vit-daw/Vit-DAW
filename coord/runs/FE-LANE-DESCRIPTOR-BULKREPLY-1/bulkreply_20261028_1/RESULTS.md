# FE-LANE-DESCRIPTOR-BULKREPLY-1（209）真栈腿回执 — bulkreply_20261028_1

- owner：ZCode GLM-5.3 PC 执行会话（209 卡会话）；2026-10-28 晚窗
- run：`coord/runs/FE-LANE-DESCRIPTOR-BULKREPLY-1/bulkreply_20261028_1`（本目录，随 `port/fe-lane-descriptor-bulkreply-1-stack` 入库）
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-bulkreply1@port/fe-lane-descriptor-bulkreply-1` @ **36cda0e（=领取基线=前端 main，零代码改动——本卡结论=无需前端修复）**
- kernel：**主检出重建二进制只读复用** `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（2026-10-28 21:26 主管重建，含 III-A 190/201 面；本卡 VitApp/+agent/ 零改动）pid 16680，ps1 `-StartKernel` 自启
- agent：stack worktree `D:/Vit_DAW_wt_209stack` 构建（Go 零改动，`go build ./cmd/vitagent` exit 0）pid 13332
- vsphub：主仓 `D:/Vit_DAW/agent/bin/VspHub.exe` 只读复用 pid 12884 本腿手启
- VIT_PROJECT_XML：run 目录 git 干净版隔离副本（blob `287956e7` 基线=202-208 首腿同基线，hash-object 亲核 `287956e71e03532139b35e8f616fb283f4dca1ca`）

## 一、勘察结论（卡面根因叙事被证据推翻——停止条件①+③命中，上交锚点）

1. **agent spill 面已有既定大回复回传设计（停止条件①）**：`agent/internal/bridge/bridge.go:195-237`——control 回复 >`maxDirectUDPReplyBytes`（32KB，bridge.go:90）即 spill 落 `BridgeReplies/<ts>_<cmd>_<rid>.json` 并**仍经 UDP 回发小信封** `{status:"ok",transport:"file_reply",reply_file,reply_bytes,cmd,request_id}`（bridge.go:170 `conn.WriteToUDP`；用户会话 agent_last.log 21:18:04-05 四次 spill 全程无 UDP 发送失败 Warn=信封确实发出）。
2. **前端已有信封消费面**：`app/kernel/autoloads/vit_ipc_client.gd:46-86` `_parse_reply`/`_parse_file_reply`——信封→FileAccess 读盘→JSON.parse_string→回复体注入 `_transport/_reply_file/_reply_bytes`。**卡面「回复落 BridgeReplies 文件、不经 UDP 回传→lane 回执永 pending」的前提不成立**（spill 机制形态与假设不符=停止条件③）。
3. **三面亲证机制工作**：
   - 离线（勘察轮 `tools/probe_survey_file_reply_volume*`，本仓在卷）：用户会话真实 971KB/348 行 spill 文件过真实 autoload `_parse_reply` 全链=status ok+348 行+17.8ms；缺文件→`file_reply_open_failed` fail-closed 如实返回；
   - 真栈 await 面（探针 P0）：warm `get_plugin_parameters` 回复 transport=file_reply+reply_bytes>32KB+rows=348；
   - 真栈 lane no-wait 面（探针 P4，=用户症状路径 rid `env_lane_params_<pid>`）：descriptor_state=loaded+rows=348+selector 插件组 348 可选+2s 稳态+重拉 3/3 幂等。
4. **用户 21:18 症状在现行基线不可复现**：21:18 会话跑旧内核（缺 III-A 190/201 面，21:26 已重建）+全 GUI 形态；同回复体（旧内核 spill 文件亲读 status ok+parameters 348 行在场）+同前端代码（36cda0e）+同 agent 信封设计（file_reply 2026-06-03 起入库，用户 10-24 二进制已含）下，真栈探针 loaded/348 行全绿。**残余不确定**：全 GUI 并发/旧内核交织面无法排除——上交主管裁定（用户手测节在新栈重验=卡面手测闭环本身即收口动作）。
5. **伴生披露（agent 域，非本卡文件域）**：spill 文件名时间戳纳秒位恒 0（`bridge.go:213-216` `20060102_150405_000000000` 格式串字面零）——同秒同 cmd+rid 多次 spill 文件名碰撞互相覆盖（用户会话 4 次 env_lane spill 仅留 2 文件亲证）。内容相同无读损坏风险，纯计数/留痕面瑕疵；修复属 agent Go 域→上交裁定。
6. **重试收敛**：真栈 P4d 披露 lane 请求 spill 文件 before=0 after=1=**P4 窗口 1 次请求即收敛**（无重试风暴）；spill_evidence/ 留档四文件另证：DIAG r3 一份（pid 1048）+正式 R1 一份（pid 1055）+正式 R2 两份（pid 1062，P4 一份+P5d 目录重拉再请求一份=重拉合法重请求经通道再收敛，非风暴形态）；触发链重拉为目录刷新合法语义（207 起），P5d 重拉 3/3 全部回 loaded=幂等收敛。卡面「同 rid 重发 8 次」=装载期触发链重拉累计（通道断供时无法收敛的旧形态），通道工作后自然消除——真栈坐实。

## 二、实现（本卡=零代码修复；交付探针两件）

1. `tools/stack_probe_lane_descriptor_bulkreply.gd/.tscn`（真栈）：P0 体积面三断言（真实 FabFilter Pro-Q 3 发现+装载+warm await：rows≥300+transport=file_reply+reply_bytes>32KB）+P1 §5.1 装配面（VitDockRoot.tscn 实件零 repo 自建，无 `_REPOSITORY_SCRIPT`/repo `.new()`/`set_plugin_catalog`/`set_track_volume_plugin_id` 可 grep 亲证）+P2 生产喂食+P3 lane 生产消费+P4 体积面核心（no-wait 描述符链 loaded/rows/selector/2s 稳态/spill 桥侧证据）+P5 III-A 联验（轨道组恰 pan/volume+默认车道落定）+重拉幂等时序面+P6 清场；DIAG 模式（`VIT_PROBE_209_DIAG=1`）断言缺陷签名。
2. `tools/probe_file_reply_volume_face_scene.gd` + `.tscn`（离线，场景模式传输惰性——零 enqueue/零 send 可 grep 亲证）：自含合成 348 行×23 字段（含 display_probe/binding_target 嵌套）~214KB spill 文件过真实 autoload 解析面+缺文件 fail-closed+耗时披露。
3. 勘察轮取证件 `tools/probe_survey_file_reply_volume*`（离线，依赖用户会话真实 spill 文件绝对路径——一次性取证形态，非回归件）。

## 三、探针义务兑现

- **体积面断言必入（本卡新增义务）**：真栈真实 FabFilter Pro-Q 3（348 参数行/回复>32KB UDP 直发上限）——P0 await 面三断言+P4 lane no-wait 面 rows≥300+selector≥300；离线合成体积件固化回归网。
- §5.1 装配面延续：VitDockRoot.tscn 实件入场（P1b/P1c dock 子树亲证），零 repo 自建、零注入（205 口径）。
- 时序面延续：P4e 2s 稳态窗+P5d 目录重拉×3 描述符状态回 loaded（跨刷新时序点断言，208 口径）。
- 全家离线回归：24 件+新增 1 件=**25/25 全绿 exit 0**（offline_regression/ 目录 25 份 log）。

## 四、真栈结果（§8 声明：LLM=0 全确定性；成功=退出码 0；失败分类=assertion_fail/environment_interrupt/script_exception）

| 轮 | 命令形态 | 结果 | 退出码 |
|---|---|---|---|
| DIAG r2（探针 v1 解析错） | 探针场景 | 场景脚本 79 行 `% (...)` 解析错→空转 560s 超时 kill——**探针缺陷非被测面**；未触栈（场景未载入） | 124(kill) |
| DIAG r3（探针 v1.1） | `VIT_PROBE_209_DIAG=1` | 9 项有效断言：P0-P3 全绿+**P4a/P4b/P5d 缺陷签名断言 FAIL（state=loaded rows=348 selector=348 重拉 3/3=机制工作，缺陷不可复现）**——exit 1 为 DIAG 预期取反形态，即「缺陷签名未坐实」的如实披露 | 1 |
| 正式 R1（探针 v1.1） | 探针场景（修复态=基线即修复态） | **26 PASS+1 INFO 全绿**（P0 三断言+P4a-e+P5a-d 全过） | 0 |
| 正式 R2 | 同上复跑 | **26 PASS+1 INFO 全绿**（同形） | 0 |

探针迭代：v1 `% (...)` 元组语法错（GDScript 无元组，`% [...]` 修正）；新 worktree 首跑缺 `.godot` 类名缓存→P1b 装配失败（class_name 全解析失败）→全量 `--import` 后正常（208 同操作，import 末段 Godot 自身 segfault 在类缓存生成后、无碍探针）。两起均为探针/环境面，非被测面缺陷。

## 五、三纪律与隔离

- 登记 `df74f648` 推送 21:55:34 后 **21:55:52/21:56:05 即时复核**（远端亲读置顶+进程/端口全零，<60s 窗口）+driver `PREFLIGHT_CLEAN` 预检（ps1 内 188 守卫）；无 already listening（kernel/agent=ps1 自启；vsphub=手启 12884 前亲核 8787 空）；登记先于启动。
- VIT_PROJECT_XML 隔离：run 副本 `287956e7`→`7e17b996`（运行后内核写回被隔离副本承接=隔离生效）；stack worktree `VitApp/` 运行后 git 干净；主检出 `default_project.xml` 零触碰（会话起即有的 M 态为用户会话遗留，非本腿）。
- 拆栈=owner 手拆 vsphub 12884/agent 13332/kernel 16680（三 pid 路径逐一亲核=本腿所属）→**22:16:00 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/UDP 4444/4445 全零**。

## 六、披露

1. **零修复裁定依据**：卡面三候选（HTTP 切换/agent 裁剪/spill 轮询）均不必要——既定信封通道已在且工作（本回执三面证据）。动 agent Go 或内核的任何形态（含字段裁剪）违反卡面「内核零改动优先」与文件域约束，未实施。
2. **上交点（主管裁定）**：①用户手测节在新栈（21:26 内核+现行前端）重验 Pro-Q 3 描述符+默认车道（=卡面手测闭环收口动作；真栈自动等价 P4/P5 已绿）；②若用户症状复现→全 GUI 并发面取证卡（headless 生产树探针未复现的残余面）；③agent 域同秒 spill 文件名碰撞瑕疵（§一.5）是否折卡。
3. **DIAG 轮语义**：DIAG 断言取缺陷签名，缺陷不存在时 DIAG exit 1=「复现失败」的如实形态（非 flaky）；正式轮两连绿为验收面。
4. **端测边界**：本腿=Godot headless 生产树断言+agent 传输面，不含 webui 渲染面与全 GUI 用户旅程面；用户手测闭环待手测节。
5. **import segfault**：新 worktree `--import` 末段 Godot 4.6.1 自身 segfault（类缓存已生成后）；探针两轮全绿证明缓存可用；留痕非被测面。
6. **勘察轮探针依赖绝对路径**：`probe_survey_file_reply_volume` 引用用户会话 spill 文件绝对路径（一次性取证件）；回归件=`probe_file_reply_volume_face`（自含合成）。
