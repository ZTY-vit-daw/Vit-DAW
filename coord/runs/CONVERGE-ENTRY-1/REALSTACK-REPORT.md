# 388 / 401 执行侧真实栈回执（待主管最终验收）

实现分支`codex/converge-entry-388`；稳定前端`C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend` @d05043e11a79452b9155a89f3405668994eafdcb。该前端未含394/402，结果不归功窗口焦点卡，不证明最终组合版本。

## 结果与命令

- **A PASS**：`stack_A_20261117_1153`，被测HEAD e966b90788621e5bd7d1b852cc0ab7cf89c3325d。内层vitshell_full_smoke exit0（rounds1/1）、外层exit0，failure三腿/legacy本run亦全部PASS。
- **B PASS**：独立`stack_B_20261117_1155`，同HEAD；ui_entry2完整四腿exit0、外层exit0。原生报告`device/artifacts/ui_entry2_smoke_report.json`。
- 完整调用（仓库根目录）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -FrontendWorktree 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend' -FixtureSource 'D:/Vit_DAW/temp/f3_empty_project.vit' -RunRoot 'C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW/coord/runs/CONVERGE-ENTRY-1/stack_A_20261117_1153'
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg B -FrontendWorktree 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend' -FixtureSource 'D:/Vit_DAW/temp/f3_empty_project.vit' -RunRoot 'C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW/coord/runs/CONVERGE-ENTRY-1/stack_B_20261117_1155'
```

A包装器内层固定-Rounds1 -EntryManagementChecks，无SkipBuild/SkipFailure/SkipLegacy；B固定-EntryManagementChecks、未SkipLegacy。每次Start前fetch远端owner并即时净空，不复用别人的栈。原生meta/phase JSON是本脚本证据，没有d1_smoke_report.json，未伪造D1报告。

## 388/401事实

- 启动/工程管理两态真实GET `/agent/launcher/recent-projects`返回exists与治理directive；POST405。管理页显示当前fixture_a.vit保持打开；两个左下设置真实打开/Escape焦点归还，仅config GET，零保存；隐藏启动卷自由对话，返回工程保留未发送输入。
- 纯视图往返status计数不变；new/open/磁带三个真实入口可用，各经既有治理并cancel。3个request_id分别`webui_launcher_br_mwk5s5f0`、`webui_launcher_br_mwk5s5gs`、`webui_launcher_br_mwk5s5ir`，原bridge有各一条canceled结果，区间无godot.spawn。terminal计数3→6恰为三次取消（宿主main.cpp:638-643对opened/failed/canceled计数），非实际进程重启。
- 整个管理段Godot/agent/kernel/hub PID、工程path/uuid、HWND、current_url不变。d05043e缺native navigate_count，采用主管批准的主frame导航事件0+sentinel+HWND/url；null未当0通过。
- GET503为独立synthetic presentation injection，错误面非空态；解除route后真实GET恢复，未把注入说成真核故障。
- 局部telemetry before/after均0字节且HTTP无chat请求。A后续既有L3有1条本机127.0.0.1:1拒绝记录（ordinary_agent_semantic_entry，tokens0），供应商调用0；这是原旅程诚实失败交付断言，不混算管理区间。
- 后续L3b工程内新建/切换、L4关闭DAW回启动页、L5治理切fixture_b、L6全树关闭均通过；精确terminal计数7/8，默认不开管理flag的offset仍0。

## 全部attempt账

| run | HEAD | 实际退出 | 分类与停止点 |
|---|---|---|---|
| A_1143 | 30c6c48b | wrapper1，inner=-1 | 新ps1无BOM+PS5.1中文regex解码装置缺陷；未启任何内层进程。改codepoint后独立Guard_1146 exit0。 |
| A_1147 | 797085b7 | inner1/wrapper1 | 旧CDP selector不适配387 MessageStream，L1“assistant frame”失败；failure/legacy已PASS，未到管理。0ad4e2d7迁移消息/textarea/send选择器，未删语义断言。 |
| A_1149 | 0ad4e2d7 | inner1/wrapper1 | CDP管理子段PASS，ps1错误假定整个3次cancel区间launch_count不变；证据表明该字段计终态链。主管按代码裁定e966b907拆纯view与3cancel精确子区间并修后续offset。 |
| A_1153 | e966b907 | inner0/wrapper0 | 正式有效成功路径；1/1轮，未宣称稳定性。 |
| B_1155 | e966b907 | inner0/wrapper0 | 独立ui_entry2四腿PASS。 |

前3次均装置缺陷，不排除成“环境中断”或改写产品成功。同版本同断点没有原样重跑。原始stdout/exit/hygiene/meta/phase/构建日志均在对应目录及ZIP；首次snapshot基线未被修改。

## 卫生与终态

- 每attempt外层seed/build前快照recents与两个root Settings.xml/default_project.xml bytes/absence，finally哈希亲证；A/B和前置失败attempt均5项还原一致。recents终态`807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`。
- 主检出Settings SHA256 `72F0F5112EF926ED77F59F015E6CE492142F8FDF1828E5082E970AA949EDFBB8`；default XML `2ADC97E01E18454F3D295DAE7FD43B729C7AD84FDFA3871E2B01421BAEDCC161`，前后一致。
- fixture源SHA256 `AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D`全attempt不变，只写run副本。
- owner于**2026-11-17 11:57:06 +0800**亲核最终Vit进程0、TCP7878/7879/8787/5555/5556/9333监听0、UDP4445=0；已报主管统一释放。没有自行改main资源。
- 被测A VitShell SHA256 `9C66F0643BCE7F6398752B06158DA3D53F2360A474DED40FBF25025B404B06C3`；agent `13A546951D52CF8955888699187EA81501A297B291A477E44EB56ADB7C42FA1C`；kernel `A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E`；hub `B10CC41295066C48BB446B255D12D1A58D2D8D64F164D875083667B464C71F68`。
- 运行后仅装置日志UTF8读取修正（防PS5.1派生bridge中文乱码，ASCII判据不变）；原raw UTF8 bridge与完整raw agent日志均已归档，不重跑通过的产品域。

## 验收边界

以上是388/401执行侧自验成功路径。是否解锁389与最终合入交主管；后续394/402组合全旅程、真实窗口加载焦点/ready交付、用户从Godot入口手测仍单列。管理页当前沿用既有directive占位历史文案，终态由下方真实bridge状态呈现，未在本卡扩视觉或重写该历史组件。
