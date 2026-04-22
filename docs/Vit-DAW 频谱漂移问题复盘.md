# Vit-DAW 频谱漂移问题复盘（2026-03）

## 一、现象总览（用户可见）

本次问题不是单点故障，而是多个“看起来像同一个 bug”的现象叠加：

1. 频域播放时，蓝色中轴/切片附近出现“持续偏移”的体感（频谱像在飘）。
2. 100Hz 纯音有时出现“偏离中心、变细、甚至瞬时消失/闪烁”。
3. 视角切换后，中轴和频谱存在“看起来不同步”的错觉。
4. 部分调参后，频谱不飘但变成“拖影/延迟感明显”。
5. 调试后期一度出现右声道粉噪显示异常（后回滚恢复）。

## 二、根因拆解（按层）

### A. 几何/相机层（并非最终主因）

- 已验证项：
  - 频域 `plane_x` 基本恒定（约 2.499）。
  - `playhead` 在频域锁到 tile 中心（约 2.5）。
  - `yaw/pitch/rig` 收敛后稳定，不存在持续几何漂移。
- 结论：
  - “中轴偏移”早期问题与相机/坐标链有关，后续已基本修复；
  - 最终残留“频谱在飘”并非几何链路导致。

### B. 内核谱值层（有影响，但不是最终决定性根因）

- 尝试过：
  - FFT 4096 -> 8192；
  - 自适应软门限；
  - 时域 EMA；
  - max/top-k/centroid 聚合对比；
  - no_norm/no_gate 等诊断矩阵。
- 诊断结论：
  - 纯音段峰值 bin 可稳定；
  - 噪声段峰值跳动属信号特性；
  - 内核可改善抖动，但不能单独解释“空间漂移观感”。

### C. Shader 表达层（最终主因）

**核心根因：在频域模式下，把“采样时间”和“空间切片位置”绑定为同一个 `n`。**

- 原先行为：
  - `n` 随 transport 前进；
  - 同时用于 `texture` 时间采样与 `UV.x` 空间切片判定；
  - 导致“时间在动时，空间位置也在动” -> 视觉上就是“整片在漂”。
- 这就是为什么：
  - 即使相机与中轴稳定，仍然体感漂移；
  - 调整 STFT 只能缓解抖动，却不能彻底消除“空间漂移”。

## 三、最终修复

在 `track shader.gdshader` 中对频域模式（`view_mode == 1`）做了解耦：

1. **空间切片固定：**
   - 切片中心固定在 `UV.x = 0.5`（频域空间不再左右跑）。
2. **时间采样继续动态：**
   - `texture(data_matrix, sample_uv)` 的 `sample_uv.x` 使用实时 `n`；
   - 频谱内容仍随时间变化，但渲染位置固定。
3. **短片段边界约束：**
   - 用 `tile_duration / mesh_time_span_seconds` 限制有效 `n` 范围，避免 3s 素材在 5s 纹理尾部继续制造滑动感。
4. **频域视觉去干扰：**
   - 降低 phase 驱动颜色/扫描光造成的“漂移错觉”。

## 四、为什么“看起来简单但很难排”

这是典型的“跨层假象”：

- 用户看到的是“空间漂移”；
- 工程上同时存在：
  - 相机时序问题（历史遗留）；
  - 频谱抖动问题（信号处理层）；
  - shader 表达耦合问题（最终主因）。

如果不做分层日志，很容易把问题归咎于 STFT 天生限制，或持续在相机层反复试错。

## 五、经验教训（必须固化）

1. **频域渲染必须解耦“采样时间”和“空间位置”。**
   - 时间轴前进 != 空间切片移动。
2. **任何“漂移”先做三层日志：**
   - 几何链（plane_x/playhead_x/yaw）；
   - 内核链（peak_bin/avg_jump）；
   - shader 链（n/slice_uv/sample_u）。
3. **短素材与固定 tile 宽度并存时，必须有 `tile_duration` 边界策略。**
4. **调优先做诊断矩阵，不要盲调平滑参数。**
5. **展示版前必须清理 debug 默认值，避免误判和性能噪声。**

## 六、相关改动（关键文件）

- 前端：
  - `D:\Godot\project\vit-daw-frontend\track shader.gdshader`
  - `D:\Godot\project\vit-daw-frontend\tile_manager.gd`
  - `D:\Godot\project\vit-daw-frontend\Turntable_Camera.gd`
  - `D:\Godot\project\vit-daw-frontend\right_3d_wrapper.gd`
  - `D:\Godot\project\vit-daw-frontend\time_ruler.gd`
  - `D:\Godot\project\vit-daw-frontend\track_row.tscn`
- 内核：
  - `D:\Vit_DAW\VitApp\Source\Service\TiledSpectrogramBaker.cpp`

## 七、后续建议

1. 将本复盘纳入版本发布说明（用户可见“问题已修复 + 原因说明”）。
2. 增加一个自动化回归用例：3s 纯音 + 频域固定切片验证。
3. 保留诊断开关，但默认关闭；仅在排障场景开启。
