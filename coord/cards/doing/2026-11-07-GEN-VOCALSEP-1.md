# GEN-VOCALSEP-1：生成产物人声分离后处理（用户 2026-11-07 听感裁定方向）

- 发卡：GLM 主管决策流 / 2026-11-07 深夜（依据=[用户三 wav 亲听裁定](../done/2026-11-05-GEN-YUE-DIALECT-1.md 听感旁证节)——「仅人声」模式声学上仍有稀疏配器（点状节奏+低音），但旋律/节奏/速度高度统一；**加入人声分离即可直接作人声用**）
- 派发确认：已确认（用户方向明示）；验收负责人：GLM 主管决策流
- 池序 306；目标仓库=主仓 agent（生成产物后处理链）+本地分离模型；P3 / 0.5-1 天 / 依赖=278 已合入（产物链在）+三 wav 样本在盘（wt_278impl 工件目录）；L1-L2 / PC；零真栈先行（分离模型本地跑），管线集成腿可选占栈一次

## 目标

1. **分离模型选型与落地**：本地人声分离（Demucs/MDX-Net 类，GPU 在场 VRAM 预算 ≤6.3GB 线内实测）——先对 278 三 wav（run4/5/6）离线分离出 vocal stem（+可选 instrumental residual）；
2. **质量面**：分离产物听感抽查（用户耳朵为主审）+客观指标（vocal stem 残余伴奏电平估计）；如实边界：分离质量模型依赖，不承诺棚级；
3. **管线集成（裁量）**：genapi yue-local 产物→分离→vocal stem 落 artifact 的可选后处理开关（默认关）；chat 话术「只取人声」类意图接线候后续卡；
4. §8 跑前成文（分离腿本地 GPU 零 LLM；墙钟/显存如实记）。

## 约束

- 文件域=scripts 分离驱动+工件+（裁量）genapi 后处理开关±测试；模型权重不入仓（artifact 区）；不触 278 已合入链本体行为（默认关）；
- 三 wav 样本只读；禁止 git reset --hard / git clean。

## 停止条件

本地分离模型在本机跑不动（显存/依赖墙）→取证上交转部署裁定；分离质量听感不过关→如实报告不强推集成。

## 领取/回执/验收

- 领取：2026-11-07 22:41 / origin/main=9180ff89 / owner=ZCode GLM-5.3 L2 执行子代理（PC，306 领卡会话）/ 分支 port/gen-vocalsep-1 / worktree D:/Vit_DAW_wt_306vsep / 领取基线：HEAD 9180ff89，status 全净，diff --stat 空；doing 三卡（ARRLOOP-AUTHBRIDGE-1/CLOSURE-ROUTE-ADAPT-1/PLUGIN-CONTROL-EVAL-S1A-1）文件域与本卡不相交已核对；零真栈先行，307 现持栈占用（GPU 腿无栈面冲突）
- 回执：（执行侧回填）
- 验收：（主管回填）
