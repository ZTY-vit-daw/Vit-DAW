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
