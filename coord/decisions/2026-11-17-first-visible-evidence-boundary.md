# 首次普通大窗的证据边界（405静态复核）

主管/root，2026-11-17。403/404已证明最小Godot非激活恢复和模式同步，但未证明首次普通可见窗口的工作区geometry/z。

405单候选为隐藏placement预设后SW_SHOWNOACTIVATE。主管复核及Astra/high最疑难只读参谋均确认：SetWindowPlacement/ShowWindow仅有目标状态契约；目标WndProc可在WM_WINDOWPOSCHANGING中改写position/size/z。OUTOFCONTEXT WinEvent只带事件时刻，不带事件发生时的历史geometry；回调即使零毫秒延迟，也不能证明调用内部状态。不得把first sampled升级成first transition。

DWMWA_CLOAK可在cloak期间阻止用户看见仍被合成的窗口，可作潜在遮蔽；官方未保证本场景跨进程调用必成功，需HRESULT与CLOAKED APP位实测。DwmFlush仅等待调用应用DirectX更新，不是整个桌面或Godot的present屏障。cloak/uncloak也不是geometry+z原子事务；不凭外部快照宣称首合成帧无竞态。本轮未实现、未运行这些候选。

决定：不消耗已知无法达到严格门槛的两场预算，不放宽用户“大窗在后/无抢焦点”语义。405先补只读“目标应用自身的转换配合/观测”可行性：考察既有extension能否在Godot所属线程合法记录/约束自身窗口的转换，不做跨进程注入/subclass、全局hook或桌面控制，不直接改生产。需要新生产文件域时另拆卡，主管先审取舍；当前无新增用户产品裁决项。

官方锚点：

- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setwindowplacement
- https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-showwindow
- https://learn.microsoft.com/en-us/windows/win32/winmsg/wm-windowposchanging
- https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/ne-dwmapi-dwmwindowattribute
- https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/nf-dwmapi-dwmsetwindowattribute
- https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/nf-dwmapi-dwmflush
