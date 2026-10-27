# FE-AUTOLANE-REBUILD-1：包络车道骨架重做——轨道头开关+叠多车道+统一选择器+单击建点（重设计链卡 2）

- 发卡：GLM 主管决策流 / 2026-10-27（依据=GUI 重设计裁定链+预览 v1；**依赖=201（M3-TRACK-PARAM-DESCRIPTOR-1）合入后才可领取**——选择器消费其轨级行）
- 派发确认：已确认（用户裁定链）
- 验收负责人：GLM 主管决策流
- 池序 202；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1 / 1-1.5 天
- 模型分级：**L2 / PC 执行会话**（车道形态整体重写）
- 并行与资源：真栈腿与手测节/其他真栈卡错峰（§2.2 增补③）。

## 目标（旧车道形态在本卡文件域内**重写作废**：三态档/在场性扫描/粘性/198 MenuButton 整体拆除）

1. **轨道头包络开关**：一键展开/收起（两态即可——三态语义随「在场性驱动」废弃而废；挂载位沿用 198 槽位 MSR 行，master/folder 隐藏维持）。
2. **叠多车道容器**：展开后轨道下方可叠 N 条车道（默认展开=音量一条）；每车道头=参数选择器+关闭钮；「＋添加包络车道」入口；行高联动按 189 C 案先例（固定高度叠加条，波形 lane 零行为改变）。
3. **参数选择器=统一描述符清单**：分组「轨道（音量/声相——201 新行）→ 发送（本卡占位禁用态，接入归 204）→ 插件（机架目录实际装载插件的实际参数，get_plugin_parameters 同源）」；列表标 ●N=该参数已有包络（count>0）；选择后车道头标签=组名·参数名，值域标签随描述符单位。
4. **空车道单击即建首点**：EMPTY 态单击画布建点（按下即建即拖，199 手势语义迁移）；空态呈现「（空）」灰条。
5. **默认收起+展开常驻**：开关关=全部车道隐藏；展开后常驻可画（无自动收起）。
6. **只读呈现面**：曲线点/折线/c 曲率采样渲染（195 几何迁移——只呈现，编辑归 203）、播放头回退链复用。

## 文件域

`app/tracks/track_scene/**`（track_envelope_lane_2d.gd 重写+Track_Row_Controller.gd 挂载/开关/198 控件替换）+探针新文件；不触 `vit_dock/**`（特写窗拆除归 204）、`app/rack/**`、`app/kernel/clients/rack_client.gd`。越域即停。

## 验收标准

独立探针全绿（开关/叠车道/选择器分组与 ●N/默认音量车道/空车道建点/回退=开关关零残留）+旧车道断言面处置清单（lane_ui/lane_auto/closeup/curve 中车道相关断言随重写迁移，逐条列明**非弱化**）+真栈腿两轮（装载 VST→展开→音量/声相车道在场→空车道建点→内核 get_curve 读回一致）+工件入 coord/runs/FE-AUTOLANE-REBUILD-1/；端测边界声明（渲染/手感留手测面）。

## 停止条件

- 201 轨级行未按卡面到达前端消费链→锚点上交；
- 多车道叠加与轨道行高/滚动/让路链冲突超预期→上交；
- 198 控件替换点结构漂移→锚点上交。

- 领取：（2026-10-27 15:24 +0800 / origin/main=1546d9c348e05c2d76a6dca6aa129b9076ea2230 / GLM-5.3 L2 PC 执行侧 ZCode 会话（本卡会话，用户口令指定开工）/ 实现分支=port/fe-autolane-rebuild-1（前端仓 D:\Godot\project\vit-daw-frontend 独立 worktree，基线 main=a0bc0c4）+ 协调 worktree D:\Vit_DAW-coord-fe-autolane1（分支 coord/fe-autolane-rebuild-1-claim）/ 领取提交=本提交。领取时核对：卡在 todo+派发已确认（2026-10-27 用户裁定链）；依赖 201（M3-TRACK-PARAM-DESCRIPTOR-1）已合入（origin/main=1546d9c3 accept 提交，202 解锁）；doing 唯一卡 GEN-SUNO-WEB-DESIGN-1 文件域=coord/runs/GEN-SUNO-WEB-DESIGN-1/** 零冲突；真栈腿按 §2.2 登记+错峰（与手测节/其他真栈卡错峰，卡面并行条款））
- 回执：（**执行侧自验完成 2026-10-27 17:00 前后，交主管验收**。实现=前端仓 `port/fe-autolane-rebuild-1` @ **b2f52fd**（已推远端；cebc2e8 实现+16a7d5c 栈探针+b2f52fd R1 缺陷修复）；工件=主仓 `port/fe-autolane-rebuild-1-stack`（run=**coord/runs/FE-AUTOLANE-REBUILD-1/autolane_20261027_1650/**，RESULTS.md 回执）。
  **离线探针全绿 10 件 473 检 exit 0**：新 probe_autolane_rebuild **73/73**（开关/叠车道/选择器分组与 ●N/默认音量车道/空车道建点/收起零残留/195 几何迁移对照）+迁移四件 lane_ui 36/lane_auto 25/lane_toggle 27/closeup 81（摘车道⤢段）+既有六件零回退（curve 55/first_point 41/detail_home 52/badge 33/datachan 50+lane 系列）。**旧车道断言面处置清单逐条非弱化**=RESULTS.md §三（废置项全部有 2026-10-27 用户裁定链依据+替代守卫：三态/在场性/粘性→防复活守卫 E1-E3；⤢→窗侧入口 detail_home R 段在位）。
  **真栈腿两轮全绿**：§2.2 登记 541a5543→ps1 最小腿 exit 0（188 守卫+PREFLIGHT_CLEAN）→stack_probe_autolane_rebuild **23/23×2 连续两轮**（R1 16:39+R2 16:40）：装载 VST（真发现）→真行装配展开→默认音量车道生产链落定→轨道组恰 volume/pan 两行（201 实面）→声相车道在场→空车道建点恰一笔+回显门+**内核 get_curve 读回一致**→●N 新鲜度实内核 count。R1 首试 22/23=L4 产品缺陷（自写回执 WRITE 分支早退致 ●N 重拉不触发）→修复 b2f52fd+F7b 回归断言→双轮全绿（缺陷轮留证 run 目录）。隔离三面亲证（run 副本内核写入/stack worktree git 干净/主检出零触碰）；拆栈 16:43 复核全零；释放记录随本批推送。
  **端测边界声明**：渲染面（叠车道布局/波形叠加观感/弹层观感）未过浏览器级 DOM 断言（E2E-WEBUI-1 未就位）；手感面（按下即建即拖/开关操作）留用户手测（裁定链：实施后用户手测节点终验）；探针断言到内部状态+几何采样值+内核读回一致。停止条件三支均未触发（201 轨级行已到前端消费链实证 L2；多车道叠加走 189 C 案叠加条零行高冲突；198 槽位结构稳定替换实证）。）
- 验收：（裁定文件 / 验收 commit）
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-27；rulings/2026-10-27-FE-AUTOLANE-REBUILD-1-pass.md；前端合入 f5b7329+工件 stack 分支。断言处置清单逐条非弱化亲核；R1 缺陷修复采信；203 解锁）
