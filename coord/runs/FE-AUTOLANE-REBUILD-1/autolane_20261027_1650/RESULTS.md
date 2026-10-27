# FE-AUTOLANE-REBUILD-1 真栈腿回执（RESULTS）

- 卡：FE-AUTOLANE-REBUILD-1（池序 202，重设计链卡 2）/ owner=ZCode GLM-5.3 L2 PC 执行侧 ZCode 会话（本卡会话）
- 被测：前端仓 worktree `D:/Godot/project/vit-daw-frontend-autolane1` @ `port/fe-autolane-rebuild-1` @ **b2f52fd**（已推远端；基线 main=a0bc0c4）
- 交付面：车道骨架按 2026-10-27 GUI 重设计裁定链重写——旧三态档/在场性扫描/粘性/198 MenuButton 在本卡文件域内重写作废；轨道头两态开关（198 槽位沿用）+叠多车道容器（默认展开=音量一条）+统一参数选择器（轨道恰音量/声相 201 两行→发送占位禁用 204→插件机架实际装载；●N=envelope_point_count>0）+空车道单击即建首点（199 手势语义迁移）+195 几何迁移只呈现（编辑归 203）+收起零残留。
- run：`coord/runs/FE-AUTOLANE-REBUILD-1/autolane_20261027_1650/`（工件随 `port/fe-autolane-rebuild-1-stack` 入库）

## 一、离线探针（-s 零栈零 UDP；LLM=0 全确定性）

新探针 `tools/probe_autolane_rebuild.gd`（新文件）：**73/73 PASS，exit 0**

- A 装配与两态开关（15）：两态控件 198 槽位（电平 label 前）/默认收起+IGNORE+hit 空/开关开=展开+默认音量一条挂起绑定/描述符回执落定/标签=轨道·音量/值域标签随描述符单位 dB/波形 lane 实例零改/开关关零残留/重开车道清单复原（内存保留）；
- S 叠多车道（4）：＋添加=车道+1/容器高度=＋条 22+96×N 底部叠加（189 C 案先例）/×关闭=车道-1+高度回缩；
- P 统一选择器（15）：分组序轨道→发送→插件/轨道组恰音量+声相两行（201 镜像）/●N=count>0（声相 ●2）+count=0 无标记/发送占位禁用/插件组=目录实际参/缺 count 键=不标+count_seen=false/选择绑定+标签=组名·参数名/描述符值域/目录外拒选/菜单分组头+禁用项；
- C 曲线呈现（7）：rid 命名空间/回执逐点（t,v,c）/pan 值域 -1..1/t→x 秒窗线性；
- F 空车道建点（12）：EMPTY 披露/按下即建/拖动零写/释放恰一笔/region=[0,尾垫] 无锚分支/payload 寻址/v clamp 值域/写 ok 回显重拉+逐点一致 LOADED/**F7b 自写回执重拉描述符（真栈 R1 L4 缺陷的回归断言）**/写 error 回滚 EMPTY+文案逐字透传；
- G 195 几何迁移（2）：segment_curve_value 对独立参考实现逐值一致（c=0 线性/+0.25 贝塞尔/±0.8 平台+贝塞尔/升降段/端点）+c=0 线性退化；
- E fail-closed（7）：空目录=仅发送占位+零车道/空目录拒选/零车道容器读回 idle/错误透传/beats 拒呈现/queue_failed；
- W 触发链迁移（3）：automation 写回执全源重拉/rack 拓扑回执刷新（无 repo=保持注入目录 fail-closed-keep）/收起零请求。

旧探针迁移四件+既有回归六件全部 exit 0：

| 探针 | 结果 | 基线（199 回执口径） | 处置 |
|---|---|---|---|
| probe_autolane_rebuild（新） | PASS 73/73, exit 0 | — | 新建 |
| probe_envelope_lane_ui（189） | PASS 36/36, exit 0 | 39 | 迁移重写（处置清单 §三-1） |
| probe_envelope_lane_auto（192） | PASS 25/25, exit 0 | 41 | 迁移重写（处置清单 §三-2） |
| probe_envelope_lane_toggle（198） | PASS 27/27, exit 0 | 40 | 迁移重写（处置清单 §三-3） |
| probe_envelope_closeup（194） | PASS 81/81, exit 0 | 83 | 摘车道⤢段（处置清单 §三-4） |
| probe_envelope_curve（195） | PASS 55/55, exit 0 | 55 | 零改动（无车道断言；几何迁移新增断言在 autolane G=净增强） |
| probe_envelope_first_point（199） | PASS 41/41, exit 0 | 41 | 零改动（画布侧） |
| probe_envelope_detail_home（200） | PASS 52/52, exit 0 | 52 | 零改动（窗侧入口覆盖在位——⤢ 摘除后窗侧入口不丢失） |
| probe_plugin_envelope_badge（191） | PASS 33/33, exit 0 | 33 | 零改动 |
| probe_automation_datachan（187） | PASS 50/50, exit 0 | 50 | 零改动 |

合计 10 件 473 检全绿。

## 二、真栈腿（§2.2 登记+三纪律+188 守卫+VIT_PROJECT_XML 隔离+错峰）

- **登记**：541a5543 推 main 成功+远端重读确认置顶（16:35 登记）；登记前只读核对 16:34:26 进程/端口全零+fetch 重读占用空（最新释放=201@15:12）+错峰核对无在飞真栈卡。
- **三纪律**：a) 登记推送后**即时复核**（16:36:02 远端亲读+进程/端口全零，距 ps1 启动 <60s 窗口）+driver 启动前 PREFLIGHT_CLEAN 二次预检（ps1 首行自检，console/ps1_minimal.log）；b) 无 already listening（kernel pid 20320/agent pid 35008 均本腿自启，188 守卫版脚本）；c) 登记先于启动。
- **栈形态**：kernel=stack worktree `D:/Vit_DAW_wt_autolane1_stack`（70b690c0=origin/main 含 201 内核面合入）自建 VitApp.exe（CMake configure+build exit 0，sha256 c665ec28…；**不沿用主检出 10-19 旧 exe**——本卡消费 201 轨级描述符行；VIT_TRACKTION_ENGINE_DIR=主检出只读；libzmq/cppzmq/libsodium FETCHCONTENT 源=主检出 _deps 只读复用——首试网络下载失败后本地源重配 exit 0）；agent=stack worktree 构建（Go 零改动）；被测=前端 `autolane1@port/fe-autolane-rebuild-1@b2f52fd`。
- **探针**：`stack_probe_autolane_rebuild.tscn` 23 检。**R1 首试 22/23（L4 缺陷轮留证 `probe_r1_attempt1_defect.log`）→ 修复 b2f52fd（见 §四）→ 连续两轮全绿：R1 23/23 + R2 23/23 均 exit 0（SUMMARY_JSON checks_failed=0 双轮，16:39/16:40）**：
  - setup 六检全绿（clear_project/add_track/get_project_state 发现轨级 volume 插件 196 面/plugin_search 真发现 fallback=false/rack_add_node 装载 ValhallaSupermassive/描述符预热）；
  - A1 真行装配：track_row.tscn 实例化→容器 TrackEnvelopeLane2D+两态开关在真实装配树在场；
  - L1 展开+默认音量车道：开关按下=展开→默认车道挂起绑定→描述符请求走生产链（bus 回执）→绑定落定→get_curve EMPTY 披露（生产链）+标签=轨道·音量；
  - L2 音量/声相车道在场：统一选择器轨道组=**恰 volume/pan 两行（201 实面镜像）**+envelope_point_count 在场全零→添加声相车道→轨道·声相 EMPTY 双披露+pan 值域 -1..1 实描述符；
  - L3 空车道建点：EMPTY 按下即建（生产 _gui_input）→拖动零写（bus 计数=0）→**释放恰一笔（bus automation.set_points 完成==1）+回显门 empty→loaded+内核 get_curve 读回逐点一致（t,v,c）**；
  - L4 ●N 新鲜度：写后选择器音量行 **●1（实内核 count）**；
  - cleanup clear_project。
- **VIT_PROJECT_XML 隔离亲证**：run 目录 git-clean 副本（blob 287956e7 基线，git hash-object 亲核字节一致）被内核写入（SHA1 8da4e035→711e9622，mtime 16:40=内核自动保存=隔离生效）；stack worktree `VitApp/` 运行前后 git 干净；主检出 default_project.xml SHA1 123d2f3a（mtime 14:01=本腿前既有值零触碰，与 201 释放记录 mtime 14:01:21 互证）。
- **拆栈**：owner 手拆 kernel 20320/agent 35008（路径亲核=worktree 自建）→16:43:28 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。
- **释放记录**：随本回执同批推送（登记行收口 541a5543 登记–16:43 释放）。

## 三、旧车道断言面处置清单（逐条，非弱化）

处置原则：旧断言→「保留」（同面同强度）/「迁移」（同能力换形态落点）/「废置」（用户裁定 2026-10-27 GUI 重设计裁定链直接作废的语义，全部有裁定依据+替代守卫）。

### 1. probe_envelope_lane_ui.gd（189：39 检 → 迁移后 36 检）

| 旧断言 | 处置 | 落点 |
|---|---|---|
| A1 ensure 返回波形 lane | 保留 | lane_ui A1 |
| A2 工厂分支挂载车道 | 保留 | lane_ui A2 |
| A3 工厂幂等 | 保留 | lane_ui A3 |
| A4 波形 lane 实例零改 | 保留 | lane_ui A4+autolane A9 |
| A5 波形 lane 视图模式不变 | 保留 | lane_ui A5 |
| A6 默认隐藏 | 保留（语义更新：默认收起=两态唯一形态，三态默认档随裁定废） | lane_ui A6+autolane A3/A4 |
| A7 关闭 hit 空 | 保留 | lane_ui A7+autolane A5/A13 |
| A8 关闭 IGNORE | 保留 | lane_ui A8 |
| A9 底部固定高度 96 叠加条 | 迁移（叠多车道几何超集：＋条 22+96×N 底部叠加，波形 lane 仍零改） | autolane S2/S4+lane_ui A10/A11 |
| C0 目录注入 | 迁移（统一目录逐源请求断言） | autolane P1+lane_auto A1 |
| C1 select_parameter 行 API 路由 | 保留 | lane_ui C1 |
| C2 选中态 | 保留 | lane_ui C2 |
| C3 param rows 解析 | 迁移（两级 OptionButton 形态重做→统一选择器；rows 断言在目录状态面） | lane_auto A3/A7+autolane P3 |
| C4 换参 rid 变化+呈现切换 | 保留 | lane_ui C4/C5/C6 |
| C7 换 plugin 重置 param | 废置（两级选择器形态重做为一次选中 (组,插件,参)） | autolane P10-P13 统一选中/拒选面覆盖 |
| C8 目录外拒选 | 保留 | lane_ui C8+autolane P13/E5 |
| B0/B2/B3/B4 曲线读回逐点 | 保留 | lane_ui B0/B2-B4 |
| B1/B6/B7 值域描述符优先+归一 | 保留 | lane_ui B1/B5+autolane C4（pan -1..1 实面） |
| B5 t→x 秒窗线性 | 保留 | lane_ui B5+autolane C6 |
| D1/D2 wire 两键+cmd 补齐 | 保留 | lane_ui D1/D2 |
| E1 空目录 | 保留（语义更新：无 repo=保持注入目录 fail-closed-keep，注入空=仅发送占位） | lane_ui E1/E2+autolane E4-E6 |
| E3 空曲线 empty 披露 | 保留（EMPTY=建点入口态） | lane_ui E3+autolane F0 |
| E4 内核错误文案逐字透传 | 保留 | lane_ui E4+autolane E1 |
| E5 beats 拒呈现 | 保留 | lane_ui E5+autolane E2 |
| E6 queue_failed | 保留 | lane_ui E6+autolane E3 |

净变化：-3 全部为两级选择器形态专属（C3/C7 并入统一选择器断言）；两态/几何面全量在位且新增叠多车道超集断言。

### 2. probe_envelope_lane_auto.gd（192：41 检 → 迁移后 25 检）

| 旧断言 | 处置 | 落点 |
|---|---|---|
| A 三态状态机（默认 auto/bogus 拒收/manual/off 迁移） | **废置**（裁定：「在场性驱动」三态语义整体废弃） | lane_auto E2（三态 API 不在场守卫）+lane_toggle D5/M5（行面同） |
| B 在场性扫描求值（pending 不落结论/任一 count>0=在场展开/全零收起/错误保持现态/field_seen） | 消费面迁移（count 字段消费从「驱动可见性」改「●N 标记」=裁定②；扫描机械废置） | lane_auto B1-B5（●N/计数缺在场/非数值 fail-closed）+A4（count_field_seen 逐源）+autolane P4/P5/P8/P9 |
| B presence rid 命名空间独立 | 迁移（合并为 env_lane_params_* 统一目录源——扫描面废） | lane_auto A1 |
| C 手动粘性覆盖（override ±1） | **废置**（粘性语义随 auto 档废；可见性=两态开关唯一） | lane_auto E1/E3（在场证据不自动展开+全零不自动收起=防复活守卫，比粘性断言更强地钉死「可见性仅由开关决定」） |
| D 外部命令触发链（automation 写→重扫/rack 拓扑→目录重读/描述符回执不触发防环） | 保留（触发面同三命令） | lane_auto C1/C3+autolane W1/W2 |
| E 两态 API 回退零变化 | 保留（两态成为唯一形态） | lane_toggle D1-D4/T1-T4+autolane A6/A13/A14 |

净变化：-16 全部为三态/presence/粘性专属面（裁定直接作废）；envelope_point_count 数据消费全量保留+新增防复活守卫。

### 3. probe_envelope_lane_toggle.gd（198：40 检 → 迁移后 27 检）

| 旧断言 | 处置 | 落点 |
|---|---|---|
| M MenuButton 三项「自动/开/关」radio 互斥+槽位 | 迁移（两态 toggle：198 槽位沿用+**新增旧控件名不在场=形态拆除实证**+非 MenuButton） | lane_toggle M1-M6 |
| D 默认档 auto+空目录零请求 | 迁移（默认收起+零请求=无扫描面） | lane_toggle D1-D4 |
| T 三选联动（开=manual 展开/关=off 收起/自动=重入扫描+presence 正证据展开） | 迁移（按下展开+默认音量车道+再按零残留+重开复原；presence 展开面随裁定废） | lane_toggle T1-T4+autolane A6-A14 |
| S 双向同步（text/勾选跟随/档位重分类同发/同档重入不发） | 迁移（pressed 跟随/信号每实变更发/同态重入不发；重分类面随三态废） | lane_toggle S1-S4 |
| R off/manual 零自动行为 | 迁移（收起态 automation/rack 回执零请求） | lane_toggle R1-R2+autolane W3 |

净变化：-13 全部为三态/presence 专属面；两态面全量在位+新增拆除实证。

### 4. probe_envelope_closeup.gd（194：83 检 → 81 检）

| 旧断言 | 处置 | 落点 |
|---|---|---|
| O5 车道头 ⤢ 按钮在场 | **废置**（裁定：车道头=参数选择器+关闭钮；特写窗包络模式整体退役归 204） | 无（形态拆除；lane_toggle M5 拆除实证同族） |
| O6 ⤢ 信号带当前选中 emit | **废置**（同上）；窗侧入口能力不丢失——vit_open_envelope_detail 直达面在 probe_envelope_detail_home.gd R 段（52/52 全绿在位） | detail_home R 段（既有） |
| 其余 R/O1-O4/T/P/G/W/E | 保留（特写窗画布本卡零改动） | 本探针 81/81 全绿 |

### 5. probe_envelope_curve.gd（195：55 检 → 55 检零改动）

- 零车道相关断言（全部面向特写窗画布，画布未触）；195 几何向车道侧的迁移新增独立断言（autolane G1/G2：参考实现对照+c=0 线性退化）=净增强。

### 6. 真栈探针（189 stack_probe_envelope_lane / 192 stack_probe_envelope_lane_auto）

- 保留原样（历史腿工件，其记录的 run 在各自钉定 commit 有效）；新形态真栈腿=本卡 `stack_probe_autolane_rebuild.tscn`（23 检×2 轮）。

## 四、R1 首试缺陷与修复（LLM=0 流程披露）

- **L4 失败（assertion_fail，产品缺陷非探针缺陷）**：本容器写回执经 WRITE 分支早退，●N 新鲜度重拉（192 触发链迁移面）对**自身写入**永不触发——外部写回执正常（W1 离线断言过）。
- 修复 b2f52fd：写回执=automation 写完成（写入方含自身），apply_write_reply 后立即 `_refresh_counts_if_expanded()`；离线探针补 F7b 回归断言（73/73）。
- 修复后连续两轮真栈全绿（R1+R2 23/23×2）=稳定性口径达成，非单次 exit 0。

## 五、端测边界声明

- **渲染面未覆盖浏览器级 DOM 断言**（E2E-WEBUI-1 建设卡未就位）：叠车道布局视觉效果/波形与车道叠加观感/播放头视觉/选择器弹层观感未过渲染面端侧烟测；探针断言到内部状态+几何采样值+内核读回一致。
- **手感面留手测**：按下即建即拖的交互手感/两态开关操作感受/车道增删操作流未做用户旅程烟测（JOURNEY-1 未就位）；按 GUI 重设计裁定链「实施后用户手测节点终验」，渲染/手感归用户手测面。
- 探针侧目录成员注入（set_track_volume_plugin_id/set_plugin_catalog）为 199 set_plugin_catalog 同姿态；生产链目录发现（repo 面）经真栈腿 A1/L1 生产传输链验证（描述符/曲线/写回全部生产链）。

## 六、§8 记账

- LLM=0（全确定性探针+脚本）；运行前声明：成功=探针 exit 0+SUMMARY_JSON checks_failed=0；失败分类=assertion_fail/environment_interrupt/script_exception；止损=同签名断言失败×2 停止上交（未触发——R1 首试失败为单次产品缺陷，修复后双轮全绿）。
