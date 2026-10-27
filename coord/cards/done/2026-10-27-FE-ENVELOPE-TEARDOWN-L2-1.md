# FE-ENVELOPE-TEARDOWN-L2-1：旧面拆除+第二层接入收口（重设计链卡 4）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=GUI 重设计裁定链；**依赖=203 合入**）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 204；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P2 / 1 天
- 模型分级：L1 / PC 执行会话
- 并行与资源：真栈腿错峰同前。

## 目标

1. **特写窗包络模式拆除**：envelope_dock_adapter.gd/.tscn+envelope_closeup_canvas.gd 删除；viewport_registry envelope 条目撤除；vit_dock_root vit_open_envelope_detail/旧口停用存根撤除；timeline_dock_adapter ⤢ 转发撤除；200/194/195 探针退役归档——特写窗保留给 3D 波形/频域（F1=A 宿主结论作废在卷）。拆除面断言：视窗清单不含 envelope+全树零残留引用。
2. **机架徽章（191）处置**：默认随旧面拆除（其数据面=外部插件行保留无害，仅 UI 撤）——如用户留用则改挂新选择器 ●N（上裁点）。
3. **发送组接入**：先探测腿（196 模式）——发送量 automation 寻址性（send 通道电平是否可寻址/有曲线挂载）；可寻址→选择器发送组启用；不可寻址→占位禁用+如实披露（不硬造）。
4. **插件参数组确认接入**：外部插件行（190/201 面）在选择器插件分组内在场（202 已建，本卡收口断言+端到端）。
5. **探针全家收口**：新旧探针清单化（退役/改写/保留），全家回归一轮+真栈腿两轮（端到端：装载插件→音量/声相/插件参数三组各画一条→内核逐点对照→undo）。

## 文件域

`vit_dock/**`（拆除面）+`app/tracks/track_scene/**` 选择器邻域（发送组接入）+探针；不触 `app/rack/**` 生产面（徽章 UI 件在 app/rack 则该单文件例外入域并在卡面标注）。越域即停。

## 验收标准

拆除面断言+选择器三组探针+探针全家回归+真栈两轮全绿+工件入 coord/runs/FE-ENVELOPE-TEARDOWN-L2-1/；端测边界声明（最终用户手测节点=本链合入后）。

## 停止条件

- 发送量寻址探测发现需内核扩面→证据上交另裁（不在本卡扩内核）；
- 拆除发现 vit_dock 之外残留引用（结构漂移）→锚点上交。

## 领取

- 2026-10-27 17:57 +0800 / origin/main=39acec166d8df5752f6942ab75aef46fa38001f4 / owner=GLM-5.3 L2 PC 执行侧 ZCode 会话（ZCode，本卡会话，用户口令指定领取）/ 分支=port/fe-envelope-teardown-l2-1（前端仓 worktree D:/Godot/project/vit-daw-frontend-envtd1，基线=前端 main 479d01c）+ 主仓协调 worktree D:/Vit_DAW_wt_envtd1_coord（coord/envtd1-claim）/ 领取提交=4207650f。领取时核对：卡在 todo+派发已确认（用户裁定链）、依赖 203 已验收合入（主仓 39acec16 accept+前端 479d01c merge 双亲核）、doing 仅 GEN-SUNO-WEB-DESIGN-1（coord/runs/ 设计卡零生产代码，文件域零冲突）、真栈空闲（最新释放=203@17:46 亲读）。

## 回执

- 2026-10-27 晚窗执行侧自验完成，交主管验收（详 [runs/FE-ENVELOPE-TEARDOWN-L2-1/envtd_20261027_1930/RESULTS.md](../../runs/FE-ENVELOPE-TEARDOWN-L2-1/envtd_20261027_1930/RESULTS.md)）——
- **commit**：前端实现=**3140681** 随 `port/fe-envelope-teardown-l2-1` 已推远端；真栈工件=**b0508523** 随主仓 `port/fe-envelope-teardown-l2-1-stack` 已推远端（§6.1 分支名核对入口）；栈登记 cf5d74af/释放 2aee5e0d 已推 main。
- **拆除面（①）**：envelope_dock_adapter.gd/.uid/.tscn+envelope_closeup_canvas.gd/.uid 删除；registry envelope 条目撤（六类生产视图在册）；vit_dock_root 两入口（vit_open_envelope_detail+vit_open_envelope_in_stage_slot 存根）撤+_open_or_focus_detail_window view_type 参数化还原（切换面机制保留、清单恰 `["detail"]`——特写窗=3D 波形/频域宿主）；timeline_dock_adapter ⤢ 三件撤（递归绑定/处理函数/信号名）。**拆除面断言双面绿**：registry 无 envelope+两入口 has_method 均否（真栈 T0）+全树 7 禁词零残留（离线 E 段，.git/.godot 外全文本面）。车道文件 4 处对已删文件的注释引用改写为卡号出处（非结构漂移，无 vit_dock 外活引用）。
- **机架徽章（②）**：默认撤=Vit_Graph_Rack.gd 徽章行构造/刷新/两调用点+ rack_node_display_rules.gd plugin_envelope_badge_state 纯函数；实例断言=子类探针机架真实装配插件卡后零 PluginEnvelopeBadgeRow。**文件域例外标注**：卡面原注「app/rack 单文件例外」，实际徽章件横跨 **两文件**（Vit_Graph_Rack.gd UI 件+rack_node_display_rules.gd 徽章专属纯函数，后者零他消费故随之退役）——按卡面例外意图（徽章 UI 件所在 app/rack 入域）执行并在此标注。数据面=envelope_point_count 外部插件行保留（车道 ●N 消费，无改动）。
- **发送组（③）——探测腿完成，停止条件分支触发，证据上交另裁**：真栈 S0 两轮实证：AuxSend 插件在 get_project_state 可枚举（id=ItemID）；**automation 面可寻址**（param_id="send level"=引擎源 tracktion_AuxSend.cpp:22 锚点：get_curve ok 空披露+set_points 读回往返 ok；其余 4 候选逐字 Parameter not found）；**描述符面拒绝内置插件**（get_plugin_parameters → "Plugin parameters are only available for external plugins"，201 B 案边界）。选择器启用需描述符面（值域/显示名/●N 数据源）=**需内核扩面**（201 B 案先例=描述符面扩 auxsend 行）→ 按停止条件不在本卡扩内核：发送组保持占位禁用（E2b 断言=当前实现如实）+不硬造参数知识。建议另裁：内核描述符面扩轨级 volume/pan 同款到 auxsend 插件行后开实现卡。
- **插件参数组（④）收口**：真栈 E2 选择器三组（轨道恰 volume/pan=201 实面镜像+发送占位禁用+插件组=实装载 VST 实参数）+E5 插件参数车道经选择器条目绑定→画一条→恰一笔+内核逐点对照（190/201/202 面端到端）。
- **探针全家（⑤）**：处置清单 RESULTS §二（**退役 19 文件**：194/195/200/199/191 五对+189/192 栈探针对——删除性工程先例直接删除；**保留 21 离线+3 栈探针对**：202 已改写的 lane 三件+现行 autolane/datachan+域外全家；**新增 2**：probe_envelope_teardown 50/50+stack_probe_envelope_teardown 44/44）。全家回归一轮 21 件全绿（车道族 274 检与 203 收据逐件同数；shrink 87/119/342 与基线主检出亲跑同数——旧收据 88/120/343 为 168 时代计数）。
- **真栈两轮**：ps1 最小腿 exit 0 + stack_probe_envelope_teardown **44/44×2 连续两轮 exit 0 零修正轮**（E1 装载链真发现→E3/E4/E5 三组各画一条恰一笔+内核逐点一致→E6 Ctrl+Z undo LIFO：插件参数回 EMPTY+内核空+音量/声相保持）。§2.2 三纪律+188 守卫+VIT_PROJECT_XML 隔离亲证（287956e7→ce0f6543）+错峰（登记前 fetch 亲读占用空）全在卷；拆栈 18:52:03 复核全零。
- **端测边界声明**：渲染面（拆除后特写窗布局零漂移目检）未过浏览器级断言（E2E-WEBUI-1 未就位）；车道三组绘制=headless 生产 _gui_input 注入；**用户手测节点=本链合入后**（特写窗三开路径目检+三组绘制手感+undo 手感）。
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-27；rulings/2026-10-27-FE-ENVELOPE-TEARDOWN-L2-1-pass.md；前端合入 f52b78f+工件 stack 分支。三披露裁定=两文件例外采信/send 停止条件正确触发另裁登记/shrink 计数差异采信；重设计链 201→204 闭环，进入用户手测节点）
