# SCORE-BINDING-SAFE-1（397）终验：PASS（零栈 + 真栈 33/33）

- 卡：`2026-11-17-SCORE-BINDING-SAFE-1`（池序 397，父 392）；主管=lead；执行=lead/score397stack。
- 实现：`c6991e18`（分支 codex/score-binding-397）；独立复核 `c17bf4bf`；集成适配/真栈 S1 `b5f74fbb`；已合入 main `8972c5a5`。
- 真栈轮次（单 owner、单次授权）：
  - attempt1 `run_20261118_201839`：31/33，2 失败=`J2-non120-clip-grid-snapped` / `J1-score-panel-from-project-journey` 旧期望仍等 10.75/上取整；归因 `smoke_expectation_stale`（非产品/非环境）。
  - 修订 `e882267e`：只改上述两条过期期望为修正后模板形态（2/4 @ 80bpm、clip 8 拍），保留 J2 面板安全三断言与全部其它断言；断言面仍 33。
  - attempt2 `run_20261118_202521`：**PASS，33/33，exit 0**（PS1=0/腿=0/summary exit_code=0；console `verdict: PASS (33 assertions)`）。
- 真栈证据：S1 五条（UI 绑定身份==真实 apply 载荷目标==kernel 回读目标；选轨前无隐式目标）全绿；A1–A8 十九；J1/J2/J3 九（含修订两条）；P5 Surge XT/probe/WAV 差分全绿。
- 装置卫生：recents 快照/还原/teardown 五段哈希一致 `807226F4…`；TEARDOWN_ALL_ZERO（Vit 进程=0、TCP 六口=0、UDP 4445=0）；零 LLM 死端点；一次正式轮、无重跑。
- 主管复核：summary exit 0/assertions fail=0、console verdict、teardown/recents 亲证、进程/端口即时复核全 0；证据已合入 main `aa3e9b47`，栈位已登记释放。

## 覆盖边界（不阻塞本卡 PASS）

- S2（非目标 clip 前后不变）/S3（dirty 切换真栈腿）/S4（多 clip 显式选择真栈腿）尚未进真栈断言；零栈 focused 11 反例已覆盖 dirty/多 clip/失败态，边界已写入 RUNBOOK §8。
- `scripts/webui_rendered_dom_smoke.mjs` 的 397 适配已完成零栈 probe，但 E2E-WEBUI-1 的 agent 实跑面需其 owner 复跑确认；不得以本卡真栈 PASS 推定其已绿。
- 用户手测按既有裁定（窗口/手测节点）安排；本卡为评分轨道绑定安全，不作为渲染面审美终验。

## 后续登记

- E2E-WEBUI-1 owner 复跑 score 段适配（agent 实跑面）。
- 如需 S2/S3/S4 真栈腿，另开小卡或并入后续评分卡；不追加重开本卡。
