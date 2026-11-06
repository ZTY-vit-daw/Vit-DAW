# D3-REF-ALIGN-THIN-1：D3 参考曲对齐薄层落地（第一档=内置参考标准；复用 RLM）

- 发卡：GLM 主管决策流 / 2026-11-06 晚窗，**2026-11-07 凌晨范围修订**（依据=[用户澄清 D3 原始意图]+[PD-2 关闭记录](../../PENDING-DECISIONS.md)：第一档=内置参考标准对齐，无渠道依赖；第二档=自定义参考音频上传渠道列 backlog 小卡；audition.* 试听命令族维持缺口登记与 D3 无关——勘察报告 [2026-11-06-d3-audition-commands-recon.md](../../reports/2026-11-06-d3-audition-commands-recon.md) 留档）
- 派发确认：已确认（主管发卡+用户澄清修订）
- 验收负责人：GLM 主管决策流
- 池序 266；目标仓库=主仓 agent；P2 / 1 天 / 依赖=D2 已收口（runtime 模式先例在库）；L1-L2 / PC 执行会话；验证腿需真栈一次

## 背景

D3=参考曲对齐（not_built）：观察面已建（M1-4：RLM reference_level_model.projection.v0 复用 B1 同款 Build+user intent+RenderBindings 披露）。runtime 未建。**范围裁定（2026-11-07）**：第一档=对齐目标用 RLM 内置交付预设（builtin:spotify 等——254 的 loudnesswatch builtin 同源）+投影差距判据+建议挂确认卡；自定义参考音频上传（track.add_audio 导入+导入期 loudness+custom profile 先例拼装）=第二档另卡。

## 目标

1. **判据+验证器（零 LLM 单测先行）**：RLM 参考电平投影（builtin 预设）+user intent→对齐差距判定纯函数（verdict 三态同 D2 模式：aligned/gap_found/unmeasurable——未绑定参考标准/投影缺数据=unmeasurable）；
2. **薄 runtime**（模式同 D2 卡：verdict 落 store→建议挂确认链→queue view D3 点亮；入口识别先测裸话术过 semantic_entry）；**动作面参照 D2 先例**——若确认链现有操作无法改变对齐差距（如纯电平类建议可走既有 gain 动作则挂卡，否则 v1 纯披露+申报）；
3. **验证腿**（真栈单 run ≈¥1-3，§8 成文）；指引面不在本卡。

## 约束

- 文件域=chat D3 runtime 新文件+判据纯函数+测试+冒测加参；**零 catalog 扩面**（audition.* 不碰，第二档上传渠道不在本卡）；禁改 RLM 投影本体（消费不改写）；禁重型 stage 机；治理红线不变；持久化新字段按 §11 缺省语义。

## 停止条件

- RLM 投影字段不支撑差距判定→取证上交转 M1/RLM 系卡；验证腿断言失败取证不重跑。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
