import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import librosa

print("🚀 [Vit-DAW] 启动立体声独立解析烘焙器 (修复频宽与幽灵音)...")

audio_path = "test_target_3s.wav"
y, sr = librosa.load(audio_path, sr=None, mono=False)
target_len = int(5.0 * sr)
if y.shape[1] < target_len:
    y = np.pad(y, ((0, 0), (0, target_len - y.shape[1])))

W_FRAMES = 500
N_BINS = 336
hop_length = int(sr / 100) 

# 1. 提取独立包络
env_l = np.max(np.abs(y[0].reshape(W_FRAMES, -1)), axis=1)
env_r = np.max(np.abs(y[1].reshape(W_FRAMES, -1)), axis=1)

# 2. 提取宽频 CQT (20Hz - 13000Hz，完美捕获 5kHz 尖峰)
fmin = 20.0
bins_per_octave = 36 # 336 / 36 = 9.33 octaves
cqt_complex_l = librosa.cqt(y[0], sr=sr, hop_length=hop_length, fmin=fmin, n_bins=N_BINS, bins_per_octave=bins_per_octave)[:, :W_FRAMES]
cqt_complex_r = librosa.cqt(y[1], sr=sr, hop_length=hop_length, fmin=fmin, n_bins=N_BINS, bins_per_octave=bins_per_octave)[:, :W_FRAMES]

# 3. 归一化并【抹除静音底噪】
mag_l = np.abs(cqt_complex_l)
mag_r = np.abs(cqt_complex_r)
col_max_l = np.max(mag_l, axis=0, keepdims=True); col_max_l[col_max_l == 0] = 1e-10
col_max_r = np.max(mag_r, axis=0, keepdims=True); col_max_r[col_max_r == 0] = 1e-10

mag_l_norm = np.power(mag_l / col_max_l, 1.5)
mag_r_norm = np.power(mag_r / col_max_r, 1.5)

# 🌟 修复幽灵音：如果该帧静音，频率彻底清零！
mag_l_norm[:, env_l < 1e-3] = 0.0
mag_r_norm[:, env_r < 1e-3] = 0.0

# 4. 提取相位差
phase_l = np.angle(cqt_complex_l)
phase_r = np.angle(cqt_complex_r)
phase_diff = (phase_l - phase_r + np.pi) / (2 * np.pi)

# ==========================================
# 🌟 5. 终极数据矩阵封装 (真正解决 L/R 独立)
# ==========================================
exr_bgra = np.zeros((N_BINS, W_FRAMES, 4), dtype=np.float32)

# B通道：相位差 (保持不变)
exr_bgra[:, :, 0] = phase_diff       

# G通道：直接把右声道包络乘进去！(物理绝对高度)
exr_bgra[:, :, 1] = mag_r_norm * env_r 

# R通道：直接把左声道包络乘进去！(物理绝对高度)
exr_bgra[:, :, 2] = mag_l_norm * env_l 

# A通道：全局最大峰值 (留作画时域外壳使用)
global_envelope = np.maximum(env_l, env_r)
exr_bgra[:, :, 3] = np.tile(global_envelope, (N_BINS, 1)) 

output_dir = "baked_tiles_test"
os.makedirs(output_dir, exist_ok=True)
out_path = os.path.join(output_dir, "tile_000_core.exr")
cv2.imwrite(out_path, exr_bgra)
print(f"✅ 基准级 4 通道 EXR 已写入！")