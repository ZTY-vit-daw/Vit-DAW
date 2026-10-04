# Ruling：SMOKE-SCEN-RANGE-1 —— pass（A 场景判据达成；B 场景停止条件正确触发，移交修复卡）（2026-10-04 晚，PC 决策侧）

## 裁定

**pass**（A 部分）+**B 部分停止条件正确触发**（卡面停止条款设计使然——证据推翻判据即上交，不猜测不凑口径）。B 场景承接移交新卡 INTENT-WIRE-FIX-1。

## 验收依据

1. **场景 A（判据=REGION-TIME-1 转正）达成**：
   - 执行侧两轮全过（210017/210105）：三确定性断言面（note_sessions 投影双会话行+store 时间键持久化往返+遥测 char_count 差 delta=230 跨进程稳定）+legacy fail-open 零报错；泊位纪律良好（每 run 独立目录、起拆即净 log 证实、预检拒绝复用已监听栈）。
   - 决策侧复跑（decide_rerun2，-StartKernel 泊位）：**断言①投影✓②store 时间键往返✓**；断言③遥测在决策侧环境未复现（空 JSONL，env 继承细节差异，记录在案）——但其证明目标（time_digest 注入 LLM prompt 并被使用）已被**更强的直接证据替代**：v3.1 回复明确答出 `Time span: 0:03.2–0:08.5`+逐 clip 相交段；legacy 回复如实声明"辖区未给出范围边界"（fail-open 不伪造）。两回复对比=注入语义的真栈真 LLM 端到端实证，强于 char_count 差值。
   - 复跑过程记录：首轮漏 -StartKernel 致就绪门正确拒绝（复核确认脚本行为正确）；遥测差异不构成产品缺陷。
2. **场景 B 停止条件处置**：证据链（同二进制三轮：210105 意图层端到端正确 tool-form 3.5→2.0+反例零提案；210955/211446 模型包络 3/3 抢先且顺序随机、同 run 漂移）推翻"exit 0 判据可交付"前提；核心缺陷定性成立——意图层坐 `server.go:2630` 兜底位（模型自吐即不接管）+agentloop 生产路径不经过意图层。执行侧四选项采纳 **a**（合成提到 LLM 之前），扩为两腿修复卡；c（概率口径）违反 §8、d（回退手测）手测同面受胁，均驳回。

## 处置

- 本卡归档 done（A 交付+B 停止条款正确行使）；SMOKE 场景模式与泊位设施（-Scenario 参数/就绪门/起拆净）为 JOURNEY-1 直接复用资产。
- REGION-TIME-1 随本裁定转正（另发 ruling）。
- INTENT-WIRE-1 维持 conditional，转正条件修订为：INTENT-WIRE-FIX-1 合入+场景 B 严格判据（tool-form 确定性）exit 0。
- 立卡 INTENT-WIRE-FIX-1（P1）。
