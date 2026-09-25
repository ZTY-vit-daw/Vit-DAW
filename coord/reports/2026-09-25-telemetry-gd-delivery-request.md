# 请求：FIX-GD-TELEMETRY-BELL-1 的 telemetry_manager.gd 投递到 mac（PORT-PCBATCH-MAC-LEGS-1 腿 5 补件）

- Mac 决策会话 2026-09-25；腿 5 已按"以 PC 已提交块为界"如实上交（四路渠道核查见 `~/Documents/vit-pcbatch-mac-legs-artifacts/leg5_frontend/CHANNEL_AUDIT.md`：transfer 远端/主仓全分支/GitHub org 仓清单/coord 工件均无该文件）
- **请求 PC 投递**：Godot 仓 `telemetry_manager.gd` 已提交的标注块+probe 到 mac 前端仓（`~/Documents/vit-daw-frontend`）。注意回执注记的"同文件他人 18 行 WIP 分账"——**以已提交块为界，勿夹带本地 WIP**。
- 通道二选一：
  1. `git format-patch` 该提交 → 推 transfer 仓 `inbox-pc/`（<100MB 合规）；
  2. SSH scp 直传（PROTOCOL §9，mDNS 主机名 `TimoZTYdeMacBook-Air.local`，IP 回退 192.168.1.131）+ transfer 留 SHA256 通知单。
- 到达后 mac 侧补腿 5（前端仓 port/* 提交+加载确认）并回执闭环。
