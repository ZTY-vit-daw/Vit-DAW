import os
import numpy as np
import librosa
import imageio.v3 as iio

os.environ["OPENCV_IO_ENABLE_OPENEXR"] = "1"

INPUT_AUDIO = r"D:\test audio.wav" 
OUTPUT_FOLDER = "baked_tiles"
TILE_DURATION = 5.0 

def vit_baker_v5_pro(audio_path, output_folder):
    if not os.path.exists(output_folder): os.makedirs(output_folder)

    print("🎵 [Pro 引擎] 启动双通道独立烘焙...")
    y, sr = librosa.load(audio_path, sr=None, mono=False)
    if y.ndim == 1: y = np.stack([y, y]) 
    
    total_duration = librosa.get_duration(y=y, sr=sr)
    num_tiles = int(np.ceil(total_duration / TILE_DURATION))
    
    N_BINS = 336
    W_FRAMES = 500
    HOP_LENGTH = int(sr / 100) 

    for i in range(num_tiles):
        y_tile = y[:, int(i*TILE_DURATION*sr) : int(min((i+1)*TILE_DURATION*sr, y.shape[1]))]
        if y_tile.shape[1] < int(TILE_DURATION*sr):
            y_tile = np.pad(y_tile, ((0,0), (0, int(TILE_DURATION*sr) - y_tile.shape[1])))

        # 🌟 1. 提取真实双声道包络 (A通道)
        env_l = np.max(np.abs(y_tile[0].reshape(W_FRAMES, -1)), axis=1)
        env_r = np.max(np.abs(y_tile[1].reshape(W_FRAMES, -1)), axis=1)
        
        alpha_channel = np.zeros((N_BINS, W_FRAMES), dtype=np.float32)
        # 上半截存左声道，下半截存右声道 (168是 336的一半)
        alpha_channel[:168, :] = env_l
        alpha_channel[168:, :] = env_r

        # 🌟 2. 列归一化 CQT 光柱 (R/G通道)
        def get_normalized_cqt(sig):
            c = np.abs(librosa.cqt(sig, sr=sr, hop_length=HOP_LENGTH, fmin=librosa.note_to_hz('C1'), n_bins=N_BINS, bins_per_octave=48))
            c = c[:, :W_FRAMES]
            if c.shape[1] < W_FRAMES: c = np.pad(c, ((0,0), (0, W_FRAMES - c.shape[1])))
            
            # 核心：将每一帧的最高频率强制拉到 1.0，确保后续乘法能完美贴合包络
            col_max = np.max(c, axis=0, keepdims=True)
            col_max[col_max == 0] = 1e-10 
            c_norm = c / col_max
            # 加点指数次方，让山峰更锐利，砍掉底噪
            return np.power(c_norm, 1.5)

        r_data = get_normalized_cqt(y_tile[0]) 
        g_data = get_normalized_cqt(y_tile[1]) 

        # 🌟 3. 相位差 (B通道)
        stft_l = librosa.stft(y_tile[0], hop_length=HOP_LENGTH)
        stft_r = librosa.stft(y_tile[1], hop_length=HOP_LENGTH)
        p_diff = np.abs(np.angle(stft_l) - np.angle(stft_r))[:N_BINS, :W_FRAMES]
        if p_diff.shape[0] < N_BINS: p_diff = np.pad(p_diff, ((0, N_BINS - p_diff.shape[0]), (0, 0)))
        b_data = np.clip(p_diff / np.pi, 0, 1)

        exr_final = np.zeros((N_BINS, W_FRAMES, 4), dtype=np.float32)
        exr_final[:, :, 0] = r_data
        exr_final[:, :, 1] = g_data
        exr_final[:, :, 2] = b_data
        exr_final[:, :, 3] = alpha_channel # 立体声包络！

        out_path = os.path.join(output_folder, f"tile_{i:03d}.exr")
        iio.imwrite(out_path, exr_final, extension=".exr")
        print(f"✅ V5 立体声烘焙: {out_path}")

if __name__ == "__main__":
    vit_baker_v5_pro(INPUT_AUDIO, OUTPUT_FOLDER)