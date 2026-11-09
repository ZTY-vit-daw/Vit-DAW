# 309 WAVESHELL-OFFRENDER-FIX-1 栈腿转交提示词（M1+插桩+判别，2026-11-09 发）

> 用法：整段复制到一个新执行会话（ZCode L2-worker / PC）。该会话按 [coord/PROTOCOL.md](../PROTOCOL.md) 领卡续做。

---

你是 Vit-DAW 仓库 309 卡（WAVESHELL-OFFRENDER-FIX-1）的执行会话，续做**栈腿（M1 实时对照 + 插桩遥测 + 判别实验 + 修法落地 + 验证）**。取证腿已验收 pass，你从五项裁定给定的形态继续。

## 必读（按序）

1. 卡面：`coord/cards/doing/2026-11-07-WAVESHELL-OFFRENDER-FIX-1.md`（目标/约束/停止条件 + 取证腿验收 + 五项裁定全文）
2. 取证主交付件：`D:/Vit_DAW_wt_309recon/coord/runs/WAVESHELL-OFFRENDER-FIX-1/recon-design.md` + PROGRESS.md
3. 协议：`coord/PROTOCOL.md` §0/§2；栈占用登记：`coord/resources/PC-RUNTIME-STACK.md`（净空亲核→登记→≤60s 复核→起栈→TEARDOWN_ALL_ZERO→释放）

## 工作形态（五项裁定已锁，2026-11-08 主管）

- ① 一次栈窗口合并 M1+插桩+判别（插桩内核=301 CC9FDACC + VIT309 遥测）；
- ② **修法 B 先行**（VitApp-only 黑名单 + realTimeRender=true；M1=RT-SOUND 即落地）+ **C 为判别实验原型**（§4.2 临时一致化：一次实验同时判别 S1/S2 + 验证 C；S1 证实且渲染限速不可接受时 C 转正）+ A 不单用；
- ③ 自检断言=形态 3（渲染后静音断言 + 自动降级 realTimeRender 重渲 + `degraded_to_realtime_render` 披露 + shell 类 hint 日志）；
- ④ 插桩落点：VIT309 mode 遥测→vendored JUCE Impl（engine 子模块独立提交，301 处置规则同款）；VIT309_RTLEVEL→VitApp（零 engine 改动优先）；
- ⑤ M1=RT-SILENT 应急预案：B 降级纯警告 + 装载取证另立卡。

## 执行要点

- worktree/分支续用：`port/waveshell-offrender-fix-1`（取证腿已推远端 0644ae89）；
- 领取时记录：`git rev-parse HEAD` / `git status --short` / `git diff --stat`（区分本卡新增 diff）；
- C++ 构建走 301 同款 cmake 配置；插桩留痕保留；
- 验证矩阵：M0a2 场景复跑出声（RMS 非零）+ M3 Surge XT 不回退 + 全包回归；
- §8 概率性声明照旧：写明运行次数、成功条件、失败分类、止损线；同一确定性断点连续两败停手上交。

## 停止条件（卡面原文）

根因进入 WaveShell 闭源二进制不可修面 → 取证上交转「黑名单+替代路径」纯策略方案裁定。

## 回执义务

栈协议两条登记、完整命令与退出码、run ID、HEAD 与工作树状态、工件目录（新 run 不覆盖旧 run）、端测边界声明（AGENTS §5）。done 只代表自验完成，主管验收后才算收口。
