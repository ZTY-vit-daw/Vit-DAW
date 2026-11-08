# PLUGIN-CONTROL-EVAL-APURE-1：A-pure 终测（真纯裸参数臂 18 轮——「越过控制脚本」正式答案）

- 发卡：GLM 主管决策流 / 2026-11-08 早窗（依据=[307 验收裁定](../done/2026-11-07-PLUGIN-CONTROL-EVAL-S1A-1.md)——A 臂实义=「自选工具面」非纯裸[typed 适配器 10/18 轮自选]+渲染面泄露两轮→S5/S2b 开卡 308；[昨晚 gate 首排](../../reports/2026-11-07-gate-3.md) §七——A-pure=用户核心问题的终测，同时决定开放录音棚适配器形态）
- 派发确认：已确认（主管发卡·分段 go/no-go，307 同模式）；验收负责人：GLM 主管决策流
- 池序 310；目标仓库=主仓 scripts（复用冻结 runner）+工件；P1 / 0.5 天 / **依赖=308 合入 main**；L2 / PC；真栈占用（登记协议照旧）

## 目标

1. **A-pure 18 轮**：六案例 × 3 重复，deepseek-flash 单模型，`VIT_PLUGIN_CONTROL_EVAL_SEAM=1`（S1-S5+S2b 全生效）——模型只见**真纯裸参数面**：裸工具（plugin.get_parameters/set_plugin_param/get_plugin_parameters/通用读写回读）在场，typed 适配器族不在工具目录（S5），渲染面无 binding_targets 语义键（S2b），无语义字段/无 typed 路由/无骨架引导；
2. **agent 二进制=含 308 的现 main 现场构建**（领取时 fetch 重读；缝实现以 main 为准，不改动源码）；
3. **判定口径**：冻结 plan.json（runner_frozen 同源快照）A_star 段 caps 原样（18 rounds/216 calls/per_round_caps 沿用）；T4 联动对形态沿用 307 标注法（verdict 原样、分析层注联动对旗标）——冻结规则修正留 B/C 批次前（307 验收在案裁定，本卡不改判据）；
4. **装置验证门（task 轮前 probe + 语料级 seam_verify 双层）**：
   - S5 亲证：模型工具目录无 plugin_grabber 类型化适配器族+裸工具四件在场；
   - S2b 亲证：渲染面（contextruntime 结果渲染）无 alias/normalized_role/template_role——负锚=307 T3_r1 受染 payload（7 请求行形态）必须检不出同型；
   - **任何缝泄露（typed 工具入目录/语义键入上下文）=装置失效立即停段取证**；write_path 出现 typed_adapter=S5 失效同罪；
5. **回报**：结果落地即回报主管=B/C 段决策输入+开放录音棚适配器形态依据（「越过控制脚本」正式答案）。

## 约束

- 文件域=工件目录+卡面+编排脚本（arm_a_orchestrator.ps1 同模式复刻，307 工件在 `D:/Vit_DAW_wt_307s1a/coord/runs/PLUGIN-CONTROL-EVAL-S1A-1/` 只读参考——runner_frozen 三件+P 对照+编排+seam_verify 均可复用快照）；源码零改动；agent 源码只读；
- 真栈协议全链（净空亲核→登记→≤60s 复核→run→TEARDOWN_ALL_ZERO→释放登记）；工件新目录不覆盖；评价器期望不进模型上下文；栈排队：**领取后候栈——若 296-R2 在栈先等其释放**（登记前重读栈记录）。

## 停止条件

缝泄露（任何形态）→装置失效停段取证；同型环境失败两轮→止损；断言失败取证不重跑。

## 预算

A 段同款：plan.json A_star segment_caps（≤18 任务轮/≤216 调用/in≤8.64M 上界）；§8 跑前成文（307 实耗参考：全程 109 调用/1.87M in，估 ≈¥3-6，用户 deepseek-flash 账户）。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
