# OBSFACE-SELECTIVE-MISS-1：kernel 观察面选择性缺失取证（266 两现转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[266 裁定](../../cards/done/2026-11-06-D3-REF-ALIGN-THIN-1.md)「再现即开取证卡」——verifyrun1[四开关]+verifyrun2[单开关]两现，叠加交互候选排除→kernel 观察面选择性缺失非确定性候选②增强；证据=[verifyrun1/evidence.md](../runs/D3-REF-ALIGN-THIN-1/verifyrun1/evidence.md)+[verifyrun2/d3_check_chat.json](../runs/D3-REF-ALIGN-THIN-1/verifyrun2/d3_check_chat.json)+265 reverify 当日 stereo missing 对照）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 272；目标=取证为主（读档零 LLM 先行）；P2 / 0.5 天 / 无依赖；L1-L2 / PC；跑腿如需再议

## 背景（三组对照证据）

同一 kernel 二进制（F9E3DA0A）同 fixture 下：265 reverify 当日 stereo=missing；266 verifyrun1 stereo=ready 但 peak/rms/crest 全缺；266 verifyrun2 digest D1/D2 全局面缺席——**kernel 铸造的观察 digest 存在选择性/非确定性缺失**，同 run 判据消费面测量行健康（两源分离按设计）。

## 目标

1. **读档取证**：kernel digest 铸造路径（VitApp C++ 观察面/digest 组装代码）勘察——缺失面的分支条件（按需计算？缓存失效？观察窗口？）+三组 run 的 digest 差异逐字段对照表；
2. **定性**：设计内（如按需披露）vs 缺陷（非确定缺失）——锚点级证据；
3. 若属缺陷：最小修方案交裁定（kernel 面=域外，另派）；若设计内：披露口径文档化建议。

## 约束

文件域=coord/runs/OBSFACE-SELECTIVE-MISS-1/（取证工件）；零代码改动；kernel C++ 面只读勘察。

## 停止条件

勘察发现 digest 缺失有在案设计说明→记录收口；证据不足以下定性→如实两案并陈。

## 领取/回执/验收

### 领取（2026-11-07，取证腿）

- Owner：GLM 主会话 freeflash 子代理（**取证腿**——纯读档零 LLM 零代码改动：三组 digest 对照+kernel C++ 只读勘察+定性，产出 forensics.md，kernel 面只读）
- 机器/worktree：PC / `D:/Vit_DAW_wt_272impl` @ 分支 `port/obsface-selective-miss-1`
- 领取基线：`git rev-parse HEAD`=`37fd7c97c26f221ab92f04dfe5b2518e5c556769`（=origin/main 当日最新）；worktree 全新检出，`git status --short` 空、`git diff --stat` 空（无领取前 diff）
- 领取动作：卡面 todo→doing（git mv），本记录回填

### 回执（2026-11-07，取证腿完成）

- **定性一句话：缺陷成立但位置改写——主缺陷=agent 侧 d1d3 观察消费面接线错位（d1d3AssemblePreMasterReport 从 harness 响应顶层读 stereo_status/peak_dbfs/waveform/rankings，实际嵌套在 out["digest"]，拍平面在 out["acoustic_digest"] 且 stereo 键名不同；M1-4 e0e2f4ac 起先天，单测喂错形状掩盖），投影结构性恒 missing；卡面「kernel 观察面选择性缺失非确定性」不成立——铸造层（mixboard.BuildDigest，agent 侧 Go，VitApp C++ 零命中）stereo 面 13/13 采样点 ready，D1 全局 waveform 面 ready→missing 翻转为 D1 settle 相位相关的真变动（设计内疑似，265 同链未翻转差异驱动未定位，两案并陈）**。
- 对照表+锚点+最小修方案草案：coord/runs/OBSFACE-SELECTIVE-MISS-1/forensics.md（§2 五采样点矩阵 / §3 错位链与单测掩盖面 / §4 定性 / §5 相邻发现[mix.read 逐轨路径第二处错位候选] / §6 修复草案[读 digest 子图+单测反钉+既有 D1/D2 门 A 为回归门]）
- 工件：主仓树 D:/Vit_DAW/coord/runs/OBSFACE-SELECTIVE-MISS-1/（PROGRESS.md+forensics.md，派发指定工件目录）；同内容随本分支提交入库（run 工件只读引用三组 run 仓库工件+wt_265rv/wt_266impl artifacts 持久化观察包原件）
- 边界遵守：零代码改动/零栈/零 LLM；VitApp 只读 grep；run 工件目录只读访问；逐轨 mix.read 路径与 waveform 翻转驱动未扩查（锚点已记，交裁定是否另开卡）
- 验证边界声明：本卡纯读档取证，无单测/烟测执行面；修复卡验收建议=接线单测+D1 门 A 真栈断言（现成 ps1:1100-1105）
