# M3-OBS-EVIDENCE-FIXTURE-1：229 验收门承接——观察证据面修复（测试工程补确定性音频 clip+capability_blocked 判定归类，主管选项裁定 A+C）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=229 重验腿新根因定案+主管验收裁定：裸轨工程 acoustic package missing→派生观察源证据缺失→模型 capability_blocked 诚实停边——模型行为正确，测试工程形态与 FS 准入语义的结构性交互问题；**A 采（fixture 补 clip）+C 采（判定归类）+B 否（FS 准入门不为测试便利弱化）**，裁定全文见 229 卡验收节）
- 派发确认：待确认（229 验收转出，终态架构阶段②收口卡；发卡即提交，领取按池序）
- 验收负责人：GLM 主管决策流
- 池序 230；目标仓库=**主仓 scripts 域**；P1 / 0.5 天 / 依赖=229 已合入（8954b8e1）✓；L1-L2 / PC 执行会话

## 标的（229 定案，不重考）

220 断言体 setup=两根裸轨（无音频 clip）→acoustic package 状态 missing（.vit_derived 亲证，229 run evidence/ 摘录在卷）→频段动态/电平 levels/多轨关系观察全部不可用→模型合规停在 FS 准入门（r3 capability_blocked 申报原文：「无法确认一个具体候选目标…要继续，需要先恢复该轨道可用的电平或频段观察数据」）。229 验收门（组 1 no_route 消失+set_points 端到端首绿）在裸轨 fixture 上结构性不可达。

## 目标

1. **fixture 补音频 clip（选项 A）**：勘察内核 clip 导入命令面（add_audio_track 已知在场；clip 导入/生成确定性途径勘察定）——setup 工程为两轨补确定性音频内容（脚本生成 wav 导入，或既有样例 clip 复制到 run 目录隔离工作区，§10 权威输入防污染纪律）。验证链逐环亲证：clip 在场→.vit_derived acoustic package 状态 available→轨道级观察返回事实数据。**断言逻辑原口径不动**（三组×5 轮、组级 ≥4/5、五分类语义、同型三连止损全部承 220；改的只是 setup 工程形态）。
2. **判定归类（选项 C，主管授权改判定面）**：模型 capability_blocked 诚实申报不再落 no_route 词表漏配——勘察 220 断言体判定段后定最小改法（五分类新增 capability_blocked 型或并入既有最贴近型的归类裁定，成文记录）；既有四型分类语义零回退。
3. **229 验收门重验（本卡验收门）**：组 1「这段让它越来越突出一点」先行——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。

## 排除面（主管裁定，卡面禁改）

- **FS 准入门（G3/G5）语义零触碰（选项 B 否决）**：空轨/无音频工程上模型诚实停边=终态架构期望行为；证据准入纪律不为测试便利弱化。「空轨可写」若为产品需求，属产品级设计讨论另立裁定。
- **agentloop/agent 生产面零改动**：本卡全部在 scripts 域（+必要时 scripts 内测试帮手/素材生成）；若发现观察管线缺陷（补 clip 后 acoustic 仍 missing），越域即停转卡上交。

## 文件域

`scripts/automation_llm_route_smoke.py`（setup 段+判定分类段）+run 目录内自足的 clip 素材生成/复制+必要时 scripts 帮手。越域即停上交。

## 停止条件

- 内核/agent 无确定性 clip 导入途径→锚点+方案上交（不改内核迁就）；
- 补 clip 后 acoustic package 仍 missing→观察管线缺陷锚点上交转卡（非本卡域）；
- 证据齐备（clip+观察事实在场）后组 1 仍三连 no_route→模型能力面锚点上交（229 停止条件 3 承接）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

（待领取：记录 HEAD/status/diff --stat；真栈错峰登记 PC-RUNTIME-STACK.md；§8 计费披露）
