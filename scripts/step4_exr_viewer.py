import os
# 🌟 同样，必须在 import cv2 之前解封！
os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"

import cv2
import numpy as np
import matplotlib.pyplot as plt

file_path = "baked_tiles_test/tile_000_core.exr"

print("🔍 [Vit-DAW] 正在启动底层 EXR 数据核验中心...")

if not os.path.exists(file_path):
    print(f"❌ 找不到文件: {file_path}")
else:
    # 🌟 IMREAD_UNCHANGED: 强迫 OpenCV 读出所有通道
    exr_data = cv2.imread(file_path, cv2.IMREAD_UNCHANGED)
    
    print(f"✅ 成功读取！真实物理尺寸: 宽={exr_data.shape[1]}, 高={exr_data.shape[0]}, 通道数={exr_data.shape[2]}")

    if exr_data.shape[2] < 4:
        print("❌ 警告：依然没有4个通道！")
    else:
        # BGRA 拆解
        B_phase_diff = exr_data[:, :, 0]
        G_right_freq = exr_data[:, :, 1]
        R_left_freq  = exr_data[:, :, 2]
        A_envelope   = exr_data[:, :, 3]

        fig, axes = plt.subplots(4, 1, figsize=(14, 12), sharex=True)
        fig.suptitle("Vit-DAW: 500x336 EXR Data Matrix Inspector", color='white', fontsize=18, fontweight='bold')
        fig.patch.set_facecolor('#1E1E1E')

        def setup_ax(ax, data, title, cmap):
            ax.set_facecolor('#1E1E1E')
            im = ax.imshow(data, aspect='auto', origin='lower', cmap=cmap)
            ax.set_title(title, color='white', fontsize=12)
            ax.tick_params(colors='white')
            ax.set_ylabel("Freq (Y)", color='white')
            fig.colorbar(im, ax=ax, fraction=0.02, pad=0.01)

        setup_ax(axes[0], R_left_freq, "🔴 R Channel: Left Channel CQT (Red=Low, Yellow=High)", 'magma')
        setup_ax(axes[1], G_right_freq, "🟢 G Channel: Right Channel CQT", 'viridis')
        setup_ax(axes[2], B_phase_diff, "🔵 B Channel: Phase Difference", 'coolwarm')

        axes[3].set_facecolor('#1E1E1E')
        im_a = axes[3].imshow(A_envelope, aspect='auto', origin='lower', cmap='gray')
        axes[3].plot(A_envelope[0, :], color='#00FFFF', linewidth=2, label="Extracted 1D Peak Envelope")
        axes[3].set_title("⚪ A Channel: Volume Envelope Matrix (Vertical Stripes)", color='white', fontsize=12)
        axes[3].set_xlabel("Time Frames (X: 0 -> 500)", color='white', fontsize=12)
        axes[3].set_ylabel("Freq (Y)", color='white')
        axes[3].tick_params(colors='white')
        axes[3].legend(loc='upper right')
        fig.colorbar(im_a, ax=axes[3], fraction=0.02, pad=0.01)

        plt.tight_layout()
        plt.show()