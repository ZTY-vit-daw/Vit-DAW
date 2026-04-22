import os
# 🌟 救命关键：必须在 import cv2 之前解封 EXR！
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"

import cv2  # 现在 OpenCV 才能读到上面的解封令
import numpy as np
import librosa

print("🚀 [Vit-DAW] 强制 4 通道底层 EXR 烘焙器启动...")

# 1. 读取音频
audio_path = "test_target_3s.wav"
y, sr = librosa.load(audio_path, sr=None, mono=False)
target_len = int(5.0 * sr)
if y.shape[1] < target_len:
    y = np.pad(y, ((0, 0), (0, target_len - y.shape[1])))

W_FRAMES = 500
N_BINS = 336
hop_length = int(sr / 100) 

# 2. 提取 A 通道：时点上的峰值 (包络)
peak_envelope = np.max(np.abs(y.reshape(2, W_FRAMES, -1)), axis=(0, 2))

# 3. 提取 R/G/B 通道：CQT 频率与相位
cqt_complex_l = librosa.cqt(y[0], sr=sr, hop_length=hop_length, fmin=librosa.note_to_hz('C1'), n_bins=N_BINS, bins_per_octave=48)[:, :W_FRAMES]
cqt_complex_r = librosa.cqt(y[1], sr=sr, hop_length=hop_length, fmin=librosa.note_to_hz('C1'), n_bins=N_BINS, bins_per_octave=48)[:, :W_FRAMES]

mag_l = np.abs(cqt_complex_l)
mag_r = np.abs(cqt_complex_r)
col_max_l = np.max(mag_l, axis=0, keepdims=True); col_max_l[col_max_l == 0] = 1e-10
col_max_r = np.max(mag_r, axis=0, keepdims=True); col_max_r[col_max_r == 0] = 1e-10
mag_l_norm = np.power(mag_l / col_max_l, 1.5)
mag_r_norm = np.power(mag_r / col_max_r, 1.5)

phase_l = np.angle(cqt_complex_l)
phase_r = np.angle(cqt_complex_r)
phase_diff = (phase_l - phase_r + np.pi) / (2 * np.pi)

# ==========================================
# 🌟 4. 暴力封装：强制使用 BGRA 顺序适配 OpenCV
# ==========================================
exr_bgra = np.zeros((N_BINS, W_FRAMES, 4), dtype=np.float32)

exr_bgra[:, :, 0] = phase_diff       # B (通道0): 相位差
exr_bgra[:, :, 1] = mag_r_norm       # G (通道1): 右声道
exr_bgra[:, :, 2] = mag_l_norm       # R (通道2): 左声道
exr_bgra[:, :, 3] = np.tile(peak_envelope, (N_BINS, 1)) # A (通道3): 时域包络

output_dir = "baked_tiles_test"
os.makedirs(output_dir, exist_ok=True)
out_path = os.path.join(output_dir, "tile_000_core.exr")

# 写入文件
cv2.imwrite(out_path, exr_bgra)
print(f"✅ 完美 4 通道 EXR 已硬核写入: {out_path}")