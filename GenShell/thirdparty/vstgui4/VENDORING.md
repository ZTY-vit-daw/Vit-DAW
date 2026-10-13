# vstgui4 vendoring 记录

- 来源：https://github.com/steinbergmedia/vstgui.git
- 获取方式：`git clone --depth 1`（浅克隆）后移除 `.git` 以普通源码形式入库（非 submodule——本卡文件域限 `/GenShell/**`，submodule 需改根 `.gitmodules` 越域）
- 上游 commit：`d646b60f0b1f9b2e68d4b65d3ea477f7bb90f487`（master，2026-10-13 克隆）
- 许可：Apache-2.0（见 LICENSE）；本目录仅含上游原样源码 + 本记录文件，无本地修改
- 用途：GenShell.vst3 编辑器（静态六区骨架）的 GUI 库（GENSHELL-CPP-1）
