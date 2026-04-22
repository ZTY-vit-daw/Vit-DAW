# Vit-DAW 展示版发布前检查清单（2026-03）

## 1. 调试开关（必须关闭）

- `track_row.tscn`
  - `Right_3D_Wrapper.debug_print_freq_runtime = false`
  - `Right_3D_Wrapper.debug_frontend_diag = false`
  - `Turntable_Pivot.debug_print_freq_runtime = false`
  - `Turntable_Pivot.debug_frontend_diag = false`
  - `Tile_Manager.debug_print_slice_params = false`
  - `Tile_Manager.debug_frontend_diag = false`
  - `HUD_Y_Left.debug_frontend_diag = false`
  - `HUD_2D_Overlay.debug_frontend_diag = false`
- 环境变量（如有设置）清理：
  - `VIT_BAKER_DEBUG`
  - `VIT_BAKER_MODE`
  - `VIT_BAKER_POOL`
  - `VIT_BAKER_SMOOTH`

## 2. 关键功能回归

- 时域：
  - 播放头随 transport 正常推进；
  - 时间标尺 seek 正常；
  - 缩放/滚动条同步正常。
- 频域：
  - 中轴不漂移；
  - 切片固定在中心显示；
  - 频谱内容会随时间更新但不发生空间漂移；
  - 100Hz/5kHz 纯音可稳定显示。
- 视角切换：
  - `Top / 3D / Side` 动画正常；
  - 切换后无明显跳帧或坐标错位。

## 3. 音频素材回归包

- `test_target_3s.wav`（3s 纯音+噪声段）
- 立体声纯音校准文件（L/R 分离）
- 一段真实音乐片段（验证观感）

## 4. 观感检查（展示视角）

- 频域下无“频谱飘逸”主诉；
- 无明显闪烁/拖影；
- 右声道粉噪显示正常；
- HUD 标尺与画面一致（Y=0 对齐）。

## 5. 打包前建议

- 保存一份本次复盘文档：
  - `docs/spectrogram_drift_postmortem_2026-03.md`
- 保留诊断代码，但默认关闭；
- 标记本次版本为“频域稳定性修复版”。
