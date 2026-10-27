# Ruling：FE-ENVELOPE-LANE-TOGGLE-1（198）——pass

- 验收人：GLM 主管决策流 / 2026-10-27 午后
- 合入：前端 9c791a8（实现）+主仓 4ad67ce3（真栈工件）。

## 亲核面

1. **diff 亲读**（四文件全量）：①Track_Row_Controller.gd——MenuButton 挂载 MSR 行电平 label 前（40x22/master+folder 隐藏/不挤 shrink）、ENVELOPE_LANE_MODE_LABELS 中文映射、下拉经行 API 落档、envelope_lane_mode_changed 连接跟随；②track_envelope_lane_2d.gd——`_lane_mode` 缺省 off→auto（2025-10-25 裁定回归，注释链完整）、新信号只在档位实际迁移时发（粘性覆盖/同档重入不发——语义正确）、_ready 兜底首扫描；③bind_virtual_track 两 hunk=包络车道 id 跟随行重绑（域内最小修）；④probe_envelope_lane_auto.gd A1/A2 期望更新（默认 off→auto）。
2. **判定面**：新探针 40/40+既有六件零回退+真栈 25/25 连续两轮（默认 auto 零显式 set+装配面控件在场+写点自动展开内核对照+菜单关粘性+clear 回零）；装配面断言=卡面验收硬要求达成。
3. **栈纪律**：登记/释放/即时复核在案（PC-RUNTIME-STACK 亲核），错峰四卡兑现（12:39/12:50/12:55/13:03 串行无重叠）。

## 披露四项裁定

- ⓐ lane_auto 探针期望更新：**采信**——卡面目标 2 的必然连带（旧期望断言的是被本卡收回的偏离行为），断言面不变 41 检；越「探针新文件」字面域属如实披露非越权。
- ⓑ bind_virtual_track 补丁：**采信**——同文件域最小修，默认 auto 在虚拟化行池下扫描对准当前轨的正确性前提；不修即缺陷。
- ⓒ 进程误杀事故：**记档不阻塞**——误杀他卡 -s 探针进程（自报+命令行核对+零持久损害+对方自跑恢复）；教训（拆进程逐 pid 命令行亲核）入 gate 风险位。
- ⓓ 多轨扫描压力：**采信留手测节观察**（停止条件②未触发）。

## 记律

工件曾以未跟踪副本滞留主检出（第五例，哈希核对一致后让位）——PROTOCOL 工件纪律（2026-10-27 入律）覆盖。
