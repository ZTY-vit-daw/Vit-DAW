# LEGAME-FRESH-1：fresh 工程重铺腿+预演副本制（L1-c）

- 发卡：GLM 主管决策流 / 2026-11-08 深夜（依据=[313 设计稿 §2.3]+事故账违例三「预演污染用户工程」）
- 派发确认：已确认（主管发卡，L1 三件组）；验收负责人：GLM 主管决策流
- 池序 316；目标仓库=主仓 scripts+coord/transfer；P2 / 0.5 天 / 与 315 布位段协调（同 owner 315→316 顺序）；L1-L2 / PC；零栈

## 目标（照设计稿 §2.3 执行）

1. 布位脚本扩展=重铺腿（幂等两连跑）+预演副本制（预演在副本工程上跑，哈希断言目标工程零触碰）+交付检查单三问模板入 coord/transfer/；
2. 零 agent 源码；脚本域验收=重铺幂等两连跑+副本制哈希断言；真栈腿并入 315（重铺后再开场）。

## 约束
文件域=scripts 布位脚本+coord/transfer 模板；零 agent 源码。

## 领取/回执/验收

### 领取

- 2026-11-08 21:21 +0800 / origin/main `cdf2f5eb`（含 314/315 领取+段1 回执四笔在卷）/ owner=ZCode GLM-5.3 L2-worker PC 执行子代理（会话 314-316 legame L1 组，同 owner 顺序 314→315→316）/ 分支 `port/legame-l1-1` / worktree `D:/Vit_DAW_wt_314l1`
- 领取基线：HEAD=`cd5aa5b0`（=315 实现提交）；`git status --short` 空；`git diff --stat` 空（净树续卡）
- 段划分：零栈（脚本域+模板域）；真栈验收并入 315 真栈腿（重铺后再开场，设计稿 §2.3/§6）

### 回执

### 回执（2026-11-08 21:26，commit `c34aed13` @ `port/legame-l1-1` 已推送，零栈整卡）

- **diff 清单**（4 文件，+231/−4，零 agent 源码）：
  - `scripts/legame_placement.py`（+205）：`relay` 重铺腿——删目标 `.vit`+工作区兄弟目录（`.vit_agent/`/`.vit_history/`/`<名>_Media`）→ 重铺纯净模板 → binding 重置（保留既有 bound_at/bound_source 溯源 → 连跑字节恒等）→ 回读核对（工程 sha256==模板 / `.vit_agent` 仅含 binding 且 opened_at=null / `.vit_history` 与 `_Media` 不存在）；`place-copy` 预演副本腿——同模板布位到隔离路径+独立 binding，命令不引用目标工程路径（read-only reference 语义内建）；模板==目标自反拒绝 exit 2
  - `coord/fixtures/legame/手测·小星星乐局.vit`（新，12811B）+`FIXTURE.md`：重铺纯净模板——自 311 布位终态一次性快照（源文件 mtime 2026-11-08 19:45:31=prep1 run 窗口亲证其后零改动；内容形状=prep1 真栈回读在卷：3 用户轨 theme 42/corrupted 42·beat37=pitch68·mute/空白伴奏；SHA256 `e0e95338…2441e3` 入档）。**域扩展声明**：`coord/fixtures/` 为本卡新增目录（卡面域=scripts 布位脚本+coord/transfer 模板）——布位纯净模板是重铺腿的必要工件，归布位面；请验收裁定
  - `coord/transfer/2026-11-08-M2-handtest-steps.md`：§〇 增补——自动开场说明（315 兑现后 A1 改任意白话触发）+重铺腿执行命令+**交付检查单三问**（重铺 exit 0 在卷[附记录位]/目标工程哈希==重铺终态/会话残留零）+**预演副本制条款**（预演/彩排/烟测禁在目标工程实跑，place-copy 到隔离目录）；旅程 B 前置保留并注 L1-d 裁定点归卡 4
- **脚本域验收亲验**（零栈，本地 temp 沙箱全路径）：relay 污染态（opened binding+agent 残留+history+media）重铺 exit 0 → 残留零+bound_at 保留+opened 重置；**两连跑恒等**（工程哈希+binding 字节全等）；place-copy exit 0 且目标哈希前后不变；自反拒绝 exit 2
- **真栈验收并入 315 真栈腿**（设计稿 §2.3/§6：重铺→开场→重铺→再开场，验证重铺后 binding/会话残留归零+自动开场可再次触发=「新工程」语义自证）——候栈（317 占用登记 21:15 在卷）
- **端测边界声明**：零栈卡无独立真栈腿；内核级回读（clips 42/42+recent 首项）归并入的 315 真栈腿断言面

#### 真栈验收补记（并入 315 段 2 腿，2026-11-09 00:0x）

- **relay 真命令对真会话污染态实证 exit 0**：runB 会话后副本工作区=.vit_agent{binding(opened)+vitproj 工作区}+.vit_history 会话记录 → `relay` → 重铺三问全过（①relay summary exit_code=0 在卷；②工程 SHA256==e0e95338…2441e3 模板终态；③残留零：.vit_agent 仅 legame_binding.json[opened_at=null]+.vit_history 不存在）；bound_at 溯源跨重铺保留（连跑字节恒等的零栈结论在真栈得到结构确认）。
- **runC：重铺后再开场 PS1/py exit 0**——新 agent 进程白话自动开场再次触发（=重铺即「新工程」语义自证）；%APPDATA% 用户目标全程零接触（runB/runC 目标哈希断言）——**预演副本制全链兑现**（预演/烟测全部在隔离副本上跑，目标仅只读参照）。
- 工件：`coord/runs/LEGAME-L1-STACK-1/`（evidence.md 索引；runB relay_summary.json 在卷）。整卡自验完成（零栈脚本域+真栈 relay/runC），mv done/ 待主管终验。

### 验收

- （主管回填）
