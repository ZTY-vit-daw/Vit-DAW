# M3-ENVELOPE-LANE-AUTO-1：包络车道三态自动档+自动展开腿（189 二段）
- 发卡：GLM 主管决策流 / 2026-10-26 晚（依据=189 验收裁定两段式——三态「自动」档+自动展开腿随 190 补验；190 已合入 main 87b032e2 ✓ 数据源就位；用户 2026-10-25 形态裁定「按需+手动混合」为此腿裁定源）
- 派发确认：已确认（189 验收裁定链+用户形态裁定）
- 验收负责人：GLM 主管决策流
- 池序 192；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=20873f8）；P3 / 0.5 天 / 依赖=190 合入 ✓（envelope_point_count 在场性字段）
- 模型分级：L1 / PC 执行会话
- 并行与资源：与 191 M3-ENVELOPE-RACK-BADGE-1（app/rack/）文件域不相交可并行；**真栈腿与 191 错峰**（§2.2 增补③——领取/登记前互查 PC-RUNTIME-STACK.md 与对方卡占用状态，同刻双登记禁止）；C4 勘察卡未开卡，无在飞冲突。

## 目标
1. **三态档位**：车道开关从两态（开/关）扩三态（关/手动/自动）——189 第一段已留口；手动档=现有行为；自动档=可见性由在场性驱动。
2. **自动展开腿**：描述符刷新链（rack_client.get_plugin_parameters 同源，189 车头参数选择器同链）拉取回执→任一参数 envelope_point_count>0→自动档下该轨车道自动展开；全部归零→自动收起；用户手动展开优先（手动档/手动展开不被自动收起，形态细节勘察定案）。
3. **覆盖面如实**：数据源=外部 rack VST 插件参数（190 覆盖面）；轨级参数（volume/pan）不在回执面=已知缺口（主管 2026-10-26 已裁定另裁数据源）——轨级参数不触发自动展开如实呈现，不硬造。
4. **回退**：默认档=现两态行为（自动档需用户显式选）；旧工程/旧状态零行为变化（AGENTS §11 缺省语义）。

## 文件域
`app/timeline/`+`app/tracks/track_scene/`（track_envelope_lane_2d.gd 三态档+Track_Row_Controller.gd 档位邻域）+探针新文件。**不触碰** `app/rack/**`（191 域）与 stack_probe_sendroute_macro.gd。越域即停。

## 验收标准
独立探针全绿（三态切换+自动展开/收起+手动优先+回退零变化）+既有探针零回退（189 envelope_lane 39/datachan 50、185 badge 36 新基线）+真栈腿 exit 0（§2.2 登记+三纪律+188 守卫）：装载插件→set_points 写点→自动档车道自动展开→clear_region→自动收起；工件入主仓 coord/runs/M3-ENVELOPE-LANE-AUTO-1/；端测边界声明。

## 停止条件
- 190 回执未按卡面到达前端消费链（envelope_point_count 缺失）→锚点上交；
- 三态档与既有让路链/仲裁器冲突超勘察预期→上交。

- 领取：2026-10-26 晚窗 / 主仓 origin/main=de8a607a / 前端仓领取基线 main=20873f8（worktree git status --short 干净亲核）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-LANE-AUTO-1 卡 192 专属）/ 前端分支 port/m3-envelope-lane-auto-1 / worktree D:/Godot/project/vit-daw-frontend-envauto1 / 领取提交=本提交；领取前核对：191 仍在 todo 未领取（真栈错峰对象）、doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）、PC-RUNTIME-STACK 当前占用=空（最新释放 190@18:57）。
- 回执：（commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
