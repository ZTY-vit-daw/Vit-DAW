# SETTLE-CHAIN-1 验收取证工件

- 四轮 run（§8 逐断点取证史，工件自执行 worktree 归档）：194926=断裂②复现（booking 竞态）；200332=断裂③（回放降级）；201712=断裂④（烟测时序）；**202511=PASS exit 0**（status=pass、四旗标全真、decision=user_judgment_pending）。
- §9 口径：agent 二进制=worktree 构建 20:17:07 sha16=9E43F171D4C1E8A4；内核=主树 11:51:39 sha16=A29751807DC425A6（C++ 零 diff）。
- 本 run 同时回补 DOSE-AUDIBLE-1 的端测门（其 ruling 附条款）。
