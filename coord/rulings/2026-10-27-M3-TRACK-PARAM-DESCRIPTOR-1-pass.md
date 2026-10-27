# Ruling：M3-TRACK-PARAM-DESCRIPTOR-1（201）——pass

- 验收人：GLM 主管决策流 / 2026-10-27 下午
- 合入：port/m3-track-param-descriptor-1（e9a49fd0 实现+f6933ed4 工件）。

## 亲核面

1. **diff 亲读**（内核 +359/-2）：buildTrackLevelParameterDescriptors——恰 volume/pan 两参经 resolvePluginParameterByID 实解析（null 跳过=与 196 可寻址面镜像，master/trim/mute/gain 不虚列）；行 schema 与外部插件行逐字段对齐（id/name/value/normalized_value/value_text/min/max/is_discrete/is_boolean/role/group/quick_controls/signature/envelope_point_count）；单位/值域取自参数自身元数据（valueToString/display_probe 采样，停止条件②未触发——未造数据）；handler 仅 type=volume 放行，其余内置插件维持原界（Level Meter 等 error 保持）；envelope_point_count 复用 190 单点 appendEnvelopePointCountsToParameterDescriptors。
2. **判定面**：连续两有效产品轮全绿（R1+R2 ps1 exit 0；201 新腿 42/42×2+196 回归×2+190 回归×2 含外部插件行 diff 级零改动）；R2 首试 188 守卫活触发拦停留证（守卫正确工作；执行侧拆栈时序缺口自纠后干净重放，非产品轮如实分类）。同源对照（描述符 count==get_curve point_count）+跨轨隔离在卷。
3. **契约变更裁定遵守**：旧 error 仅对 type=volume 消失（B 案裁定面），消费方九面盘点零旧 error 语义硬依赖（agent 工具/观察/PCA/grabber/前端 rack 层/车道层/四冒测面）。
4. **栈纪律与工件**：登记/释放/即时复核在案（时戳偏差机器钟修正如实注记）；工件随 port 分支入库，主检出零未跟踪副本（§6.1 纪律首次达成）。

## 裁定

**整卡 pass**。质量优先口径达成（两轮稳定性+消费方全量回归）；202（FE-AUTOLANE-REBUILD-1）依赖解锁——其统一选择器消费本卡轨级行（volume 值域 0..1 fader 位/pan -1..1、单位经 value_text/display_probe，消费端按此映射）。
