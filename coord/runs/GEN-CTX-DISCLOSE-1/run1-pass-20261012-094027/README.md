# GEN-CTX-DISCLOSE-1 权威冒测轮（run1-pass-20261012-094027）

- 脚本：`scripts/music_context_disclosure_smoke_mac.sh`（内核直连 ZMQ REQ tcp://127.0.0.1:5555 两阶段；agent/Godot 不在探针面——agent 用户可见工具面会过滤内核键，本卡零 agent diff，断言面=内核回复本体）
- 退出码 0；24/24 门全绿（summary.json schema `music.context.disclosure.mac.v1`）
- 双内核：baseline=9a70f064…（GEN-MAC-BASELINE-1 构建，源 32fcab06 与本卡基线 dba5c516 的 VitApp/Source 零 diff 已核证）→ candidate=8e445dbb…（本卡 worktree 增量构建，含 appendMusicContextDisclosure）
- 关键证据：
  - `zmq/B_open.json`：music_context={tempo_bpm:128.0, tempo_setting_count:1, time_signature:"4/4", numerator:4, denominator:4, time_sig_setting_count:1, key_signature:null, style_note:""}（与设计件二 §1.1 逐键一致）
  - set_tempo 联动：phase A set 128→auto-persist→phase B open 披露 128.0；phase B set 96→回显 96.0→再查 96.0（引擎真值面 getTempo(0) 读写一致）
  - 零变更快照 diff：A_open vs B_open 剥离 music_context 后逐键相等（diff_count=0）；键集 delta=∅（keyset_delta_exact=true）；唯一规范化易变路径 `$.plugin_list_hygiene.completed_at`（内核启动墙钟戳，TIM-KERNEL-HYGIENE-1 语义，与工程语义无关）
- 探针迭代史（AGENTS §8 失败分类，均为脚本缺陷/环境中断，非栈功能失败）：见 ../probe-iteration-history/——run1=内核 Tracktion 设备管理器初始化挂起（环境中断，PORT-SMOKE-MAC-4 先例）；run2=探针面选错（agent 投影层过滤内核键→改直连 ZMQ）；run3=三处门断言 bash 3.2 兼容缺陷+易变时间戳未规范化
- teardown 复核：7878/5555/5556/5557 监听=0、VitApp/vitagent 进程=0（脚本自 teardown，收尾独立复核）
- run 时 repo 状态：repo_head=dba5c516 + 本卡三文件未提交改动（CommandDispatcher.cpp / 冒测脚本 / SMOKE_TESTS.md）——候选二进制即从该树构建（repo_dirty=3 entries）
