import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"
print("🚀 [Vit-DAW] 启动工业级 3D 正交全息投影仪 (防爆版)...")

exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)
if exr_data is None or exr_data.shape[2] < 4:
    print("❌ 无法读取 4 通道 EXR。")
    exit()

B_phase_diff = exr_data[:, :, 0]
G_right_freq = exr_data[:, :, 1]
R_left_freq  = exr_data[:, :, 2]
A_envelope   = exr_data[:, :, 3]

W_FRAMES = exr_data.shape[1] 
N_BINS = exr_data.shape[0]   

X, Z = np.meshgrid(np.arange(W_FRAMES), np.arange(N_BINS))

Y_left = R_left_freq * A_envelope
Y_right = -(G_right_freq * A_envelope)

# ==========================================
# 🌟 核心修复：强行锁死浮点数，满足 Matplotlib 的洁癖
# ==========================================
B_phase_diff = np.clip(B_phase_diff, 0.0, 1.0)

C_left = np.zeros((N_BINS, W_FRAMES, 4))
C_left[..., 0] = 0.1 
C_left[..., 1] = 0.3 + 0.7 * B_phase_diff 
C_left[..., 2] = 1.0 
C_left[..., 3] = 0.9 
C_left = np.clip(C_left, 0.0, 1.0) # 绝对锁死在 0-1

C_right = np.zeros((N_BINS, W_FRAMES, 4))
C_right[..., 0] = 1.0 
C_right[..., 1] = 0.1 + 0.7 * B_phase_diff 
C_right[..., 2] = 0.1 
C_right[..., 3] = 0.9 
C_right = np.clip(C_right, 0.0, 1.0) # 绝对锁死在 0-1

# ==========================================
# 渲染引擎
# ==========================================
fig = plt.figure(figsize=(18, 6), facecolor='#1E1E1E')
fig.suptitle("Vit-DAW Orthographic Hologram (Phase-Lit)", color='white', fontsize=18, fontweight='bold')

stride = 2 
z_limit = np.max(A_envelope) * 1.2 if np.max(A_envelope) > 0 else 1.0

def setup_ax(ax, title, elev, azim):
    ax.set_facecolor('#1E1E1E')
    ax.set_proj_type('ortho') # 工业级正交投影
    
    ax.plot_surface(X, Z, Y_left, facecolors=C_left, rstride=stride, cstride=stride, antialiased=False, shade=False)
    ax.plot_surface(X, Z, Y_right, facecolors=C_right, rstride=stride, cstride=stride, antialiased=False, shade=False)
    
    ax.set_title(title, color='#00FFFF', fontsize=14, pad=10)
    ax.set_zlim(-z_limit, z_limit)
    ax.set_axis_off()
    ax.view_init(elev=elev, azim=azim)

ax1 = fig.add_subplot(1, 3, 1, projection='3d')
setup_ax(ax1, "1. 3D Ortho Canyon", elev=30, azim=-50)

ax2 = fig.add_subplot(1, 3, 2, projection='3d')
setup_ax(ax2, "2. Waveform View (Perfect Ortho)", elev=0, azim=-90)

ax3 = fig.add_subplot(1, 3, 3, projection='3d')
setup_ax(ax3, "3. EQ Slice (Perfect Ortho)", elev=0, azim=0)

plt.tight_layout()
plt.show()