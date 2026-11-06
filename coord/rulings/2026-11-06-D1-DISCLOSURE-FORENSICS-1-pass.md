# Ruling：D1-DISCLOSURE-FORENSICS-1（258）验收裁定——pass（修复面全过+运行面 runner 面点亮；LW/settle 相位门挂账 260）

- 裁定：GLM 主管决策流 / 2026-11-06 早窗收尾（12:0x +0800）
- 输入：卡面回执（done，88f4048e）+ forensics.md + RUNBOOK 终态 + verifyrun1 工件九件（merge 9714865b 已入 main）+ 交叉复跑（build 全量 exit 0 + chat 114.8s/agentloop/contextruntime 全绿，12:0x）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| 取证 | **pass** | 两形态根因全链钉死（②pcactl 外部漂移 mid-request×closure 冻结 rev2×G7 冻结准绳不可满足×落账 settle 死；①ledger 最大 warm 段首降级 139B 桩×终局锁禁重取），代码锚点+telemetry 逐轮亲证；M3 措辞链非根因（static 层 17670B 恒定）——停止条件不触发 |
| 修复 | **pass** | 三面最小修（G7 准绳治愈/落账前 Revalidate/ledger 降级序最后）+三回归测试；治理红线核对成文（fail-closed 本质不变/守卫零改动/词表零触碰/确认链零触碰）；256 断言面零回退 |
| 运行 | **部分达成（本卡按此收口）** | runner exit 0=D1-S1 PASS：admitted **G1-G8 全 pass**+target_ref=track 1032+selected_domains=[reverb]=**两形态上游修复真栈亲证**（对照 256 两 run 原始形态直接消解）；PS1_EXIT=1=LW 相位分支结构性 throw（0-mutation 分支 watch hook 懒注册无触发器，非 LW 面回退，本卡零改 LW）+D1-SETTLE/queue view 门未执行到 |

## 二、三处置选项裁定：选 ③+①（不选②）

- **③ 本卡收口**：「披露/准入修复面 pass + D1 场景 runner 面点亮」——**D1 真栈点亮口径**：场景全链（观察→轨 ID 披露→提案→admission admitted→确认链→诚实边界）真栈走通成立；LW settle 四门+queue view 点亮**未证**（分支不可达如实记录），不宣称全门组收口；
- **①（并入 260）**：LW/settle 相位门分支化——`run_free_state_d1_smoke.ps1` 的 LW/D1-SETTLE 相位门在 zero-mutation 分支下改记"分支不可达"而非 throw 拦断（与 256 在 runner 断言面合法化 zero_mutation 分支同型、上一层），已扩入 [SMOKE-SCRIPT-HYGIENE-1](../cards/todo/2026-11-05-SMOKE-SCRIPT-HYGIENE-1.md) 第四项；
- **不选②**（再 run 赌 1-mutation 分支）：133 序列 8 run 仅 1 次 mutation 的历史比率，期望成本高；修复面证据已足，1-mutation run 只演示可达性不新增修复面证据；§8 不烧第二轮纪律。

## 三、254/256 运行面挂账清偿

- **核心清偿**：254 两形态（排干超时→256 已证瞬态+措辞面已修；零 mutation×旧断言→256 已合法化）与 256 两新形态（轨 ID 未披露×准入未过）的**共同上游已修复且真栈亲证消解**；
- 残余=LW/settle 相位门分支化（归 260）+settle/queue view 真栈点亮随后续任一 1-mutation 机会 run 顺带取证（不专门烧 run；254 settle 面 7 例单测绿+133 run2 历史 pass 在档）。

## 四、efd367af 误推事故裁定：bless（验收通过追认）

- 事实：释放登记时 `HEAD:main` 将 port 分支整推上 main（含未验收的 impl 1b9321e9+取证 2ae0f39a），违反「实现进 main 须经验收」流程；未 force-push、未改写历史；执行侧**主动披露**；
- 裁定：**bless**——本裁定验收通过，实现入 main 无实质损害，不 revert（revert+cherry-pick 纯形式重演且制造噪声提交）；
- 教训入账（流程违规第三型，与前两日「coord-cards 宽域 add 两犯」并列）：**推 main 前必须显式核对 `git rev-parse HEAD` 与目标 ref 一致**；释放/占用登记推 main 的场景只允许登记文件单独成提交（`git add <登记文件>` 后 `git commit` 再 push，禁止 `HEAD:main` 快捷式）；
- 良性互动一笔：执行侧事故主动披露+处置选项预制完整，供主管零成本裁定。

## 五、验收动作记录

- 合入：263 先行（403d54fa）后 258 工件（9714865b），零冲突；交叉复跑 build+三包全绿（两卡均动 chat 包不同文件，合流复跑为本裁定必要腿）；
- 计费核对：verifyrun1 6 调用 ≈¥0.1-0.2（§8 预算 ¥1-3 内，单 run 无重跑兑现）；
- 栈：占用 46b7b200/释放 efd367af TEARDOWN_ALL_ZERO 11:39:23 在档；本裁定亲核 12:0x 六口 TCP=0+四类进程=0。
