# MATRIX-LEG-PCA-CONTEXT-1：compressor matrix 腿族 PCA 授权面接通（G5）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G5——9/9 门拦=腿过时非门缺陷）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 261；目标=scripts/compressor_compat_matrix_smoke.py（invoke 面）；P2 / 0.5 天 / 无依赖；L1；验证腿需真栈一次（de-esser Matrix 单腿）

## 目标

1. matrix py 腿 `/agent/invoke` 注入 full-project-access context（或 PCA 授权注入面，按 8a415ef7 修复先例的授权语义——不绕门，走门认可的服务端授权路径）；
2. 验证=de-esser Matrix 单腿真栈复跑 9 case 走到断言面（过/如实分列；门拦截=0）；compressor 系列其余三腿修后不强制本卡跑（共享模块同修，下轮体检抽查）。
- §8：零 LLM 预期（原腿设计）；单 run。

## 约束

文件域=该 py 腿+必要时 ps1 入口；禁改 harness 门本体；门语义零削弱。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）

- 领取：2026-11-06 18:11 +0800 / origin/main=964c8026（发卡基线 929d6cd9 后仅增 259 领取协调提交，chat 测试域与本卡 scripts/*.py 不相交）/ owner=ZCode GLM-5.3 执行会话·PC / 分支 port/matrix-leg-pca-context-1 / worktree D:/Vit_DAW_wt_261impl / 领取提交=本笔（hash 见 git log）

## 回执（执行侧 2026-11-06 18:2x +0800）

- 实现：`5e5dd86e` @ port/matrix-leg-pca-context-1（四文件 +51/-3，全在文件域内）：
  - `scripts/compressor_compat_matrix_smoke.py`（共享模块+compressor 腿）：新增 `ensure_full_project_access(base, timeout)`——GET `/agent/authority` 记 before → 非 full 则 POST `{"authority_mode":"full_project_access"}` → GET 复核，切换被拒/未生效即 raise；main() 在 health check 后接线，`authority.json` 工件落 output-dir，summary 增 `authority` 字段。
  - `de_esser/limiter/transient_shaper_matrix_smoke.py` 三腿 main() 同款接线（共享模块同修，未跑——卡面"不强制本卡跑"，下轮体检抽查）。
  - **门语义零削弱亲证**：invoke 请求体保持零 context；权限唯一授予点仍是服务端 `/agent/authority`（8a415ef7 授权语义+journey1 先例同款：stampInvokeAuthorityMode 服务端打戳+authorizeFullProjectAccessLoad 从 promoted PCA 目录解析唯一身份）；harness 门本体/Go 代码零改动（`git diff --stat` 四 py only）。
- 零栈自检：四文件 `py_compile` 通过；`ensure_full_project_access` 四分支语义经本地桩服务器验证全绿（manual→switch / already-full→no POST / refused→raise / empty→switch），桩脚本在 Temp 未入仓。
- 真栈验证腿（matrixrun1）：run `20261106_181640`，`run_de_esser_control_validation.ps1 -Phase Matrix -Sets both` **PS1_EXIT=0 §5 门槛过**，§8 单 run 无重跑兑现：
  - 9 真实插件 case 全 passed（training 5+regression 4；stereo/mono 同 identifier 代际一致，determinism 佐证）；synthetic 6=covered_by_unit_fixture；summary verdict=passed；
  - **门拦截计数=0**（全部工件 grep `pca_load_gate` 零命中——257 的 9/9 同型 `missing non-forgeable exact selection authorization` 清零，G5 修复面达成）；
  - authority.json 亲证 `manual_confirmation→full_project_access`（switched=true；persisted=false 无 workspace 污染，内存态随栈消亡）；
  - 零 LLM 亲证：agent_last.log grep llm/message_loop=0，计费 ≈¥0（§8 预期兑现）；
  - 被测 agent=worktree 现场构建 SHA256 05FAE77A308F1981C6927C8D91D8E95D057B254806D5E51796A7D7F1F5FA9B5A；kernel=wt_228 VitApp.exe SHA256 F9E3DA0A…5ED0D67A 只读复用（worktree dev 候选路径副本，副本哈希亲核）；
  - 栈占用：登记 5d16a515（18:15）→释放 6835cd45（18:17，TEARDOWN_ALL_ZERO 18:17:44 亲核：四端口 0+VitApp/VitAgent/VspHub 0+Godot 0）；跑前 ≤60s 即时复核 18:16:31 全零后启动。
- **端测边界声明（AGENTS §5）**：本卡真栈烟测覆盖 agent HTTP/invoke 面+插件装载+inspect 断言面（脚本退出码 0）；不涉渲染面与用户旅程（零 webui/GUI 改动）。
- 工件：`coord/runs/MATRIX-LEG-PCA-CONTEXT-1/matrixrun1/`（RUNBOOK §8 跑前成文+console+matrix 全件+agent_last/journal+godot 双 log，随 `port/matrix-leg-pca-context-1` 6c50de3a 入库）。
- 领取前已有 diff 核对：领取基线 964c8026 干净树（git status 零改动），本卡全部 diff=四 py+coord/ 工件。
