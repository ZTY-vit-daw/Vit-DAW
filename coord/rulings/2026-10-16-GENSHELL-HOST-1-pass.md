# Ruling：GENSHELL-HOST-1 pass（2026-10-16 深夜，Mac 决策侧）

- **裁定：pass**。合入 main = cherry-pick `dce49fac`（→ `db84003e`，基线 `88f313cd`，干净零冲突）。实现分支 `port/genshell-host-1` 已推在位。
- **决策侧独立复验（Mac 决策会话亲核，非转述回执）**：
  - 文件域亲核：5 文件与卡面域一致——GenerationShellService.{h,cpp}（新 51+779 行）+CommandDispatcher.cpp **仅注册面 +13 行**（include+lambda 持有 shared_ptr 注册块；未触 .h、未触披露块，diff 亲读）+CMakeLists（源登记+SDK include 路径）+`scripts/gen_shell_host_smoke_mac.sh`；agent/ 零改动（diff stat 亲核）。
  - 真栈工件亲读：`~/Documents/vit-genshell-artifacts/host/` 四 run 与回执一致（两次调试期 RED 220552/221004 + **两次全绿 221150/221207，各 26 ok / 0 red**）；关键工件补档入 `coord/runs/GENSHELL-HOST-1/`。
  - **降级 A 证据锚点亲核**：SDK `public.sdk/source/vst/vstcomponentbase.cpp` `ComponentBase::connect` 确为单 peer fail-closed（`if (peerConnection) return kResultFalse`）——回执证据链②属实；运行时探针腿（H2：qi_edit_controller=kNoInterface / qi_connection_point=kResultOk / connect_attempt=kResultFalse / t2_transport=downgrade_a_param_chunk）在两份全绿报告中在案。
  - 主选→降级 A 判定采信：证据链五条静态+运行时双证，规格预设路径（"降级启用时队列语义不降（烟测同断言）"），H3-H6 全过=队列语义未降。
- **决策侧复验边界（如实）**：未重跑全量内核构建——采信执行侧烟测两次连续全绿的内核构建证据（worktree 构建 sha256 a08f7593，run prereq 在案）；go test 0 FAIL 采信回执（agent/ 对基线零 diff 亲核成立，无需决策侧重跑的改动面）。
- **边界七条采信**：providers 恒空数组（卡 3 runner 自带目录）、cancel_requested/interrupted 无枚举值不镜像、下行三类壳侧消费者在卡 4、请求载荷 v1 内存态（重启恢复=卡 4 chunk 写入口）、requestHash 自含 sha256、无前端旅程断言、Settings.xml 运行残留未入 commit。
- **解锁**：池序 97（GENSHELL-AGENT-1，P1——GEN-CTX-INJECT-1 ✓ + HOST ✓ 双前置满足，关键路径下一张）。
- **对卡 3/4 的锚点交接**：runner 读 `gen_shell.pending`（providers 由 runner 自填）+job upsert 走 `aigc_register_job` 同面（H4 伪推进已按此面实证）；chunk 写入口（pending_requests scaffold）待卡 4。
- 验收人：Mac 决策侧（用户 2026-10-16「三张卡都已完成，可以检查」授权检查；PCA-1/REALVERIFY-1 两条 blocked 上交另行讨论处置，不在本 ruling 内）。
