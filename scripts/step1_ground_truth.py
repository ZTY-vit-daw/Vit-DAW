import numpy as np
import librosa
import matplotlib.pyplot as plt
import soundfile as sf
import os

print("🚀 [Vit-DAW 真理生成器 V2] 启动...")

# ==========================================
# 1. 物理源头：生成 3 秒测试靶标音频
# ==========================================
sr = 48000
duration = 3.0
t = np.linspace(0, duration, int(sr * duration), endpoint=False)
y = np.zeros((2, len(t))) 

# 0-1s: 左声道 100Hz 
y[0, (t >= 0) & (t < 1)] = np.sin(2 * np.pi * 100 * t[(t >= 0) & (t < 1)]) * 0.8
# 1-2s: 右声道 5000Hz 
y[1, (t >= 1) & (t < 2)] = np.sin(2 * np.pi * 5000 * t[(t >= 1) & (t < 2)]) * 0.6
# 2-3s: 双声道底噪 + Click
mask3 = (t >= 2) & (t < 3)
y[0, mask3] = np.random.randn(np.sum(mask3)) * 0.1
y[1, mask3] = np.random.randn(np.sum(mask3)) * 0.1
y[0, int(2.5 * sr):int(2.5 * sr)+50] = 1.0
y[1, int(2.5 * sr):int(2.5 * sr)+50] = 1.0

# 导出音频
sf.write("test_target_3s.wav", y.T, sr)

# ==========================================
# 2. 长纸缝：真正的 DAW 立体声音轨形态 (3D 侧视剪影)
# ==========================================
plt.figure(figsize=(12, 4), facecolor='#1E1E1E') # DAW 的暗黑背景
ax = plt.gca()
ax.set_facecolor('#1E1E1E')

# 🌟 核心修正：在一个坐标系内，左声道向上，右声道向下！
# 取绝对值是因为我们要在 3D 里看的是它的“外边缘包络”
plt.fill_between(t, 0, np.abs(y[0]), color='#4A90E2', alpha=0.9, label="Left Channel (Top)")
plt.fill_between(t, 0, -np.abs(y[1]), color='#E02020', alpha=0.9, label="Right Channel (Bottom)")

# 画出中间的 0 dB 轴 (纸缝的中心轴)
plt.axhline(0, color='#FFFFFF', linewidth=1, alpha=0.5)

plt.title("Ground Truth: 3D Waveform Side View (Stereo Track)", color='white')
plt.xlabel("Time (s)", color='white')
plt.ylabel("Amplitude", color='white')
plt.ylim([-1.1, 1.1])
plt.tick_params(colors='white')
plt.grid(True, color='#333333', linestyle='--')
plt.tight_layout()
plt.savefig("Truth_1_Waveform_Stereo.png", facecolor='#1E1E1E', dpi=150)
print("✅ [1/2] 真正的立体声波形图已生成: Truth_1_Waveform_Stereo.png")

# ==========================================
# 3. 切纸刀截面：FabFilter 实时频域图 (EQ View)
# ==========================================
n_fft = 4096
hop_length = 512
freqs = librosa.fft_frequencies(sr=sr, n_fft=n_fft)

D_left = librosa.amplitude_to_db(np.abs(librosa.stft(y[0], n_fft=n_fft, hop_length=hop_length)), ref=np.max)
D_right = librosa.amplitude_to_db(np.abs(librosa.stft(y[1], n_fft=n_fft, hop_length=hop_length)), ref=np.max)

frame_0_5s = librosa.time_to_frames(0.5, sr=sr, hop_length=hop_length)
frame_1_5s = librosa.time_to_frames(1.5, sr=sr, hop_length=hop_length)

plt.figure(figsize=(12, 6), facecolor='#1E1E1E')

# T = 0.5s 截面
ax1 = plt.subplot(2, 1, 1)
ax1.set_facecolor('#1E1E1E')
plt.plot(freqs, D_left[:, frame_0_5s], label="Left Channel (100Hz)", color='#4A90E2')
plt.plot(freqs, D_right[:, frame_0_5s], label="Right Channel", color='#E02020', alpha=0.3)
plt.xscale('log')
plt.xlim([20, 20000])
plt.ylim([-80, 5])
plt.title("Spectrum Slice at T=0.5s (Look along X-axis)", color='white')
plt.ylabel("Amplitude (dB)", color='white')
plt.tick_params(colors='white')
plt.grid(True, which="both", color='#333333', linestyle='--')
plt.legend()

# T = 1.5s 截面
ax2 = plt.subplot(2, 1, 2)
ax2.set_facecolor('#1E1E1E')
plt.plot(freqs, D_left[:, frame_1_5s], label="Left Channel", color='#4A90E2', alpha=0.3)
plt.plot(freqs, D_right[:, frame_1_5s], label="Right Channel (5000Hz)", color='#E02020')
plt.xscale('log')
plt.xlim([20, 20000])
plt.ylim([-80, 5])
plt.title("Spectrum Slice at T=1.5s", color='white')
plt.xlabel("Frequency (Hz)", color='white')
plt.ylabel("Amplitude (dB)", color='white')
plt.tick_params(colors='white')
plt.grid(True, which="both", color='#333333', linestyle='--')
plt.legend()

plt.tight_layout()
plt.savefig("Truth_2_FabFilter_EQ_Dark.png", facecolor='#1E1E1E', dpi=150)
print("✅ [2/2] 频域截面图已生成: Truth_2_FabFilter_EQ_Dark.png")