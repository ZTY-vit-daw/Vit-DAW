# FE-LANE-DESCRIPTOR-BULKREPLY-1：巨参数插件描述符回复超 UDP 上限——车道描述符回执断供（手测节第三轮缺陷卡 III-B）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=用户手测：Pro-Q 3 参数永远「加载中」/新包络线无可选参数；发卡侧取证在卷）
- 派发确认：待用户确认（手测节当场折卡惯例）
- 验收负责人：GLM 主管决策流
- 池序 209；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（agent 面若须动 Go→超域上交裁定）；P1 / 0.5 天 / 依赖=无（基线=前端 main 36cda0e）
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰手测节

## 根因（发卡侧亲核，2026-10-28 用户 21:18 会话取证）

1. **内核侧完好**：用户装载 Pro-Q 3（plugin id=1040，挂轨 1007）→lane 发 get_plugin_parameters（rid=env_lane_params_1040）→内核回复 status=ok、**348 参数行、约 971KB**（spill 文件 BridgeReplies/20261028_211804_*_env_lane_params_1040.json 亲读：行字段 23 个含 display_probe/binding_target 等）。
2. **断点=回复体积超 UDP 回传上限**：agent bridge 对超大 control reply 执行 spill-to-file（agent 日志 21:18:04-05 四次 spill 留痕，同 rid 重发 8 次=前端在重试）——**回复落 BridgeReplies 文件、不经 UDP 回传**→lane 描述符回执永 pending→「加载中」永驻+selector 无可选行（selector 只列描述符 loaded 行）。
3. **探针盲区=体积面**：207/208 探针装载的测试插件参数行少（回复在 UDP 上限内）→描述符链绿；**Pro-Q 3=首个暴露体积上限的真实巨参数插件**（348 行×23 字段）。装配面（§5.1）与时序面口径均不覆盖体积差集。
4. 伴生事实（III-A，另线处置非本卡）：用户栈内核 10-19 构建缺 190/201 内核面（envelope_point_count+内建 volume 插件 volume/pan 行）——主管已重建主检出内核（用户 volume 描述符 pending 的根因）；本卡只修体积面。

## 目标

1. **大回复通道**：lane 描述符请求面对超 UDP 上限回复可承载——候选形态（勘察定案，最小面优先）：①lane 描述符改走可承载大回复的通道（agent HTTP 面若已有 get_plugin_parameters 端点则切换/并行）；②agent 对 env_lane_params_* rid 的回复裁剪至 lane 最小字段集（若裁剪后入 UDP 上限）；③spill 回执文件轮询/查询面（agent 已有接口则用）。**内核零改动优先**。
2. **重试收敛**：现同 rid 重发 8 次=无效重试风暴——通道修复后自然消除；若保留重试须有退避。
3. 手测闭环：Pro-Q 3 描述符加载完成（selector 出现插件参数行）+默认车道正常（与 III-A 内核重建联验）。

## 探针义务（§5.1+时序面+体积面增补）

- **体积面断言必入**：描述符链断言须含巨参数形态（≥300 参数行或回复>UDP 上限）——优先用真实大插件（用户环境 FabFilter Pro-Q 3 在场）；无则最小构造并披露。
- §5.1 装配面+时序面延续；禁注入延续；既有全家回归零回退。

## 文件域

`app/tracks/track_scene/track_envelope_lane_2d.gd`（描述符请求/回执面）+`app/kernel/clients/*`（通道客户端）+探针文件。**动 agent Go 面（agent/internal bridge spill/HTTP）→上交扩域裁定**；动内核→上交。

## 随卡观察项（非本卡文件域）

- **删除效果器 GUI 无反馈**（数据面已删+repo 更新+lane 目录跟随，机架 UI 节点不移除）——内核重建重测后若仍在→独立折卡（机架 UI 刷新链），本卡不动机架面。

## 停止条件

- 勘察发现 agent spill 面已有既定大回复回传设计（文件轮询 API/推送）→上交锚点接用；
- 修复须动 agent Go 或内核（字段过滤等）→上交扩域裁定；
- UDP 上限阈值与 spill 机制形态与假设不符→上交锚点。
- 领取：2026-10-28 21:28 / origin/main=d610500e / owner=GLM-5.3 ZCode 执行会话（PC，本会话，用户当场指派=派发确认）/ 分支=port/fe-lane-descriptor-bulkreply-1 / worktree=D:/Godot/project/vit-daw-frontend-bulkreply1（基线=前端 main 36cda0e）/ 领取提交=3f76183b
- 回执（2026-10-28 22:2x，doing→done 交主管验收）：
  - **勘察结论（停止条件①+③命中，上交锚点）**：①agent 桥**已有既定大回复回传设计**——`agent/internal/bridge/bridge.go:195-237`：control 回复>32KB（maxDirectUDPReplyBytes）即 spill 落 BridgeReplies 文件**并仍经 UDP 回发小信封** `{transport:"file_reply",reply_file,reply_bytes,request_id}`（用户会话 agent_last.log 四次 spill 无 UDP 发送失败 Warn=信封确实发出）；②前端**已有信封消费面**——`vit_ipc_client.gd:46-86` 信封→FileAccess 读盘→JSON.parse_string；③卡面「回复落文件不经 UDP 回传→lane 永 pending」前提**不成立**（spill 机制形态与假设不符=停止条件③）。三面证据：代码亲读+离线（用户会话真实 971KB/348 行 spill 文件过真实 autoload 全链 17.8ms 全绿）+真栈（正式轮 P0 await 面与 P4 lane no-wait 面[=用户症状路径]均 loaded/348 行/selector 348 可选）。
  - **零修复裁定**：三候选（HTTP 切换/agent 裁剪/spill 轮询）均不必要——既定信封通道已在且工作；前端/agent/内核生产代码零改动。候选①核实：agent HTTP 面（127.0.0.1:7878，chat.Routes）无 get_plugin_parameters 端点；候选③核实：无 spill 查询端点——但信封推送设计已在，两者均无需。
  - **重试收敛**：真栈 P4 窗口 lane 请求 spill 计数 before=0 after=1=1 次请求即收敛；P5d 目录重拉×3 描述符全部回 loaded（幂等）。卡面「同 rid 重发 8 次」=装载期触发链重拉累计（通道断供旧形态无法收敛），通道工作后自然消除——真栈坐实。
  - **用户症状在现行基线不可复现**：DIAG r3（基线 36cda0e+21:26 重建内核）缺陷签名断言 FAIL（state=loaded rows=348 selector=348）=机制工作。21:18 会话差异面=旧内核（缺 III-A 190/201，21:26 已重建）+全 GUI 并发形态；残余不确定上交主管：①用户手测节在新栈重验 Pro-Q 3 描述符+默认车道（=卡面手测闭环收口动作，真栈自动等价 P4/P5 已绿）；②若复现→全 GUI 并发面取证卡；③agent 域伴生瑕疵上交裁定：spill 文件名时间戳纳秒位恒 0（bridge.go:213-216 格式串字面零）→同秒同 cmd+rid 文件名碰撞互相覆盖（用户会话 4 次 env_lane spill 仅留 2 文件亲证；内容相同无读损坏，纯留痕面）。
  - **实现 commit**：前端 port/fe-lane-descriptor-bulkreply-1@4911072（探针三件，+758 行，零生产改动）；工件 port/fe-lane-descriptor-bulkreply-1-stack@13816ad8（37 文件）；报告=coord/runs/FE-LANE-DESCRIPTOR-BULKREPLY-1/bulkreply_20261028_1/RESULTS.md。
  - **探针与真栈 run**：run ID=bulkreply_20261028_1；正式轮 R1/R2 **exit 0×2 连续**（26 PASS+1INFO 同形）；DIAG r3 exit 1=缺陷签名未坐实的如实形态（P0-P3 九项全绿）；离线全家 24/24+新增 probe_file_reply_volume_face 1/1=**25/25 exit 0**。异常如实分类：探针 v1 语法错空转超时（124 kill，未触栈）+worktree 缺 .godot 类缓存（--import 后复常，import 末段 Godot 自身 segfault 在缓存生成后无碍）——均探针/环境面非被测面；无环境中断归因于功能。
  - **端测边界声明**：本腿=Godot headless 生产树断言+agent 传输面，**不含 webui 渲染面与全 GUI 用户旅程面**；手测闭环（用户 GUI 侧 Pro-Q 3 描述符加载完成+默认车道正常）待用户手测节——真栈探针 P4/P5 已覆盖自动等价（含 III-A 联验）。
  - 栈纪律：登记 df74f648（21:55:34）→即时复核 21:55:52/21:56:05（<60s 窗口）→PREFLIGHT_CLEAN→拆栈 22:16:00 三 pid 路径亲核+进程/端口全零；释放登记见本批提交。

- 验收：（**主管整卡 pass 2026-10-28 晚，GLM 主管决策流；零修复裁定接受**）——①发卡前提推翻证据三处锚点主管亲读：bridge.go:195-237（>32KB spill+`{transport:file_reply,reply_file,reply_bytes}` 小信封 UDP 回发——发卡侧此前只读 agent 日志 spill 行未追信封，前提「不经 UDP 回传」错误记档）；vit_ipc_client.gd:46-86（信封→FileAccess 读盘→解析，失败显式 error 形态）；bridge.go:213-216 纳秒位瑕疵（Go Format "000000000"=字面九零，同秒同 cmd+rid 文件名碰撞坐实）。②SUMMARY 亲读：正式轮 26/26×2（真实 Pro-Q 348 行/1MB 过信封通道=体积面断言义务兑现）+DIAG r3 三败=缺陷签名未坐实的如实形态（不可复现证据本身）+离线全家 25/25。③零生产改动亲核（diff stat=探针七文件 +758）。④上交两件处置：ⓐ用户手测新栈重验=手测闭环收口动作（真栈自动等价 P4/P5 已绿）；ⓑagent spill 文件名碰撞瑕疵=**折 P2 小卡 FE-BRIDGE-SPILL-NS-COLLIDE-1（agent 域，低危留痕面不阻主线）**。合入：前端 main=84396f1（探针 merge）；工件=主仓 merge（bulkreply_20261028_1 全量）。**手测节第三轮重验第四次解锁（新内核 21:26+现行前端全量面）**。
