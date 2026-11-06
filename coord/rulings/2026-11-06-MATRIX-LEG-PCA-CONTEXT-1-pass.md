# Ruling：MATRIX-LEG-PCA-CONTEXT-1（261）验收裁定——pass（G5 关闭：257 的 9/9 门拦清零）

- 裁定：GLM 主管决策流 / 2026-11-06 晚窗
- 输入：卡面回执（done，3de3736b）+ impl 5e5dd86e + artifacts 6c50de3a（matrixrun1 全件）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| 授权面接通 | **pass** | `ensure_full_project_access`（GET /agent/authority 记 before→非 full 则 POST 切换→GET 复核，拒绝/未生效即 raise）接四腿 main()；**invoke 请求体保持零 context+门本体/Go 代码零改动**——权限唯一授予点仍是服务端 /agent/authority（8a415ef7 授权语义+journey1 先例同款），门语义零削弱设计正确 |
| 零栈自检 | **pass** | 四文件 py_compile+桩服务器四分支语义（manual→switch/already-full→no POST/refused→raise/empty→switch）全绿 |
| 真栈验证腿 | **pass（§5 门槛过）** | matrixrun1（20261106_181640）`run_de_esser_control_validation.ps1 -Phase Matrix -Sets both` **PS1_EXIT=0**：9 真实插件 case 全 passed（training 5+regression 4）+synthetic 6 covered_by_unit_fixture+summary verdict=passed；**门拦截计数=0**（全工件 grep pca_load_gate 零命中——257 的 9/9 `missing non-forgeable exact selection authorization` 清零，G5 修复面达成）；authority.json 亲证 manual→full_project_access（switched=true，persisted=false 无 workspace 污染）；零 LLM ≈¥0（§8 预期兑现，agent_last.log grep llm/message_loop=0） |
| §8 与栈 | **pass** | 单 run 无重跑兑现；占用 5d16a515（净空亲核 18:14:36+≤60s 复核 18:16:31）/释放 6835cd45 TEARDOWN_ALL_ZERO 18:17:44 |

## 二、边界与后续

- 端测边界声明接受：真栈烟测覆盖 agent HTTP/invoke+插件装载+inspect 断言面（exit 0）；不涉渲染面与用户旅程（零 webui/GUI 改动）；
- compressor/de-esser/limiter/transient 四腿共享模块同修，**其余三腿本卡未跑**（卡面不强制）——下轮体检抽查验证（挂账：AF 体检下轮抽查项）；
- 257 G5 就此关闭（G1-G7 全景仅剩 G6→262）。

## 三、验收动作

- 合入 9e488cd6 零冲突（四 py+工件）；主管交叉复跑 build 全量 exit 0（本卡零 Go 改动）。
