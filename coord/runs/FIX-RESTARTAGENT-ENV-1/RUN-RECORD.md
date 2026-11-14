# RUN-RECORD — FIX-RESTARTAGENT-ENV-1 真栈轻腿（restart_agent_env_passthrough_check.ps1）

- run 根：coord/runs/FIX-RESTARTAGENT-ENV-1/verify_20261114_2230/（轮 2，有效轮）
- 占用窗口：占用登记 3ce55cac（22:15，main 亲证）；轮 1 22:20（装置缺陷轮）+轮 2 22:30（有效轮）；释放登记随本记录提交（main）
- 被测：D:/Vit_DAW_wt_env1 @ port/fix-restartagent-env-1（HEAD af645034 装置修复后；dev_agent_smoke.ps1 契约注释+新断言工具；**既有脚本零行为 diff，被测面=脚本重启链本身**）
- agent 二进制：sha256 52569D9B22A582DC6839D47E0D068470B037E6388C483CC877C1BB269FBC79C4（worktree 现场构建 exit 0 22:08，origin/main 源；-SkipBuild，二进制不在被测面）
- 命令与退出码：
  - `powershell.exe -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_env1/scripts/restart_agent_env_passthrough_check.ps1 -RepoRoot D:/Vit_DAW_wt_env1 -ArtifactsDir <run根>` → **exit 0 ALL_PASS（轮 2）**；轮 1 同命令 exit 1（装置轮：StrictMode PropertyNotFoundStrict，见失败分类）
- 验证链（phase 1 透传断言）：本会话设 VIT_PLUGIN_CONTROL_EVAL_SEAM=1 + VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1 → powershell.exe 子进程（边界 A）→ dev_agent_smoke.ps1 **-SkipBuild -RestartAgent -NoChatSmoke exit 0**（真停+真启，边界 B）→ psutil 活体读 VitAgent.exe 进程 env → **pid=7156 两键均="1"**（agent_env_phase1.json 在卷）→ **PASS**
- 回归链（phase 2 无变量零变化）：同会话清两变量 → 同款重启 **exit 0**（与无变量基线行为一致）→ psutil 读 pid=7116 **两键缺省**（agent_env_phase2.json 在卷）→ **PASS**
- 无内核运行注记：dev_agent_smoke exit 0 含 /agent/state gate 通过（ZMQ 5555/5556 warn 为预期告警，agent 无内核可独立服务 HTTP 面）——state gate 未要求内核，卡内 -StartKernel 预案未启用
- 拆栈与 §10 卫生：finally 停 VitAgent（pid=7116）→ 7878/5555/5556 释放亲核；独立复核 22:3x 五端口+五进程名 BUSY_COUNT=0；global_project_config.json 快照还原哈希前后一致 **4B835024A463EFBDE7CB97983BC7F7B08AD157427918AF2134190E859F2C48AA**（快照内容仅 recents，无敏感键，入库于卷）
- 失败分类（§8）：
  - 轮 1（verify_20261114_2220）=**装置缺陷轮**：断言装置自身在 Set-StrictMode Latest 下访问 psutil JSON 行的可选属性 .error 抛 PropertyNotFoundStrict；phase1 数据面已阳性（pid=3812 两键=1，agent_env_phase1.json 在卷），烟测 exit 0，finally 拆栈+配置还原完成。根因明确修复（PSObject.Properties 安全读，commit 装置修复）后轮 2 重跑。非环境中断、非断言失败、非 agent/脚本功能缺陷。
  - 轮 2 =有效轮，两腿全 PASS，一次通过（无凑过：断言数据为进程 env 活体读回原值）。
- 结论：**真栈轻腿验收面达成**——透传断言 exit 0 + 无变量回归 exit 0（卡面验收标准两条）。
