import librosa
import numpy as np
import imageio
import os

os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"

def bake_exdr_texture(wav_path, output_path="output_spectrum.exr"):
    print(f"🚀 开始提纯音频地貌: {wav_path}")
    
    # 1. 加载音频 (不重采样，保持原始品质)
    y, sr = librosa.load(wav_path, sr=None, mono=False)
    if y.ndim == 1: # 强制转为双声道处理
        y = np.stack([y, y])
    
    left, right = y[0], y[1]

    # 2. 参数设置 (决定 3D 地形的分辨率)
    n_fft = 2048        # 频域轴分辨率 (对应 Shader 的 Z 轴)
    hop_length = 512    # 时间轴步长 (值越小，山脉越细长)
    
    # 3. 提取频域振幅 (R & G 通道)
    stft_l = np.abs(librosa.stft(left, n_fft=n_fft, hop_length=hop_length))
    stft_r = np.abs(librosa.stft(right, n_fft=n_fft, hop_length=hop_length))
    
    # 4. 提取相位差 (B 通道 - 哈斯监督核心)
    # 利用复数域的相位角差值，映射到 0.0 (同相) 到 1.0 (反相)
    stft_l_c = librosa.stft(left, n_fft=n_fft, hop_length=hop_length)
    stft_r_c = librosa.stft(right, n_fft=n_fft, hop_length=hop_length)
    phase_l = np.angle(stft_l_c)
    phase_r = np.angle(stft_r_c)
    phase_diff = np.abs(phase_l - phase_r) / np.pi # 归一化到 0-1

    # 5. 提取包络轮廓 (A 通道 - 视觉对位参考)
    # 提取每个时间点的最高振幅作为轮廓保护
    envelope = np.max(stft_l + stft_r, axis=0, keepdims=True)
    a_channel = np.repeat(envelope, stft_l.shape[0], axis=0)

    # 6. 数据归一化与组装 (RGBA32F 格式)
    # 我们使用 32位浮点数，不进行破坏性的 8bit 压缩
    # R: 左, G: 右, B: 相位, A: 轮廓
    exdr_data = np.stack([stft_l, stft_r, phase_diff, a_channel], axis=-1)
    
    # 动态范围映射：确保最强振幅对应 1.0 高度
    max_val = np.max(exdr_data[:,:,:2]) # 只参考振幅进行归一化
    if max_val > 0:
        exdr_data[:,:,0:2] /= max_val
        exdr_data[:,:,3] /= np.max(exdr_data[:,:,3]) # A通道独立归一化

    # 7. 导出 EXR (工业级高动态范围图像)
    # 注意：Godot 4 能完美读取 .exr 格式作为浮点纹理
    imageio.imwrite(output_path, exdr_data.astype(np.float32))
    print(f"✨ 烘焙成功！生成的 3D 地貌图层已保存至: {output_path}")
    print(f"📊 纹理分辨率 (时间 x 频率): {exdr_data.shape[1]} x {exdr_data.shape[0]}")

if __name__ == "__main__":
    # 替换为你本地的测试音频路径
    test_wav = r"D:\test audio.wav" 
    if os.path.exists(test_wav):
        # 导出路径建议直接设在你的 Godot 项目文件夹内
        # 这样生成后 Godot 会立刻自动导入
        bake_exdr_texture(test_wav, output_path="spectrum_data.exr")
    else:
        print(f"❌ 错误：在路径 [{test_wav}] 未找到音频文件")
        print("💡 请检查 D 盘根目录下文件名称是否完全匹配（包括扩展名 .wav）")