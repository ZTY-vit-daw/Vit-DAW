import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"
exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)

B_phase_diff = exr_data[:, :, 0]
G_right_height = exr_data[:, :, 1] # 🌟 已经是绝对高度了！
R_left_height  = exr_data[:, :, 2] # 🌟 已经是绝对高度了！

W_FRAMES = exr_data.shape[1] 
N_BINS = exr_data.shape[0]   

X, Z = np.meshgrid(np.arange(W_FRAMES), np.arange(N_BINS))

# 直接读取，不需要再乘 A 通道！
Y_left = R_left_height
Y_right = -G_right_height

# 🌟 B通道应用：X轴中轴线(低频区)亮度增强
# 相位差数据会转化为纯白色高光，混合进基础颜色
C_left = np.zeros((N_BINS, W_FRAMES, 4))
C_left[..., 0] = 0.2 + B_phase_diff * 0.5 
C_left[..., 1] = 0.4 + B_phase_diff * 0.5
C_left[..., 2] = 1.0 
C_left[..., 3] = 1.0
C_left = np.clip(C_left, 0, 1)

C_right = np.zeros((N_BINS, W_FRAMES, 4))
C_right[..., 0] = 1.0 
C_right[..., 1] = 0.2 + B_phase_diff * 0.5
C_right[..., 2] = 0.2 + B_phase_diff * 0.5
C_right[..., 3] = 1.0
C_right = np.clip(C_right, 0, 1)

fig = plt.figure(figsize=(18, 6), facecolor='#1E1E1E')
fig.suptitle("Vit-DAW Ground Truth Verifier (Phase Illuminated)", color='white', fontsize=18)

def setup_ax(ax, elev, azim):
    ax.set_facecolor('#1E1E1E')
    ax.set_proj_type('ortho') 
    ax.plot_surface(X, Z, Y_left, facecolors=C_left, rstride=1, cstride=1, antialiased=False, shade=False)
    ax.plot_surface(X, Z, Y_right, facecolors=C_right, rstride=1, cstride=1, antialiased=False, shade=False)
    ax.set_box_aspect((5.0, 3.36, 2.0))
    ax.set_zlim(-1.0, 1.0)
    ax.set_axis_off()
    ax.view_init(elev=elev, azim=azim)

ax1 = fig.add_subplot(1, 3, 1, projection='3d'); setup_ax(ax1, 35, -55)
ax2 = fig.add_subplot(1, 3, 2, projection='3d'); setup_ax(ax2, 0, -90)
ax3 = fig.add_subplot(1, 3, 3, projection='3d'); setup_ax(ax3, 0, 0)

plt.tight_layout()
plt.show()