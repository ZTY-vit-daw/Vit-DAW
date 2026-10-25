# SMOKE-STACK-GUARD-1（池序 188）回执——冒测脚本栈守卫 + GUI 场景腿工程隔离默认

- 执行 owner：GLM-5.3 L1 PC 执行会话（ZCode，188 卡会话）；分支 `port/smoke-stack-guard-1`；worktree `D:\Vit_DAW_wt_guard1`；领取基线 `aa06fca7`（领取登记 6009b37c）。
- 文件域：`scripts/dev_agent_smoke.ps1` 仅此一件（+本工件目录）。Go/前端零改动。
- 真栈占用：登记 73be5071（20:29）→ 启动前即时复核 20:46:39（距推送 9s）→ 本腿收口后拆栈复核（见 §五）。

## 一、实现摘要（diff 亲读：136 行纯增量，零删改既有行）

1. **守卫①（already-listening 不静默复用）**：新增 `Assert-StackPortOwnership` 函数 + `-AllowForeignStack` 显式放行开关（默认关）。插入点两处：内核启动步 else 分支（端口被占且占用 pid ≠ 本 run 自启内核 → throw，指引文案指向 `coord/resources/PC-RUNTIME-STACK.md` §2.2 与属主确认）、agent "Start or reuse" else 分支（同款；附带堵住非 `-Strict` 时 restart 后 Fail-Or-Warn 仅 warn 继续复用外来的漏洞）。本 run 自启 pid 匹配放行（防御性）；journey/scenario 模式既有 preflight throw 不动。
2. **守卫②（GUI 场景腿工程隔离默认）**：`SendRouteGuiSmoke`/`AutomationDatachanGuiSmoke`（非 scenario 模式）且 `VIT_PROJECT_XML` 未显式传入时，默认将其设为 run 目录隔离副本（`git show HEAD:VitApp/Workspace/default_project.xml` 经 cmd 原生重定向字节级拷贝，XML 可解析校验；`-RunArtifactsDir` 优先、否则 `VitApp\Workspace\Artifacts\smoke\gui_project_isolation_<ts>`）；显式传入优先（只打 ok 行不改值）；journey berth 既有隔离逻辑零触碰（本块 `-not $ScenarioMode` 排除）。腿完成后 `Write-GuiLegProjectIsolationPost` 刷新 marker（187 二轮同款证据形态：副本路径+被写时间戳）。

## 二、守卫①证据（工件=本目录 t1/t2b/t4/t5，console+摘要）

| 场景 | 结果 | 文案 |
|---|---|---|
| T1 假监听 7878（agent 端口），`-SkipBuild` | exit=1 ✓中止 | "not a process this run started (agent start step)"+PC-RUNTIME-STACK 指引在场 |
| T2b 假监听 5555+7878（内核端口），`-SkipBuild -StartKernel` | exit=1 ✓中止 | kernel start step 守卫+指引在场；假监听拆净（listeners_left=0） |
| T4 真栈占用（T3 自启 kernel+agent 留存），`-SkipBuild -StartKernel` 无放行 | exit=1 ✓中止 | kernel 步守卫先触发——185 事故同款形态被显式拦下 |
| T5 同 T4 + `-AllowForeignStack` | **exit=0** ✓复用路径走通 | "reusing a FOREIGN kernel/agent"双 warn 在场；默认腿全族完成（"dev agent smoke completed"） |

T2 首跑（driver 内）exit=1 但死于 agent 二进制缺失前置检查（T3 build 之前 worktree 无 exe），未达守卫——非守卫缺陷，由 T2b 双假监听形态补证。

## 三、守卫②证据（工件=t7_gui_isolation/ 一轮 + t7b_gui_isolation/ 二轮收口）

命令（两轮同形）：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -SkipBuild -AutomationDatachanGuiSmoke -RunArtifactsDir <run 目录> -NoChatSmoke -NoStripSilenceSmoke -KernelExe 主检出 K3`。

- **二轮（t7b，验收主证据）**：`exitcode.txt`=**0**（硬退出码）；console.log（合并 UTF-8）13 个 ok 行——含默认腿族全过（HTTP state/Strip Silence catalog/Range context/Kernel ports/Project state，即干净端口回归面）+ 隔离 ok 行 `VIT_PROJECT_XML=<t7b>/default_project.xml (git-clean run-dir copy)` + "automation datachan GUI smoke passed" + 隔离 post 证据行（副本被写 UTC 21:14:08）+ "== Summary / ok: dev agent smoke completed"。
- **marker `gui_project_isolation.json`**（两轮各一）：created_at → **isolated_copy_written_at**（探针写工程后内核写副本）→ leg_finished_at 时间线自洽=187 二轮同款证据形态；source=`git HEAD aa06fca7:VitApp/Workspace/default_project.xml`。
- **主检出零触碰（两轮）**：`VitApp\Workspace\default_project.xml` 跑前 sha256 `1FEF9CE9…424BE`/mtime 19:39:01 → 一轮、二轮跑后均**逐字节一致**（19:39:01=187 事故前末次写，此后零触碰）。
- **副本被真实使用**：副本内容 ≠ git HEAD 版（内核写了隔离副本，非摆设）。
- **腿本体**：stack probe 16/16 PASS（checks_failed=0，SUMMARY_JSON pass，180 横幅在场）×2 轮。
- **一轮工件损失披露**：t7 一轮的 ps1 主 console 因中间文件清理失误丢失（driver 事故后统一清理 `*.so.log` 时误删，探针断言 console/marker/副本残件在库）；二轮 t7b 全证据重跑补齐并以二轮为验收主证据。

## 四、回归模式清单（既有模式零行为改变）

- **diff 亲读**：全部改动为增量插入；journey/scenario preflight、journey workspace 隔离、chat/strip/mix 等默认腿路径无一行删改；不开 GUI 腿开关时隔离块整体跳过、干净端口时两守卫函数空过（`$null -eq $Listener` return）。
- **跑通回归**：T5（复用态默认腿族 exit=0 硬证据）+ T7b（干净端口默认腿族 13 ok 行+exitcode.txt=0——T3 的 console 因 driver 挂起+清理失误丢失，其证据缺口由 T7b 同栈同腿族补齐）。journey 模式未重跑（diff 亲读零触碰+preflight 既有 throw 独立于本改动）。

## 五、端测边界声明与披露

- 本卡改动面=ps1 脚本行为（启动守卫+环境变量默认），无 webui/Godot 渲染面改动，E2E-WEBUI-1/JOURNEY-1 层级不适用。
- **T3 退出码未直接捕获**（driver 挂起事故+console 清理失误双重损失）；干净端口默认腿族证据由 T7b 补齐（exitcode.txt=0）。
- **driver 挂起披露**：`driver_guard_acceptance.ps1` 在 T3 console 完成后卡死（未达 T4 目录创建），driver 进程被杀；T4/T5/T6 改为等效直接调用执行并落盘 t4/t5/t6 工件。driver 修复与重跑不属于本卡文件域（ps1 域外另卡可议）；已修的 driver 版本随工件入库备查。
- T4/T5/T7 复用的内核=主检出 `VitApp\build\VitApp_artefacts\Release\VitApp.exe`（只读复用，187 先例）；T5/T7/T7b `-SkipBuild` 二进制=T3 本 worktree 构建（Go 零改动，构建产物=当前分支代码）。
- 拆栈复核（T6/T8 两轮）：5555/5556/7878/8787/18787 tcp listen=0、UDP 4444/4445=0、VitApp/VitAgent/VspHub/Godot=0；假监听进程全数拆净。
- 主检出 `Settings.xml` 等内核启动常规写入面不在本卡验收口径（零触碰验收针对 default_project.xml 工程文件）。

## 六、与 185 的错峰记录

185 复跑登记腿（20:22）于 20:23:42 PASS 收口（84/84 亲核）+20:26 复核栈拆净；本腿登记（73be5071）注明错峰裁定依据，未覆盖、未代写其记录行。
