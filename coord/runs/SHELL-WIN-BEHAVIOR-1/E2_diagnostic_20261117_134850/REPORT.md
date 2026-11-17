# 394-E2 最后normal诊断：FAIL，停止继续运行

- Run由Get-Date生成；2026-11-17 13:48:50启动。主仓ce1c87d0、前端575dd7138e835a23c40dc630f65aab6c4c4daef0；实际fresh native/agent/UI构建，无C/旧基础腿重复。命令为既有wrapper LegA、CardIdSHELL-WIN-BEHAVIOR-1、WindowBehaviorChecks、normal、SkipFailureLegs、SkipLegacyLegs，前端绝对路径shell-first-focus-394，FixtureSource D:/Vit_DAW/temp/f3_empty_project.vit，RunRoot本目录。
- **inner exit1、wrapper exit1**。真实native失败经collector failfast保全，不再等待250s或手工终止；当前127samples/0sample错误。原两个失败attempt永久保留；这是最后允许诊断，dependent矩阵未跑，不再原样/另版继续运行。
- request=`webui_launcher_br_mwk9vvxk`、directPID29464。GD readiness.begin与bootstrap.config feature=true initialmode1；native ticket首cause=`hidden_geometry_preparation_failed`。queued/ui tick95130734，completed95130812。before同PID/window1575222：visible=true、iconic=true、topmost=false、noactivate=false；after：visible=false、iconic=true、topmost=false、noactivate=true。安全iconic窗口hide/style步骤后仍iconic，未达ordinary非iconic大窗，prepared/confirmed/direct.newopen/mounted/snapshot未发生。真实failed不能记作覆盖不足成功。
- Native trace daw_foreground_seen=false/foreground_identity_unknown=false；before/after foreground_hwnd1772186外部应用。开前sample foreground=other；shell-preservation分支not_exercised，未强制焦点或把other当shell通过。此腿在bootstrap真实失败处已停，不能声称rawFG最终完整成功或真实ready链通过。
- GD raw、原bridge/ticket、完整partial capture与消息、stdout、原exit/meta/buildhash均保全；供应商LLM0（未到旧L3、无telemetry记录）。没有主机GUI/键鼠/SetForegroundWindow测试动作，未修改前端focus产品。
- 13:50:03.742 +0800 owner亲核Vit进程0、TCP7878/7879/8787/5555/5556/9333监听0、UDP4445=0。六快照hash前后一致；fixtureAC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D不变，recents807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353。可由主管统一释放。
- 本回执交主管裁定，不通过391/394-E、不放行用户体验，不恢复delayed/negative矩阵或重复已验389 C。
