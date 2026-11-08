# PLUGIN-CONTROL-EVAL-SEAM2-1：A-pure 第二组缝（S5 工具目录裁剪+S2b 渲染面剥离）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[307 两项装置发现](../done/2026-11-07-PLUGIN-CONTROL-EVAL-S1A-1.md)——①A 臂菜单仍含类型化适配器（plugin_grabber_inspect_*/apply_*_controls 族，10/18 轮被模型自选）②contextruntime 结果渲染面（context.go:515-533）把内核 binding_targets 语义行（alias/normalized_role/template_role）回灌模型上下文，S2 摄取剥离不覆盖）
- 派发确认：已确认（主管发卡·装置类，305 同模式）；验收负责人：GLM 主管决策流
- 池序 308；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=305 已合入；L2 / PC；零栈

## 目标（两缝，并入既有单 env `VIT_PLUGIN_CONTROL_EVAL_SEAM`，默认 unset 字节不变）

1. **S5 工具目录裁剪**：缝开时 plugin_grabber 类型化适配器族（inspect_*/apply_*_controls/explain_controls 等语义适配工具）从模型工具目录裁剪——**裸参数工具必须保留**（plugin.get_parameters/set_plugin_param/get_plugin_parameters/通用读写回读）；307 工件 write_path 清单为裁剪面蓝本；
2. **S2b 渲染面剥离**：contextruntime context.go:515-533 的内核结果渲染路径下，binding_targets 行的 alias/normalized_role/template_role 语义键剥离（原生参数行保留）——与既有 S2 摄取面互补成全覆盖；
3. 配对单测：S5 缝开=目录无类型化适配器+裸工具在场；S2b 缝开=渲染结果无语义键+受染两轮形态（307 工件 T3_r1 payload 为负锚）复钉；默认 unset 字节不变三态；
4. 回归：四包全绿+全包兜底；与 305 既有 16 测不冲突。

## 约束
- 文件域=tools/catalog.go 工具目录面+contextruntime/context.go 渲染面±测试；生产默认不变红线；不触 300/307 scripts。
- 禁止 git reset --hard / git clean。

## 停止条件
裁剪面与在案工具治理裁定冲突→取证上交。

## 领取/回执/验收
- （执行侧回填）/（执行侧回填）/（主管回填）
