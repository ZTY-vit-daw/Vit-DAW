# FE-AUTOLANE-EDIT-1（203）真栈腿回执——autolane_edit_20261027_1730

- 卡：[FE-AUTOLANE-EDIT-1](../../../cards/done/2026-10-27-FE-AUTOLANE-EDIT-1.md)（池序 203，重设计链卡 3；领取 97d47c0b@main）
- 执行：GLM-5.3 L2 PC 执行会话（ZCode）；2026-10-27 17:42–17:46
- 被测=前端 worktree `D:/Godot/project/vit-daw-frontend-edit1` @ `port/fe-autolane-edit-1` @ **94386bb**（已推远端；deee903 实现+94386bb 栈探针与单点延伸线命中分支）
- 交付面：194/195 编辑逻辑迁移入 202 车道画布——车道局部手势机（拖点 Shift 锁 t/Ctrl 锁 v+实时数值标签/折线加点按下即建/Delete 删点/删至唯一点=clear_region）+写回链（释放恰一笔 set_points region c 透传+回显门逐点对账+error/队列失败/超时回滚 baseline+拖拽态外部回执挂起零位移释放合并）+曲率编辑（段中点单柄=曲线采样值+左键拖连续 c+右键预设四档+T/V 吸附默认关）+Ctrl+Z/Y 接内核 undo/redo 事务（回执 ok 全条重拉对齐+●N 刷新/error 请求条披露/拖拽态与收起态不劫持）。**vit_dock/** 零改（194/195 件拆除归 204）**。

## 一、离线探针（无栈腿，-s 模式）

- 新探针 `tools/probe_autolane_edit.gd` **63/63 检全绿 exit 0**（G 手势机 G1-G11/W 写回链 W1-W5c/K 曲率 K1-K4f/S 吸附 S1-S5/U 撤销 U1-U7；G11=车道单点延伸线加点——202 渲染面画水平延伸线，命中面随绘制「所见即所得」，194 特写窗无此绘制故无此分支，车道侧补齐否则单点曲线无法长点）。
- **202 探针零回退**：11 件全绿=autolane_rebuild **73** + lane_ui 36 + lane_auto 25 + lane_toggle 27 + closeup 81 + curve 55 + first_point 41 + detail_home 52 + badge 33 + datachan 50（既有 10 件合计 **473** 检与 202 验收基线逐件同数）。closeup/curve/first_point/detail_home 四件打 vit_dock 适配器面（本卡零改文件），天然不回退。
- 全量合计 63+473=**536 检全绿**。

## 二、真栈腿（§2.2 登记+三纪律+188 守卫+VIT_PROJECT_XML 隔离+错峰）

- **登记**：799fe53a 推 main 成功+远端重读确认置顶（17:42 登记）；登记前只读核对 17:41:50 进程/端口全零+fetch 重读占用空（最新释放=202@16:43）+错峰核对无在飞真栈卡（doing 唯一他卡 GEN-SUNO-WEB-DESIGN-1 无栈腿）。
- **三纪律**：a) 登记推送后**即时复核**（17:42:44 远端亲读置顶+进程/端口全零，距 ps1 启动 <60s 窗口）+driver 启动前 PREFLIGHT_CLEAN 二次预检（ps1 首行自检，console/ps1_minimal.log）；b) 无 already listening（kernel pid 2980/agent pid 31288 均本腿 ps1 -StartKernel 自启，188 守卫版脚本）；c) 登记先于启动。
- **栈形态**：kernel=stack worktree `D:/Vit_DAW_wt_edit1_stack`（97d47c0b=origin/main；内核面=889a373c 含 196 automation 三命令+201 轨级描述符行+undo 事务，**本卡内核零改动**——203 全部改动在前端）自建 VitApp.exe（CMake configure+build exit 0，sha256 **317129e6**0e31227b…；VIT_TRACKTION_ENGINE_DIR=主检出只读；cppzmq/libzmq=find_package 主检出 _deps 构建产物只读复用零网络）；agent=stack worktree 构建（Go 零改动，sha256 edd3230d…）；被测=前端 `edit1@port/fe-autolane-edit-1@94386bb`。
- **探针**：`stack_probe_autolane_edit.tscn` 30 检。**R1 首试 6/8（探针缺陷轮留证 `probe_r1_attempt1_defect.log`）→ 探针侧修复（见 §四）→ 连续两轮全绿：R1 30/30 + R2 30/30 均 exit 0（SUMMARY_JSON checks_failed=0 双轮，17:45:10/17:45:22）**：
  - E1 真行装配+展开+默认音量车道生产链落定（描述符 bus 回执+get_curve EMPTY 披露，生产链）；
  - E2 空车道建首点：按下即建（生产 _gui_input）→拖动零写（bus 计数=0）→**释放恰一笔（bus automation.set_points 完成==1）+回显门+内核 get_curve 读回逐点一致（t,v,c）**；
  - E3 单点曲线延伸线加点（车道所见即所得命中面）：按下即建→释放恰一笔→内核读回 2 点一致；
  - E4 拖点改值：生产手势→释放→内核读回 v 一致；
  - E5 曲率拖柄：段中点柄左拖连续改 c 至 -0.5→释放恰一笔→**内核 get_curve 读回 c 一致（曲率 c 读回）**；
  - E6 预设切换读回：fast_in c=-1 → smooth_s c=-0.5 → linear c=0 三档逐档内核读回一致；
  - E7 Delete 删点：焦点删除→内核读回恰 1 点；
  - E8 **undo 事务往返**：编辑写（v 移动）回显落定→Ctrl+Z（生产按键链→内核 undo 事务）→回执 ok→**车道重拉对齐+内核读回=编辑前态（undo 后车道呈现与内核读回一致）**→Ctrl+Y redo→**内核读回=编辑后态+车道对齐**。
- **VIT_PROJECT_XML 隔离亲证**：run 副本 git-clean 基线（blob **287956e7**=202 同基线，hash-object 亲核）被内核写入（SHA1 **7ca02e9c**，mtime 17:45:21=本腿运行期=隔离生效）；stack worktree `VitApp/`+`agent/` 运行后 git 干净；主检出 default_project.xml 工作树 e365e72b（mtime 14:01:21=本腿前既有值零触碰，会话开局即 M 状态）。
- **拆栈**：owner 手拆 kernel 2980/agent 31288（路径亲核=worktree 自建）→17:45:47 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。
- **释放记录**：随本回执同批推送（登记行收口 799fe53a 登记–17:46 释放）。

## 三、端测边界声明

- 渲染面（拖拽标签/曲率柄观感/吸附开关观感）未过浏览器级 DOM 断言（E2E-WEBUI-1 未就位）；手感面（拖点/锁轴/删点按键/undo 快捷键操作手感）留用户手测（裁定链：实施后用户手测节点终验）。
- 探针断言到内部状态+真内核读回一致（get_curve 逐点/c/undo 事务往返），按键链经生产 _gui_input 事件注入（headless 无真实键盘焦点路径）。
- 停止条件两支均未触发（车道手势=条内局部状态机零仲裁器接触，与时间线滚动/框选/clip 手势无冲突面——MOUSE_FILTER_STOP 天然隔离，194 领土隔离先例；undo 与回显门交互=E8 实证对齐无漂移）。

## 四、R1 首试缺陷披露（探针缺陷，非产品缺陷）

R1 首试 6/8：E1d/E1e 默认音量车道绑定挂起。归因=**探针缺陷**：headless 场景无 project_repository autoload，探针未注目录（202 栈探针有 set_plugin_catalog 注入面）→`_request_track_descriptors` 不发车→轨级描述符回执永不落定→绑定挂起。修复=探针侧补 `lane.set_plugin_catalog([])` 空目录注入（触发轨级描述符请求发车，202 探针同姿势）——**零生产改动**。缺陷轮留证 `probe_r1_attempt1_defect.log`；修复后双轮全绿。
