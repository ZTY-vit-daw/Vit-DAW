# D1-RUNNER-FORENSICS-1 verifyrun1 RUNBOOK（§8 跑前口径，验收腿）

- 2026-11-05 晚窗 / owner=ZCode GLM-5.3 L1 PC 执行会话（256 卡会话）
- 目的：修后单 run 重跑 **254 门组**（卡面验收腿）——LW 四门不回退+D1 settle 四门+queue view D1≠not_started，过则 D1 真栈点亮补全、254 运行面挂账清。

## 被测面

- 被测改动=`scripts/free_state_d1_smoke.py`（断言分支容忍，impl 3a52f640）；`-RepoRoot D:\Vit_DAW_wt_256impl`（worktree 检出即含修复）
- 被测 agent=`D:\Vit_DAW_wt_254impl\agent\bin\VitAgent.exe` SHA256 `F321AF5E268CE2C14FB83BDA15BE1C4F73083AFC3C3BC620299E486720C6E1AD`（不重建；合规前提亲证：`git diff --stat c6fbb7d6..HEAD -- agent/` 为空=当前 HEAD agent 代码与 armrun2 被测树逐字节同源，agent 面零改动）
- kernel=`D:\Vit_DAW_wt_228\VitAppuild_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A`（wt_228 只读复用，225/249/251/253/254 同款）
- 引擎=本机默认无注入（config `~/.vit/config.json` defaultModel=deepseek-flash 跑前亲读，同 armrun2）
- 命令=与 armrun2 逐字同参，仅 -RepoRoot 换 wt_256impl：`run_free_state_d1_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_256impl -PublicManifest <主检出 fixture> -PromptFlavor reverb -ExpectDomain reverb -LoudnessWatch -D1LoudnessSettle -SkipBuild -KernelExe <wt_228 显式>`

## 成功条件（254 门组）

1. PS1_EXIT=0（含 python runner exit 0：模型走 1-mutation 分支过主链 legacy 断言，或走 0-mutation 分支过新紧断言集——**两分支皆合法 PASS**）；
2. LW 四门不回退（①observe 披露 loudness_watch ②verdict store schema+round trigger ③dedupe ④gate4 config-off silent）；
3. D1 settle 四门（A stop_reason=d1_loudness_settled+观察面+零 mutation/B d1_settle verdict 落盘/C action face 按 verdict 分支/D queue view D1 node ∈ {in_progress, needs_confirmation}）；
4. 全程零崩溃、栈可拆净。

## 运行次数 / 预算 / 止损

- **单 run 无重跑**；deepseek-flash 预算 ≈¥1-3（armrun2 实测 10 调用 ≈¥0.2-0.3 基线，上界容轮数分支浮动）。
- 失败分类预写：
  ①模型再走 41 轮长链排干超时（armrun1 形态，窗口不动决策的已知残余风险）→取证上交不重跑；
  ②断言失败（新 0 分支断言/主链断言/LW/settle 门任一）→取证上交不重跑（卡面停止条件）；
  ③环境中断（端口冲突/进程残留/API 额度耗尽/LLM 服务中断）→按环境中断记录，至多重建栈 1 次续跑（同 run 预算内），再失败即上交。
- 止损线：本 run 失败即收口上交，不烧第二轮；不因偶然成功倒推验收标准。

## 占用与工件

- 占用登记：coord/resources/PC-RUNTIME-STACK.md（§2.2 全流程：净空亲核→登记推 main→重读确认→≤60s 即时复核→启栈→finally 拆栈核净→释放登记）
- 工件：coord/runs/D1-RUNNER-FORENSICS-1/verifyrun1/（随 port/d1-runner-forensics-1 入库）；大件留盘 worktree artifacts/

## verifyrun1 结果与 verifyrun2 加跑口径（跑前成文，2026-11-05 22:2x）

- **verifyrun1（stamp 20261105_222048）：exit 3 NOT_EXERCISED——门组未执行到（LW/settle 门在 runner exit 0 后才跑）**。形态取证：resp0=limit_reached「我还在继续处理」（与 133 run2 resp0 逐字同款）→continuation 进 fs4_diagnostic_round 观察（budget 6 用 1）→`terminal_causes=当前上下文未披露任何轨道标识，无法填写提案目标;选定候选轨的轨道级观察不在当前证据窗口内`→提案面未达→selected_domains=[]→exit 3。**本卡改动零触碰驱动循环（git diff 亲证=纯增量 120 行：validate_zero_mutation_boundary+分派 return+测试函数），运行时行为不可能受本卡影响**——此为披露/模型面随机分支第三形态（与 armrun1 排干、armrun2 零 mutation 边界并列），不落本 RUNBOOK 预写分类①②③（exit 3 是 NOT_EXERCISED 设计中性出口，非断言失败非环境中断）。
- **加跑 verifyrun2（最终轮）依据**：①AGENTS §8 总则=同一代码版本同一确定性断点**连续失败两次**后禁原样重跑——本形态首见未达两次；②133 卡先例=run1（20261018_124040 模型随机分支 materiality missing）重跑 run2 通过且终验采纳（rulings/2026-10-18-BATCH2-rulings.md §一）；③exit 3 中性出口≠断言失败，卡面停止条件「断言失败→不重跑」不适用。**verifyrun2 为最终轮：任何结果（PASS/not_exercised/断言失败/排干超时/环境中断重建后仍败）后即收口上交，不跑第三轮**。
- 栈处置：verifyrun1 未自拆栈（exit 3 路径无 teardown，同 armrun1/2 先例），owner 栈存活（VitAgent 44016/VitApp 13372）；verifyrun2 复用同 owner 栈（ps1 自带 Restart existing VitAgent），同占用窗口内连续 run，不重新登记。
- 预算：verifyrun1 计费待 telemetry 核（≈2-3 调用量级）；两轮合计上界仍 ≈¥1-3 内（armrun2 单轮 ¥0.2-0.3 基线）。

## verifyrun 终态（2026-11-05 22:33 收口，最终轮承诺兑现——不跑第三轮）

| run | stamp | 结果 | 形态 | 门组 |
|---|---|---|---|---|
| verifyrun1 | 20261105_222048 | **exit 3 NOT_EXERCISED** | resp0=limit_reached（与 133 run2 resp0 逐字同款）→continuation fs4_diagnostic_round（budget 6 用 1）→`terminal_causes=当前上下文未披露任何轨道标识，无法填写提案目标;选定候选轨的轨道级观察不在当前证据窗口内`→提案面未达→selected_domains=[] | 未执行到（LW/settle 门在 runner exit 0 后才跑） |
| verifyrun2 首次 | 20261105_222843 | **启动腿环境中断（零 LLM）** | verifyrun1 未自拆栈→kernel 13372 残留→dev_agent_smoke 5555 守卫（187×185 事故后禁静默复用）拦截→owner 拆净重建（RUNBOOK 预写③至多重建 1 次兑现） | 未起栈 |
| verifyrun2 最终 | 20261105_222912 | **exit 1 FAIL：`D1 loop projection is missing`** | **首响应即 capability_blocked**（reply=「有界改进提案未通过自由态准入门：证据新鲜度一项准入检查未过…没有执行任何实验或修改」mut=False）→零 loop 投影→find_d1_loop None | 未执行到 |

- **两 run 失败形态均不在本卡修复面**（修复=validate_d1 断言分支化，纯验证段增量 120 行零触驱动循环 git diff 亲证）：verifyrun1=披露面轨道标识缺失（模型/披露分支）；verifyrun2=证据新鲜度准入分支（首轮 capability_blocked，连 loop 投影都未形成——**无证据判定其为 D1 合法验收形态，不扩分支容忍**，该形态落「场景未行使/准入拒绝」族，与 armrun2 的 blocked+loop-在场形态不同）。同 fixture/话术/二进制（F321AF5E）/引擎（deepseek-flash）四变量恒定下，armrun2 走到提案+边界、verifyrun1 停 fs4、verifyrun2 首轮拒——模型行为分支分布的随机性亲证。
- **计费**：verifyrun1 ≈12-20 调用+verifyrun2 5 调用（agent log message_loop.llm 亲数，verifyrun1 含 fs4 观察轮内部调用），合计 ≈¥0.3-0.5，预算 ¥1-3 内。
- **TEARDOWN_ALL_ZERO 亲核 22:33:40**（五口 TCP 0+VitAgent/VitApp/vsphub/Godot 0+python 0；verifyrun2 首次中断后 22:28 亦拆净一次）。
- **结论：验收腿（254 门组重跑）未达成——两形态均未执行到门组面；本卡取证+修复面完成（离线闭环验证），运行面验收挂起上交决策侧**。处置选项：①择时再派单 run 验收腿（模型分支分布下 armrun2 形态可复现——其被本卡修复合法化后应 PASS）；②先派披露面取证卡（轨道标识披露窗口×证据新鲜度准入的分支条件——verifyrun1/2 两形态的共同上游）；③接受离线验证+armrun2 形态回放作为修复面证据，运行面门组点亮另行安排。
