# FE-ENVELOPE-FIRST-POINT-1 真栈腿回执（RESULTS）

- 卡：FE-ENVELOPE-FIRST-POINT-1（池序 199）/ owner=ZCode GLM-5.3 PC 执行会话（ZCode，199 卡会话）
- 被测：前端仓 worktree `D:/Godot/project/vit-daw-frontend-firstpt1` @ `port/fe-envelope-first-point-1` @ **3f52ad5**（已推远端）
- 交付面：`envelope_closeup_canvas.gd` 空曲线（CURVE_STATE_EMPTY）首点创建——单击画布按下即建即拖（折线加点同手势机）；首笔 set_points region=[0,尾垫]（`_edit_region` 既有无锚分支，C1 契约零新语义）；探针只建新文件（`tools/probe_envelope_first_point.gd` + `tools/stack_probe_envelope_first_point.gd/.tscn`），194/195 既有探针文件零改动（git diff 空实证）。
- run：`coord/runs/FE-ENVELOPE-FIRST-POINT-1/envelope_first_point_20261027/`

## 一、首笔 set_points region 契约勘察（卡面目标②，回执披露）

从零到有的区域语义**执行侧勘察结论：零新语义**。

- `_commit_point_edit("add", [t_press, t_release])` 在 baseline=空（从零）时走 `_edit_region` 既有两分支的组合：
  - **start=0.0**：baseline 无更早锚点→前锚缺省 0（既有「编辑首锚点」同分支）；
  - **end=max(t_release, total_duration, view_end)+1.0**：baseline 无更晚锚点→无后锚尾垫（既有「尾加点/拖尾锚」同分支）。
- C1 契约（automation_client.gd + PluginRackControlService 亲核注释）：region 模式 start≥0、end>start、区域内新点接收域=[start,end] 闭区间；从零场景 start=0/end=尾垫>t_release 全部满足，不触任何校验边界（无 malformed_region/point_out_of_domain 面）。
- 幂等面不变：region 内旧点移除（空曲线=无点可移除）+新点回灌；clear_region 路径零改动（唯一点删除语义仍由 194 既有 V7 面覆盖）。

## 二、离线探针（-s 零栈零 UDP；LLM=0 全确定性）

`tools/probe_envelope_first_point.gd`（新文件）：**41/41 PASS，exit 0**（首轮即绿）

- S 从零装配（4）：EMPTY 披露前提态+描述符值域；
- G 空态守卫（6）：scrub 带/吸附开关命中面零冲突（停止条件②邻面钉死）；
- F 主旅程（13）：按下即建（F1a/F1b）→手势期零写（F1c/F2b）→释放恰一笔（F3）→region=[0,31.0] 无锚分支（F4）→payload 寻址/单点（F5/F6）→回显门重拉读回一致（F7/F8）→empty→loaded（F9/F10）；
- E 错误回滚+重试（7）：写回 error→回滚空基线+write_rejected(target_missing) 披露+EMPTY 可重试→重试成功 LOADED；零位移单击（无拖动）同样建点（E1a，was_new 绕过零位移守卫=卡片手势语义）；
- A assumed 值域（5）：无描述符 assumed [0,1] clamp（顶越界 1.0/底越界 0.0）+同无锚 region。

## 三、既有探针回归（只读复跑；194/195 文件零改动=git diff 空）

worktree `firstpt1@3f52ad5` 下全部 exit 0：

| 探针 | 结果 | 卡面期望 |
|---|---|---|
| probe_envelope_closeup (194) | PASS 79/79, exit 0 | 79 |
| probe_envelope_curve (195) | PASS 55/55, exit 0 | 55 |
| probe_envelope_lane_ui (189) | PASS 39/39, exit 0 | 39 |
| probe_envelope_lane_auto (192) | PASS 41/41, exit 0 | 41 |
| probe_plugin_envelope_badge (191) | PASS 33/33, exit 0 | 33 |
| probe_automation_datachan (187) | PASS 50/50, exit 0 | 50 |

注：lane_ui 首轮 FAIL 为本 worktree 新建 `.godot` 导入缓存缺 `frosted-warm.png` ctex（资源在仓、主检出 211 ctex vs worktree 0）——`--headless --import` 生成 160 ctex 后复跑即绿；判定=环境面非代码回归（失败输出留 `/tmp` 会话临时目录，未入工件）。

## 四、真栈腿（§2.2 登记+三纪律+188 守卫；与手测节/198/200 错峰）

- **登记**：09453dfb 推 main 成功+远端重读确认置顶（机器钟 13:03 前后；登记行文内「13:05」为预计时戳，机器钟时间线为本节权威）。
- **三纪律**：a) 登记 push 后即时复核=远端记录亲读+进程/端口全零（VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0）+driver 启动前二次预检 `PREFLIGHT_CLEAN`（ps1_minimal.log 首行，登记-启动窗口内）；b) 无 already listening（kernel pid 27316/agent pid 23696 均本腿自启，188 守卫版脚本防复用）；c) 登记先于启动。
- **错峰**：200 12:50 释放+198 12:57 释放均 fetch 亲核后才登记；用户手测节=本腿全程 headless 场景探针无 GUI、窗口 13:03–13:05。注：200 登记注记中 12:36–44 的 lane_auto/lane_ui `-s` Godot 进程=**本 199 会话离线回归探针**（零栈端口），非 198——此处澄清归因。
- **栈形态**：kernel=主检出 K3 只读复用（sha256 722A79D7…，178–195 腿同源；本卡零内核改动亲核 e70bafc7..origin/main=ea599e88 无 VitApp/ 提交）；agent=stack worktree `D:/Vit_DAW_wt_firstpt1_stack`（ea599e88，Go 零改动）构建 pid 23696；ps1 最小腿 **exit 0**（console/ps1_minimal.log）；被测=前端 `firstpt1@3f52ad5`。
- **探针**：`stack_probe_envelope_first_point.tscn` **连续两轮 R1 17/17 + R2 17/17 均 exit 0**（SUMMARY_JSON checks_failed=0 双轮，console/probe_r{1,2}.log）：
  - setup 七检全绿（clear_project/add_track/plugin_search **真发现 fallback=false**/rack_add_node ValhallaSupermassive/描述符预热/目标参数）；
  - F1 从零前提双源：内核 get_curve 空 + 视窗 EMPTY 披露（生产传输链）；
  - F2 按下即建（create-on-press，t/v=点击映射真描述符值域）；
  - F3 手势期零写（bus automation.set_points 计数=0 跨两次 motion）；
  - F4 释放恰一笔（bus 完成==1）+回显门 empty→loaded+**内核 get_curve 读回逐点一致**。
- **VIT_PROJECT_XML 隔离三面**：隔离副本 sha 4E899FB6→8BAE6D17（内核写入=隔离生效亲证）；stack worktree `VitApp/` git 干净+default_project.xml mtime 12:46（=检入时刻先于运行）+blob=HEAD（287956e7；盘面 certutil 947E4D00 与 git show 4E899FB6 差=autocrlf 行尾转换非异常）；主检出 default_project.xml=13A155AA mtime 10-27 11:54（本腿前既有值，零触碰；与 198 释放记录同值互证）。
- **拆栈**：owner 手拆 VitAgent 23696/VitApp 27316（路径亲核 `D:\Vit_DAW_wt_firstpt1_stack\agent\bin\` 与主检出 `build\`）→13:05:10 复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。
- **释放记录**：随本回执同批推送（登记行收口为 09453dfb 登记–13:05 释放）。

## 五、§8 记账

- LLM=0（全确定性探针+脚本）；运行前声明：成功=探针 exit 0+SUMMARY_JSON checks_failed=0；失败分类=assertion_fail/environment_interrupt/script_exception；止损=同签名断言失败×2 停止上交。
