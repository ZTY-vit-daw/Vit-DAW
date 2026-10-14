# 2026-10-14 批量五裁定（115 验收 + 109 rerun6 解锁）

> 验收人：GLM 主管决策流（ZCode 主会话）。触发：用户"任务卡已完成，可以检查"。
> 方法：diff 亲读（seam 谓词/置位/守卫零改动逐核）+谓词语义亲读+95 包独立亲跑+gofmt blob 核；main 起点=d6204359。

## ① 卡 115 FIX-SETTLE-PARK-REPLAY-RACE-1 —— **pass，合入 5ad6b797**

- **seam 取证采信**：候选(b) 会使非停泊路径旧谱系观察从 settle 变 skip（违反硬约束②）、候选(c) 连坐 frontier/任务态投影/协议修复记账——排除论证与代码相符；候选(a) 与 SETTLE-DELIVER-1 症状 B 先例同构（修回放喂入面不动守卫），唯一同时满足三硬约束。
- diff 亲读：`parkedAtJudgmentBoundary=freeStateJudgmentBoundary(loop)`（谓词族与 :798 FS-SETTLE-TERMINAL-1 延迟同源，语义亲读=UserJudgmentRequested 且证据未回=停泊中；settled/stopped 不保护）+跳过条件**精确不等、逐字镜像 driver.go:182 守卫触发谓词**（含双方非空守卫，不吞空修订形态）+置于 dedup/证据顶之前（key 前移，原位删除无重复计算）；守卫与 audioclosure 零改动亲核；停泊态只跳过不采纳。
- 单测三钉亲核在位（①停泊回放跳过+停泊存活②continuation 窗口零 machine settle=不变量③非停泊守卫语义原样），回执红绿验证（stash 还原基线复跑）采信。
- 独立亲跑：worktree fsprr1 谱系 `go test ./... -count=1` **exit 0，95 包全 ok**；gofmt blob 级两文件净。
- 端测边界=纯单测（卡面约定），真栈链路验证归 109 rerun6。

## ② 109 rerun6 解锁

- 裁定六既定条件满足（115 验收合入）：**rerun6 可发**——flash 思考档 2 轮，worktree fsdc1r2、分支自当前 main 建；判据不变：exit 0=三笔挂账同销+M2 体验场次解锁（裁定六梯子例外继续有效：若仍同签名 2/2，先取证排除产品面再议 v4-pro）；核验锚点=停泊存活过 2s 轮询窗口（115 修复点）+既有三锚点沿用。
- 三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）维持未销至 rerun6。

## 工件

- 合入：5ad6b797（cherry-pick -x a4bcdf33）；worktree fsprr1 拆除（本批执行），分支 origin 留档。
- 亲跑工件：/tmp/gotest_115.log（95 包 exit 0）。
