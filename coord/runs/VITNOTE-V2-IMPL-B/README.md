# VITNOTE-V2-IMPL-B 验收工件留档（结论可回指原始工件）

- 日期：2026-10-02；工作仓：D:\Godot\project\vit-daw-frontend，分支 port/vitnote-v2-impl-b（自 port/vitnote-v2-impl-a@091cf80 切出，实现 commit=3ea8a0a，已推 origin）
- Godot：D:\Godot\Godot_v4.6.1-stable_win64_console.exe（v4.6.1.stable.official.14d19694e，与 IMPL-A 同二进制）

## 命令与退出码（仓库根目录执行）

| 工件 | 命令 | 退出码 |
|---|---|---|
| probe_face_resolve_run1.txt | `"D:\Godot\Godot_v4.6.1-stable_win64_console.exe" --headless --path . -s tools/probe_vitnote_face_resolve.gd` | 0（探针 62/63——lifecycle 重挂断言红，见下） |
| probe_face_resolve_run2_scope_fixed.txt | 同上（_enter_tree/_exit_tree 生命周期修正后） | 0，74/74 PASS |
| probe_face_resolve_run3_rerun.txt | 同上（确定性复跑） | 0，74/74 PASS |
| probe_face_resolve_run4_final.txt | 同上（注释修正后终版，与 commit 3ea8a0a 代码一致） | 0，74/74 PASS |
| import_zero_script_error.txt | `... --headless --path . --import`（终版代码后重跑） | 0，SCRIPT/PARSE ERROR 计数=0 |

（原始输出为控制台重定向文本；按 FIX-BROADBAND-SHARED-1/go_full_after.txt 先例以 .txt 落档，内容字节不变。）

## 探针结果（run2 起三轮一致）

- checks=74 failed=0，PROBE PASS：
  - 静态契约 10 项：两供给器脚本四函数（identity/affordance/rects/resolve）齐备+参数个数；manager resolve_circle/_circle_hint_text 在位；
  - 沙盒管线套件 52 项：占比双口径数值（share 1.0/0.25、coverage 0.25/0.0625、多块面 5000/35000+coverage 1.0）、降序合并（平局 priority 决胜 j2>j1、再平局 face_id 字典序 k1<k2）、噪声门两侧（inter 3.0px² 剔除/4.0px² 边界保留/无交叠剔除）、空辖区+零面积 rect 守卫、null resolve=空 entries 不阻塞他面、残缺契约（缺 affordance）整面跳过、rect 原样透传、供给器返回字典非突变（深拷贝）、§5.2 元素六键形、priority 排序后抹除、生命周期（remove_child 离组/add_child 回组）、胶囊文案两分支（「圈选 3 面」/「空辖区」）、_on_circle_select_finished 端到端（hint.snapshot.faces 真解析+首面=fake_a+文案）；
  - 两面场景烟测：legacy+dock 的 VitNoteLayer 均持两供给器子节点+组注册；timeline vit_face_rects 类型正确（headless 下各 1 行）；resolve_circle 全视口真实几何路径跑通——两面均解析出 ["control@main", "timeline@<场景根名>"]（legacy=timeline@Main_Control、dock=timeline@VitDockRoot，scope 两面可区分）；control snapshot tracks 数组形态。
- 退出时 ObjectDB leaks 警告为探针环境产物（headless 场景 teardown，IMPL-A 同象），退出码 0 为门槛判据。

## 过程记录（诚实申报）

- run1 红项为**实现真实缺陷**：设计 §6.0 字面「_ready 入组」不对称——_ready 只在节点首挂时执行一次，remove_child 后重挂不重新入组（探针 lifecycle 断言抓到）。修正为 _enter_tree/_exit_tree 对称入组/离组（静态挂载语义不变），run2 起全绿。首红日志留档=修复前证据。
- run1→run2 之间另修 timeline face_id scope：首版行走至 Window 根（两面同值 "timeline@root"，设计 §6.1 scope 语义退化）；改为停在 Window 前最近容器（两面异值）。
- 探针侧三处开发中修正（不影响实现）：透传断言次序（G 块会覆盖 received_rect，移至前段）、完成回调面数断言（该圈实际命中 3 面）、Node 类型变量脚本属性访问改 .get()。

## 与卡片的对位

- §6.0 契约四函数+组 vit_face_supplier 注册：落地（供给器自注册，manager spawn；无 autoload——沿两面挂载先例 F8）。
- §5.1 管线①–⑤：resolve_circle 逐条对上（①组枚举+实时 rects 求交 ②双口径 D3 ③4.0px² 噪声门 ④resolve 失败=空 entries ⑤降序合并）。
- §6.1 timeline 供给器：Right_3D_Wrapper 行矩形（vit_track_rows 组+可见性过滤）+ `_collect_marquee_hits_global` **只调不改**（git diff 091cf80..3ea8a0a 对 app/tracks/track_scene/、app/timeline/、app/shell/、app/legacy/、vit_dock/ 零改动）+ time_window 命中包络/空命中映射两分支。
- §6.4 control 供给器：走带+轨头两块矩形+snapshot 形态（BPM/播放位置/播放态/循环区/相交轨当前值），observe only。
- §9 payload v2：faces[] 降序+空辖区合法态；胶囊文案面数。**键名沿 IMPL-A 仓内约定 `rect_global`**（§9 序列化示例的 rect 在内部快照字典沿用 rect_global 键——panel 消费方 vit_note_panel.gd:84/:167 读该键，不破兼容）。

## 未覆盖边界（归用户手测 §10.2 第 3[单面]/4 步——挂 V2 手测场）

- 真实跨面矩形占比目检（时间线单面圈选→摘要占比+clip 清单；圈住走带+轨头→BPM/播放位置/循环区/相交轨参数当前值、未相交轨不入）；
- 渲染视觉/IME/输入链（IMPL-A 边界不变）；
- 面板头部摘要呈现：v2 faces 形态下 vit_note_panel `_rebuild_summary` 仍读 v1 顶层 track_ids/clip_ids（本卡后显示「空辖区」）——多面版摘要归 IMPL-D（vit_note_panel.gd 不在本卡文件域，零改动）。
