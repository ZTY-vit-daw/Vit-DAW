# 404 无窗实现自验回执（运行验证尚未授权）

执行 owner=/root/restore403，主管验收=/root。已从结束的403 B预算显式转404；读取共享main3882d198中的404卡。沿用两执行树/branch，不合main、不改协调卡资源。403全部旧证据及两个隔离shader缓存原封保留。

唯一代码文件：前端VitShell/tests/win32_noactivate_restore_probe.cpp；代码提交cdf1924。主仓runner内容仍等于f9792767，CMake未改。前端领取基线bb2a401；主执行树基线beb38025之后只有403工件提交831fe811，当前代码未叠其他改动。implementation.diff.txt保存本卡完整差异；最终hash与HEAD/status另存。

## 改动及作用

去掉“EnumWindows首个same-PID且无owner窗口就是主窗”的findChild写入。现在只从Godot ready记录提取nonce/PID/seq/native HWND；列出exact child的全部顶层候选，记录class、owner、PID、IsWindow、style/exstyle、visible/iconic/placement/geometry以及原始CREATE/SHOW/DESTROY匹配。枚举列表不修改target，helper排前不影响选择；按engine原生句柄精确查找，缺失不回退。

纯关联谓词要求ready/seq完整、nonce一致、PID为本次创建child、句柄有效、GetWindowThreadProcessId为exact child、无owner/非WS_CHILD/class非空，以及同一generation的原始窗口CREATE+SHOW、未destroy。SHOW原生时间早于当前CREATE会拒绝。关联成功后target只赋值一次；每次后续读数核nonce/PID/engineHWND、IsWindow/PID/class、窗口generation以及seq不倒退，检测句柄更换、销毁/复用或class变化即coverage不足，不能转指helper。

WinEvent保留原生tick和投递tick。事件只在OBJID_WINDOW/CHILDID_SELF通道更新生命周期；已知child窗口销毁后PID读不到时保留其原始事件。ready文件与原生回调异步，关联前有限drain，原始事件时间仍独立保存。本地生命周期/seq不构成OS绝不丢事件的证明；真实Godot连续性仍待另批运行取证。

新增显式入口 --identity-selftest <output.jsonl>，在Godot/自窗分支之前直接分发。该入口只用内存里的ReadyIdentity/NativeIdentity/WindowEvents/MainBinding纯数据，不RegisterClass/CreateWindow/ShowWindow/SetWinEventHook/CreateProcess，不启动Godot或任何测试窗口。不是用合成窗口冒充真实Godot。

## 命令与原始输出

完整命令/exit见commands.txt。现有tests独立CMake configure exit0，首次build exit0；最初22例自验exit0。增加stale-SHOW时间反例后，最终build exit0、最终无窗自验exit0，23例全部通过；两批原始输出均保留。既有C4244 ASCII nonce宽窄转换warning仍在，无编译错误。git diff --check exit0，runner差异检查exit0。

identity_selftest_final.jsonl含24行：23个case与1个汇总，failures=0/window_operations=0。覆盖helper-first与main-first都选engine主句柄；wrong ready/native PID、invalid/null/missing HWND、未ready/缺seq/错nonce、缺SHOW/错event PID/缺CREATE/旧SHOW、销毁/重用/更换HWND、seq倒退、owner/WS_CHILD/class变化；同seq重复读取和下一seq保持身份正常放行。这些是准入谓词反例，并不证明Win32事件采集或真实Godot恢复。

最终源码SHA256=85F2CBAB4A785D5D52328AAB2A24225A863818F26116D10EF17CF25F87AD2AED。
最终自测binary SHA256=73A244DAB2EAC5927B8FC4802C8D1FCC1EED0B22DCC6DC6D8E977DEC5771EAC9。
final_hashes.json对应最终版本；hashes.json对应增加时间反例之前的第一次自验，不混归最终binary。

## 边界及卫生

本轮只启动两个显式无窗自验进程；没有Godot、窗口、hook、真实栈、工程/fixture、用户配置写入、键鼠或截图。15:14:07+0800最终Vit/Godot/probe进程空；recents SHA仍807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353。构建目录/二进制不提交。前端提交后clean；主仓仍只含旧403缓存及本卡工件。

可证：代码已消除首个helper决定主窗的启发式；23个纯关联正/反例按预期接受或拒绝；产物可构建。不可证：真实Godot主窗关联成功、native安全iconic前提、恢复后引擎mode同步、两秒非激活/前台保持。403 B不补跑，404真实运行须主管读diff和本回执后另登记预算。主管保留最终验收权。
