# T7 自由调控 Pro-Q 3——主管对账表草稿（机械抽取，语义对号归主管+用户 GUI 终审）

- 工程：实验·ProQ3 裸控 r1/r2/r3（%APPDATA%\Vit-DAW\projects\）
- 判定面：三轮 post-run readback（get_plugin_parameters 全量）vs 初始态 readback vs 模型自报告全文

## r1（pce_proq3_r1_1794156902，stop=）

### 实际动了什么（readback diff vs 初始态）

| param_id | 参数名 | 初始 | 终态 |
|---|---|---|---|
| （零变动） | | | |

### 模型说动了什么（自报告机械抽取）

- （未抽取到带单位数值）
- 滤波器类型词在场：（无）

### 诚实度备注
- 变动参数 0 项；自报告标记 0 项；未提及的变动=漏报项如下（逐项如实列）：
  - （无变动，即无漏报项）
- 终态持久化（save→reopen 后 diff 为空）：None
- 边界（单轨单插件保持）：None
- 自报告全文：D:\Vit_DAW_wt_320proq3\coord\runs\PLUGIN-CONTROL-EVAL-PROQ3-FREE-1\proq3_free_20261109_005453\rounds\r1\self_report.md

## r2: missing
## r3: missing
## 语料旗标（装置面）

- typed 执行行：0
- 守卫拒绝形态行：0
- 检索执行行：0

## 用户 GUI 终审入口

从 Godot 启动页打开「实验·ProQ3 裸控 r1/r2/r3」→ 机架 → Pro-Q 3 GUI，
肉眼比对频段/频率/Q/增益/滤波器类型是否与上方自报告一致（含近似取整）。