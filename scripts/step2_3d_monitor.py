import numpy as np
import librosa
import matplotlib.pyplot as plt

print("🚀 [Vit-DAW 3D 观测器] 启动！正在构建全息空间...")

# 1. 加载测试音频
y, sr = librosa.load("test_target_3s.wav", sr=None, mono=False)
hop_length = int(sr / 100) # 10ms 精度
W_FRAMES = 300 # 3秒 * 100帧
N_BINS = 100 # 为了 3D 渲染流畅，我们将每个声道的频点精简到 100 个

# ==========================================
# 2. 核心算法：提取包络与归一化频率
# ==========================================
# 提取物理包络 (外壳)
env_l = np.max(np.abs(y[0][:W_FRAMES*hop_length].reshape(W_FRAMES, -1)), axis=1)
env_r = np.max(np.abs(y[1][:W_FRAMES*hop_length].reshape(W_FRAMES, -1)), axis=1)

def get_norm_cqt(sig):
    # 提取频率分布
    c = np.abs(librosa.cqt(sig, sr=sr, hop_length=hop_length, fmin=librosa.note_to_hz('C2'), n_bins=N_BINS, bins_per_octave=12))
    c = c[:, :W_FRAMES]
    # 列归一化：让最高峰强行顶到 1.0
    col_max = np.max(c, axis=0, keepdims=True)
    col_max[col_max == 0] = 1e-10
    return (c / col_max) ** 1.5 # 1.5次方让山峰更瘦削锐利

cqt_l = get_norm_cqt(y[0])
cqt_r = get_norm_cqt(y[1])

# ==========================================
# 3. 物理合体：构建 3D 表面数据
# ==========================================
# X轴：时间 (300 帧)
# Z轴：频率 (左声道 100 + 右声道 100 = 200 个深度节点)
X, Z = np.meshgrid(np.linspace(0, 3.0, W_FRAMES), np.linspace(0, 2, N_BINS * 2))

# Y轴：高度 (振幅)
Y = np.zeros_like(X)

# 🌟 核心映射法则：左声道向上 (+)，右声道向下 (-)
for i in range(W_FRAMES):
    # 左声道：Z轴的前半段 (0 到 100)，高度 = 频率光柱 * 左声道包络
    Y[:N_BINS, i] = cqt_l[:, i] * env_l[i]
    # 右声道：Z轴的后半段 (100 到 200)，高度 = 频率光柱 * 右声道包络 (并取负数！)
    Y[N_BINS:, i] = -(cqt_r[:, i] * env_r[i])

# ==========================================
# 4. 渲染 3D 交互窗口
# ==========================================
fig = plt.figure(figsize=(14, 8), facecolor='#1E1E1E')
plt.suptitle("Vit-DAW Pure Python 3D Monitor", color='white', fontsize=16)

# ---- 视图 1：3D 全局视角 ----
ax1 = fig.add_subplot(1, 2, 1, projection='3d')
ax1.set_facecolor('#1E1E1E')
ax1.plot_surface(X, Z, Y, cmap='coolwarm', edgecolor='none', alpha=0.9)
ax1.set_title("3D Hologram View", color='white')
ax1.set_xlabel("Time (X)", color='white')
ax1.set_ylabel("Frequency / L-R (Z)", color='white')
ax1.set_zlabel("Amplitude (Y)", color='white')
ax1.set_zlim(-1.1, 1.1)
ax1.tick_params(colors='white')
ax1.view_init(elev=30, azim=-60) # 经典的 3D 斜俯视

# ---- 视图 2：长纸缝视角 (绝对侧视) ----
ax2 = fig.add_subplot(1, 2, 2, projection='3d')
ax2.set_facecolor('#1E1E1E')
ax2.plot_surface(X, Z, Y, cmap='coolwarm', edgecolor='none', alpha=0.9)
ax2.set_title("Waveform View (Look along Z-axis)", color='white')
ax2.set_xlabel("Time (X)", color='white')
ax2.set_ylabel("Freq (Z) - Invisible", color='white')
ax2.set_zlabel("Amplitude (Y)", color='white')
ax2.set_zlim(-1.1, 1.1)
ax2.tick_params(colors='white')
# 🌟 纸缝理论验证：仰角为0，方位角为-90，目光完全垂直于 Z 轴！
ax2.view_init(elev=0, azim=-90) 

plt.tight_layout()
print("✅ 渲染完成！请在弹出的窗口中用鼠标拖拽 3D 模型！")
plt.show()