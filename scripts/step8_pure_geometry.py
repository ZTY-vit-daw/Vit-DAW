import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"
print("🚀 [Vit-DAW] 启动纯几何物理矫正仪...")

exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)
G_right_freq = exr_data[:, :, 1]
R_left_freq  = exr_data[:, :, 2]
A_envelope   = exr_data[:, :, 3]

W_FRAMES = exr_data.shape[1] 
N_BINS = exr_data.shape[0]   

X, Z = np.meshgrid(np.arange(W_FRAMES), np.arange(N_BINS))
Y_left = R_left_freq * A_envelope
Y_right = -(G_right_freq * A_envelope)

fig = plt.figure(figsize=(18, 6), facecolor='#1E1E1E')
fig.suptitle("Vit-DAW Pure Geometry (True Aspect Ratio)", color='white', fontsize=18, fontweight='bold')

stride = 2 
z_limit = np.max(A_envelope) * 1.1 if np.max(A_envelope) > 0 else 1.0

def setup_ax(ax, title, elev, azim):
    ax.set_facecolor('#1E1E1E')
    ax.set_proj_type('ortho') 
    
    # 放弃花里胡哨的自定义颜色，用自带的平滑蓝红色系，只看形体！
    ax.plot_surface(X, Z, Y_left, cmap='Blues', rstride=stride, cstride=stride, antialiased=False)
    ax.plot_surface(X, Z, Y_right, cmap='Reds', rstride=stride, cstride=stride, antialiased=False)
    
    # 🌟 神级修复：告诉渲染器，X长500，Z宽336，Y高200，不要挤成正方体！
    ax.set_box_aspect((5.0, 3.36, 2.0))
    
    ax.set_title(title, color='#00FFFF', fontsize=14, pad=10)
    ax.set_zlim(-z_limit, z_limit)
    ax.set_axis_off()
    ax.view_init(elev=elev, azim=azim)

ax1 = fig.add_subplot(1, 3, 1, projection='3d')
setup_ax(ax1, "1. 3D Ortho Canyon", elev=35, azim=-55)

ax2 = fig.add_subplot(1, 3, 2, projection='3d')
setup_ax(ax2, "2. Waveform View (Perfect Ortho)", elev=0, azim=-90)

ax3 = fig.add_subplot(1, 3, 3, projection='3d')
setup_ax(ax3, "3. EQ Slice (Perfect Ortho)", elev=0, azim=0)

plt.tight_layout()
plt.show()