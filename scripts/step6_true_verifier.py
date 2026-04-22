import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"
print("🚀 [Vit-DAW] 启动终极 3D 解算器 (完美还原 Step 2 物理空间)...")

exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)
if exr_data is None or exr_data.shape[2] < 4:
    print("❌ 无法读取 4 通道 EXR。")
    exit()

# 读取纯净的通道数据
G_right_freq = exr_data[:, :, 1]
R_left_freq  = exr_data[:, :, 2]
A_envelope   = exr_data[:, :, 3]

W_FRAMES = exr_data.shape[1] # X轴: 500帧
N_BINS = exr_data.shape[0]   # 单声道频段: 336

# ==========================================
# 🌟 核心修正：构建与 Step 2 完全一致的“并排”连续大网格
# Z 轴总宽度 = 336 (左) + 336 (右) = 672 个节点
# ==========================================
X, Z = np.meshgrid(np.arange(W_FRAMES), np.arange(N_BINS * 2))
Y_combined = np.zeros((N_BINS * 2, W_FRAMES))

# 前半截 (0 ~ 335)：铺设左声道 (R通道 * A通道)
Y_combined[:N_BINS, :] = R_left_freq * A_envelope

# 后半截 (336 ~ 671)：铺设右声道 (取负倒挂：-G通道 * A通道)
Y_combined[N_BINS:, :] = -(G_right_freq * A_envelope)

# ==========================================
# 渲染输出
# ==========================================
fig = plt.figure(figsize=(18, 6), facecolor='#1E1E1E')
fig.suptitle("Vit-DAW True Physical Reconstruction", color='white', fontsize=18, fontweight='bold')

stride = 4 
z_limit = np.max(A_envelope) * 1.2 if np.max(A_envelope) > 0 else 1.0

def setup_ax(ax, title, elev, azim):
    ax.set_facecolor('#1E1E1E')
    # 像 Step 2 一样，只画一个统一的连续大表面，用 coolwarm 区分上下！
    ax.plot_surface(X, Z, Y_combined, cmap='coolwarm', rstride=stride, cstride=stride, alpha=0.9, antialiased=False)
    ax.set_title(title, color='#00FFFF', fontsize=14, pad=10)
    ax.set_zlim(-z_limit, z_limit)
    ax.set_axis_off()
    ax.view_init(elev=elev, azim=azim)

ax1 = fig.add_subplot(1, 3, 1, projection='3d')
setup_ax(ax1, "1. 3D Canyon View", elev=25, azim=-60)

ax2 = fig.add_subplot(1, 3, 2, projection='3d')
setup_ax(ax2, "2. Waveform View (Side)", elev=0, azim=-90)

ax3 = fig.add_subplot(1, 3, 3, projection='3d')
setup_ax(ax3, "3. EQ Slice View (Front)", elev=0, azim=0)

plt.tight_layout()
plt.show()