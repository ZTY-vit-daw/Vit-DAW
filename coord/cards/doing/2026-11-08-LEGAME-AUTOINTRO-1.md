# LEGAME-AUTOINTRO-1：进入工程自动开场+重进恢复播报（L1-b）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[313 设计稿 §2.2]+用户首验核心诉求「进入即自动开场」）
- 派发确认：已确认（主管发卡，L1 三件组）；验收负责人：GLM 主管决策流
- 池序 315；目标仓库=主仓 agent+scripts；P1 / 1 天 / **依赖=314（布位段同改）**；L2 / PC；零栈先行+真栈腿候队列

## 目标（照设计稿 §2.2 执行）

1. 工程↔level 绑定元数据（binding sidecar 形态照设计稿）+activation/restore 完成后自动开场注入；**幂等语义=开场工程级一次性/恢复会话级播报**；只在「binding 在案∧无活跃局」触发（不触挂起语义）；
2. chat/edu_jam_chat.go ~120 行+server.go handleChat ~15 行插点+布位脚本 binding 段；
3. 单测=binding×会话四象限+幂等+F5 竞态回归（281-B2 教训）；真栈腿=白话触发开场→重进恢复播报→exit 0；GUI 最后一米边界声明。

## 约束
文件域=chat/edu_jam_chat.go+chat/server.go 插点±测试+scripts 布位段；与 316 布位段协调（同 owner 顺序执行）；禁动 312 域。

## 领取/回执/验收

### 领取

- 2026-11-08 21:11 +0800 / origin/main `684d8367`（≥发卡基线 `699d486d`；含本组 314 领取+段1 回执两笔在卷）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 314-316 legame L1 组，同 owner 顺序 314→315→316）/ 分支 `port/legame-l1-1` / worktree `D:/Vit_DAW_wt_314l1`
- 领取基线：HEAD=`bdf2e0fc`（=314 实现提交，依赖兑现起点）；`git status --short` 空；`git diff --stat` 空（净树续卡）
- 段划分：段 1=实现+单测零栈先行；真栈腿候队列（315 真栈腿含 316 重铺后再开场验收，照设计稿 §6 顺序）

### 回执

- （执行侧回填）

### 验收

- （主管回填）
