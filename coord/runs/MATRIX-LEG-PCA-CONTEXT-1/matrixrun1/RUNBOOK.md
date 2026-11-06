# RUNBOOK：261 MATRIX-LEG-PCA-CONTEXT-1 真栈验证腿 matrixrun1（§8 跑前口径，跑前成文）

- 日期：2026-11-06 晚窗 / owner=ZCode GLM-5.3 L1 PC 执行会话（261 卡会话）
- 被测：worktree D:/Vit_DAW_wt_261impl @ port/matrix-leg-pca-context-1（领取 ff6b8318 / 实现 5e5dd86e）
  - 改动面=scripts/ 四 py（matrix 腿族接服务端 /agent/authority 授权）；agent/Go 代码零改动（git diff 亲证）→ agent=worktree 现场构建 stock main
  - kernel=wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A 只读复用（225/249/251/253/254/256/258/263 同款；经 worktree VitApp/build/VitApp_artefacts/Release/ 副本供 Godot dev 候选拉起，副本哈希跑前亲核）

## 命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_261impl/scripts/run_de_esser_control_validation.ps1 -RepoRoot D:/Vit_DAW_wt_261impl -Phase Matrix -Sets both
```

（与 257 批 1 同入口同参数；257 基线 15ef1050→本卡 964c8026 之间 agent 面无 harness/chat 门相关改动，可比性以门代码锚点 enforceAgentProcessorLoadGate 语义不变为准）

## §8 口径（跑前成文）

1. **运行次数**：单 run，无重跑。同代码版本同断点失败两次即禁原样重跑（AGENTS §8）。
2. **成功条件**（全部满足为 PASS）：
   - PS1 exit 0；
   - de-esser summary verdict=passed；
   - 9 个真实插件 case（training 5 + regression 4）status=passed；
   - 5 个 synthetic case=covered_by_unit_fixture（设计内）；
   - **门拦截计数=0**：全部 case 工件 json 中 `pca_load_gate` 字样计数 0（257 形态=9/9 同型拦截，修复后应=0）；
   - authority.json 工件在：authority_after=full_project_access；
   - 零 LLM 亲证：run 目录 agent_last.log grep llm/telemetry 零命中（原腿设计零 LLM，257 批 1 同款亲证法）。
3. **失败分类**（预写，如实分列不凑过）：
   - ①环境中断：栈未起/端口占用/build 失败/cmake 腿 throw——零 LLM 不计有效运行轮（258 首启形态先例）；
   - ②门拦截新形态：case error 含 pca_load_gate 但错误串≠257 同型（如 unpromoted/fingerprint/歧义 fail-closed）——属授权面之外的新证据，如实上交；
   - ③功能/断言失败：case rejection code 不符、classification 错、topology 变化；
   - ④脚本缺陷：py 自身 traceback（非被测面）。
4. **止损线**：单 run 失败即停，取证上交主管，不重跑不追绿。
5. **预算**：零 LLM 预期（装载+inspect 全走工具面不经 chat 轮）→ 预期 ≈¥0。
6. **栈占用**：登记 5d16a515（18:15 推送成功）；启动前 ≤60s 即时复核进程/端口；finally 脚本自拆+owner 复核四端口/三类进程全零后释放登记。

## 拆栈口径

- ps1 finally 自拆（Godot CloseMainWindow→ WaitForExit → 端口进程清场）；
- owner 拆栈后复核：5555/5556/7878/8787 listen=0 + VitApp/VitAgent/VspHub=0 + 本腿 Godot 进程=0；
- 异常未拆净：保留占用登记标"待处置"上交。
