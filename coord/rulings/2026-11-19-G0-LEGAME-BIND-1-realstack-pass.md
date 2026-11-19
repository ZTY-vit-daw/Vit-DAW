# 裁定：G0-LEGAME-BIND-1（425）真栈腿 PASS

- 日期：2026-11-19 18:30 +0800
- 卡：G0-LEGAME-BIND-1（池序 425，W01-C）
- 裁定：**真栈腿技术 PASS，卡收口**；W01（解析/编译/工程绑定）完成。
- 依据：正式轮 `coord/runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/legame_final/summary.json`（status=passed），S0–S7 全绿；主管亲自运行并独立核栈。

## 验收事实

- S1：设计源导入 `edu_jam_legame_imported`；项目包 `A.vit_project` 内编译卡 + compile manifest；manifest.files 校验和自洽。
- S2：A/B UUID `vitproj_4ce90ea8e53c42098079e5612d80dd46` / `vitproj_73ce04f5fd484961ace6d04252639a00` 相异；binding 路径 `.vit_agent/<uuid>/legame_binding.json` 相异；父目录旧 binding 未被写；A/B 编译卡路径相异。
- S3：A 工程「开始乐局」`edu_jam_entered`，level_id=`music-creation-unit2-demo`。
- S4：B 导入不串 A binding；A/B 项目包/编译卡不串。
- S5：重开 A 后新会话自然语句触发 auto-intro，装载卡 level_id 与导入一致；compile manifest cache_key 四元组逐字段一致（未重编译/未切版）。
- S6：修订包内设计源标题→缓存失效→新卡生效；`design_source_sha256` 变化、回复含新标题、包 manifest 仍自洽。
- S7：recents 快照/还原哈希一致；栈 teardown 后 VitAgent/VitApp/VspHub/Godot 进程 0，TCP 7878/7879/8787/5555/5556/9333 与 UDP 4445 全零。
- 零 LLM：`agent_llm_telemetry.jsonl` 未生成；全部走确定性 edu_jam 路由。

## 装置与边界（必须随卡记录）

1. 装置修已合入 main：`029e1c6c`（agent 端口 7878+agent 面预检）、`745b6265`（invoke `args`/`confirmed`、project.open `file_path`、state `current_project`、S5 自然语句、S6 会话延续、UTF-8 BOM）。产品实现 `bef5e9c1` 未改。
2. attempt1 为编码解析失败；attempt2 为 S5 测试期望缺陷（命令词「乐局状态」按设计跳过 auto-intro，不能作为重开取证）；attempt3 为正式轮。分类均在 [DEVICE-NOTES](../runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/DEVICE-NOTES.md)。
3. **权威输入偏差**：`VitApp/Workspace/default_project.xml` 被内核在 `project.new`/`save_as` 时改写（berth 前 `5BC9C756…` → 当前 `E9B668A0…`）。berth 只存哈希未复制，无法逐字节还原；预跑内容是六 stems 测试残留（temp 路径），不是用户工程。`Settings.xml` 未变；recents 已还原 `63D0F3F7…`。后续真栈腿必须在 berth 启动前**复制**权威文件并在 teardown 还原（本轮装置债，另行处理）。
4. S5 观察到同进程 `project.open` + 新会话的 auto-intro 返回 `edu_jam_entered`（新局）而非 `edu_jam_recovered`；这是同进程切换场景，**不覆盖** 315 的跨进程重启恢复路径。本卡 S5 的验收核心是 cache/version，已通过；315 恢复回归需单独真栈腿确认（不在本卡域内）。

## 结论与下一步

- 425 转 done；W01 收口。下一步：422 跨工作树组合（需先写 runbook/脚本）→ W02 最小切片设计与实现。
- 装置债：berth 权威文件复制/还原；425 脚本内建 A/B 创建或 setup 前置隔离；POOL-INDEX 397–431 回填。
