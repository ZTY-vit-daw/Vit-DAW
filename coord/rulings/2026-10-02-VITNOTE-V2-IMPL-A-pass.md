# Ruling：VITNOTE-V2-IMPL-A — pass（2026-10-02 决策会话；渲染面手测挂 V2 手测场）

## 验收四层

1. **diff 直读**（Godot 仓 port/vitnote-v2-impl-a@091cf80，7 文件 +730/-45）：
   - GestureMachine 纯类与设计 §4.1 转移表**逐条吻合**：发起闸门三条件（Q 持续按住+焦点文本区让位+无 clip 体让位[D1 迁键]）、阈值 4.0px（F5 先例）、Q+单击无害空操作、四类取消路径（ESC/松 Q/RMB/pending 三式）零残留、消费边界（活动态只消费手势事件流，字符/滚轮透传）、宿主 cancel/observe_position 兜底、RESOLVING 瞬态。零场景依赖设计落实（ctx 注入环境事实）。
   - 圈选层 Control 接线：`_input`+活跃期 set_input_as_handled（IDLE 完全透明）、`_process` Q 轮询复查+鼠标位置兜底（§4.4 防御性设计两条都落）、§4.3 视觉常量齐（暗幕镂空 α0.18/亮框 1.5px/四角标记/0.12s 淡出）。
   - manager 升格协调者：圈选层为 manager 子序首位（胶囊绘制于暗幕之上——顺序注释有据）；note 载荷 v2 最小形态（origin=global_circle+rect+faces 空辖区=§5.3 合法态，面数待 IMPL-B）；跨文件边界巧招**核实成立**——胶囊文案经 `hint.set("_hint_text",...)` 预置，vit_note_hint.gd:23/32-33 证 `_ready` 空文案才取默认值，先设后挂生效（不动卡外文件达成文案换装，合规）。
   - lane 通知原子退役（§8/D2 裁定）：`_notify_vit_note_hint` 调用点+函数体删除；采集函数保留（6 处引用在位，IMPL-B 转供）；两面挂载点顺序不变量注释固化。
2. **我方独立复跑**：①headless 状态机探针我方自跑 **checks=52 failed=0，EXIT=0**（状态机套件+lane 退役静态+两面挂载烟测）；②`--import` 零 SCRIPT/PARSE ERROR。与回执两轮一致。
3. **工件亲读**：coord/runs/VITNOTE-V2-IMPL-A/ 四件+README——命令/退出码/结果全可回指；**过程诚实申报优质**（首轮 import 抓到真实缺陷的修复前日志留档；探针开发中 S8a 暴露实现缺陷[release 位置未回写致终值漂移]已修；.gitignore 排 .log 按 FIX-BROADBAND-SHARED-1 先例改 .txt 落档字节不变）。
4. **边界核对**：渲染视觉/淡出动画/真实输入链/IME 归手测（§10.2 第 1/2/6/7 步）；Q 字符透传边界与分离窗口边界按设计声明；ObjectDB leaks 警告为 headless teardown 产物（退出码 0 为门槛）；运行栈零启动零残留声明在案。

## 裁定

- **pass**。渲染面手测（§10.2 第 1/2/6/7 步）挂 **V2 手测场**（九步清单一场销三卡；建议用户今日合并手测时顺带快测第 1/7 步作链路早期信号，非验收门槛）。
- Edge e 处置：V1 文档修订标注由决策侧随本裁定更新（Alt→Q 迁键追记+退役已落地标注）。
- 分支链：port/vitnote-v2-impl-a@091cf80 为现 HEAD，**V2-IMPL-B 依赖解锁**（注册处+timeline/control 供给器，设计 §6.0/§6.1/§6.4/§5/§9）。
