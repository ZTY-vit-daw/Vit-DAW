import os
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"
import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"
print("🚀 [Vit-DAW] 启动零延迟 3D 三视图定焦仪...")

exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)
if exr_data is None or exr_data.shape[2] < 4:
    print("❌ 无法读取 4 通道 EXR，请确认烘焙是否成功。")
    exit()

G_right_freq = exr_data[:, :, 1]
R_left_freq  = exr_data[:, :, 2]
A_envelope   = exr_data[:, :, 3]

W_FRAMES = exr_data.shape[1] # X轴: 时间
N_BINS = exr_data.shape[0]   # Z轴: 频率

X, Z = np.meshgrid(np.arange(W_FRAMES), np.arange(N_BINS))

# 物理映射
Y_left = R_left_freq * A_envelope
Y_right = -(G_right_freq * A_envelope)

# 召唤宽屏画布
fig = plt.figure(figsize=(18, 6), facecolor='#1E1E1E')
fig.suptitle("Vit-DAW EXR 3D Static Views (Lag-Free)", color='white', fontsize=18, fontweight='bold')

# 渲染步长加大，拒绝卡顿
stride = 5 
max_amp = np.max(A_envelope)
z_limit = max_amp * 1.2 if max_amp > 0 else 1.0

def setup_ax(ax, title, elev, azim):
    ax.set_facecolor('#1E1E1E')
    ax.plot_surface(X, Z, Y_left, cmap='Blues', rstride=stride, cstride=stride, alpha=0.9, antialiased=False)
    ax.plot_surface(X, Z, Y_right, cmap='Reds', rstride=stride, cstride=stride, alpha=0.9, antialiased=False)
    ax.set_title(title, color='#00FFFF', fontsize=14, pad=10)
    ax.set_zlim(-z_limit, z_limit)
    ax.set_axis_off() # 关掉丑陋的坐标轴，纯享模型
    # 🌟 核心：锁定绝对视角
    ax.view_init(elev=elev, azim=azim)

# ==========================================
# 🎥 视角 1：上帝 3D 峡谷 (等轴测俯视)
# ==========================================
ax1 = fig.add_subplot(1, 3, 1, projection='3d')
setup_ax(ax1, "1. 3D Canyon\n(Elev: 25, Azim: -45)", elev=25, azim=-45)

# ==========================================
# 🎥 视角 2：长纸缝 (绝对侧视 / 时域波形)
# ==========================================
# 仰角0度（平视），方位角-90度（目光完全垂直于 Z 轴频段，只看 X 轴时间）
ax2 = fig.add_subplot(1, 3, 2, projection='3d')
setup_ax(ax2, "2. Waveform View\n(Elev: 0, Azim: -90)", elev=0, azim=-90)

# ==========================================
# 🎥 视角 3：宽纸缝 (绝对正视 / EQ 截面)
# ==========================================
# 仰角0度（平视），方位角0度（顺着 X 轴时间看过去，如同切蛋糕的横截面）
ax3 = fig.add_subplot(1, 3, 3, projection='3d')
setup_ax(ax3, "3. EQ Slice View\n(Elev: 0, Azim: 0)", elev=0, azim=0)

plt.tight_layout()
print("✅ 渲染完毕！请查看弹出的三视图窗口。")
plt.show()