# 验收裁定：SMOKE-GUIDANCE-CHECK-DISCRIM-1（252）——pass 合入

- 裁定：GLM 主管决策流 / 2026-11-05 19:4x +0800
- 对象：done 提交 62548aec；impl 2135d9df+工件 6cfef43a（随本裁定 cherry-pick 入 main：380095c4+ceab71f9）
- 结论：**pass 合入**——guidance 检查三态化达成，251 揭示的结构性误报闭环。

## 一、工件核验（决策侧亲核）

- **实现面**：classify_telemetry_sources 按 source 归族（message_loop* / *semantic_entry* / other），**fail-closed 设计**（rows>0 且全 semantic_entry 族才判全仲裁；unknown/无 source 计 other 不作证据）✓；check_prompt_guidance 三态=①ok ②needle 缺席维持 fail ③文件缺失+全仲裁=n/a；非③态 summary 片段与 stop-condition-2 决策**与改前逐字节一致**；ps1 零改动 ✓；
- **亲复跑**：py_compile 0+unittest 11/11 OK（scripts/ 目录正确调用；含改前等价枚举断言）✓；对照驱动工件亲读（改前模块 76305d87 提取件 vs 候选：16 组合 15 same/1 intended_change/0 regression，exit 0；intended=251 归档全仲裁 fixture→n/a+telemetry 证明 13/13 semantic_entry）✓；
- **边界**：真栈复验按卡面挂下次全仲裁 run 搭车（本卡既定，不另起栈）——253 armrun1 非全仲裁形态已隐式回归（prompt_guidance_rendered=True 正常路径）。

## 二、处置

- 执行侧「实现不随 done 入 main、待验收 cherry-pick」的保守做法合规且干净（避免与 main 卡面提交重复）——入册为先例；
- 251 揭示的双重盲区（文件缺失≠装配缺陷不可区分/全仲裁形态指导缺席）就此闭环：全仲裁 run 不再误报 exit 2/1。

## 三、链状态

M3 链 229→253 全线收口（组 1 稳定绿/组 2 判别定案/组 3 修复转绿）；验证面盲区修复到位——后续全仲裁场景 run 的判定面可信。
