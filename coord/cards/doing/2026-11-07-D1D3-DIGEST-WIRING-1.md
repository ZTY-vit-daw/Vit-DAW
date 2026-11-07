# D1D3-DIGEST-WIRING-1：d1d3 观察消费面 digest 子图接线修（272 取证转卡）

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[272 取证](../../runs/OBSFACE-SELECTIVE-MISS-1/forensics.md)+[272 裁定](../../cards/done/2026-11-07-OBSFACE-SELECTIVE-MISS-1.md)——主缺陷=d1d3AssemblePreMasterReport 从 harness 响应**顶层**读 stereo_status/peak_dbfs/waveform/rankings 而实际嵌套 `out["digest"]`（拍平面 `out["acoustic_digest"]` 键名又不同）→投影结构性恒 missing，M1-4 起先天；单测喂 digest 形状掩盖）
- 派发确认：已确认（主管发卡，自主批次·限额恢复后首批）；验收负责人：GLM 主管决策流
- 池序 274；目标仓库=主仓 agent（d1d3 消费面）；P1 / 0.5 天 / 依赖=272 工件在库；L1-L2 / PC；零栈先行（单测+形状反钉），真栈回归门可搭后续 run

## 目标

1. **接线修**：d1d3AssemblePreMasterReport 改读 `out["digest"]` 子图（拍平面 `out["acoustic_digest"]` 键名差异同步处理）——三面（stereo/peak/rms/crest+waveform+rankings）按 272 对照矩阵逐面修；
2. **单测反钉**：现有单测夹具形状改用真实响应形状（266/265 run 工件中的实盘响应为夹具蓝本——教训=夹具形状反钉真实响应，杜绝顶层/digest 双形状自洽假绿）；
3. **相邻两处顺带勘察**（272 遗留）：①D1 全局 waveform 面 settle 相位 ready→missing 翻转（265 同链未翻转，差异驱动）②265 逐轨 stereo mix.read 7/7 missing vs feature snapshot 19 行 ready（第二处消费错位候选）——勘察结论同卡交付（可修则修=同文件域，跨域则记录）；
4. 回归：D1/D2 门 A 真栈断言可作回归门（搭后续 run）；chat/mixboard 包全绿。

## 约束

文件域=agent/internal/chat/d1d3 消费面+测试；kernel 零改动（铸造层 13/13 ready 亲证）；禁动 harness 响应结构本体（只修读取侧）。

## 停止条件

相邻勘察发现第三处跨域缺陷→记录上交；断言失败取证不重跑。

## 领取/回执/验收

- 领取：2026-11-07 01:16 UTC / origin/main=09f284d8661b4b6d6561c0269d9d88379415c7bc / owner=GLM 主会话 L2-worker 子代理（ZCode/PC）/ 分支=port/d1d3-digest-wiring-1 / worktree=D:/Vit_DAW_wt_274impl / 领取基线：HEAD=09f284d8（worktree 自 origin/main 新建）、status --short 空、diff --stat 空（零领取前 diff）；doing 三卡（ARR-SRCLOAD-1=capability_pack+catalog、EDU-LEVEL-RUNTIME-B=edu_jam_chat+server.go 接线、CLOSURE-ROUTE-ADAPT-1=audio_closure_controller）与本卡文件域核对不相交
- 回执：（执行侧回填）
- 验收：（主管回填）
