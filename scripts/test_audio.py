import json
import time
from pathlib import Path

import zmq


TEST_AUDIO_FILE = "D:/test audio.wav"  # 请用户在此填入真实路径。
TARGET_TRACK_ID = "1007"


def send_command(socket, payload):
    print(f"[REQ] Sending: {json.dumps(payload, ensure_ascii=False)}")
    socket.send_string(json.dumps(payload))
    reply = socket.recv_json()
    print(f"[REQ] Reply: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def resolve_test_audio_file():
    if any(ord(ch) < 32 for ch in TEST_AUDIO_FILE):
        raise ValueError(
            "TEST_AUDIO_FILE 包含转义后的控制字符。Windows 路径请改成原始字符串，例如 "
            'r"D:\\4_ZNXC_False_059.wav"' "，或使用正斜杠。"
        )

    path = Path(TEST_AUDIO_FILE)

    if not path.exists():
        raise FileNotFoundError(f"测试音频文件不存在: {path}")

    return path.as_posix()


def main():
    audio_file = resolve_test_audio_file()
    context = zmq.Context()
    socket = context.socket(zmq.REQ)
    socket.connect("tcp://127.0.0.1:5555")

    try:
        send_command(socket, {"cmd": "set_click", "enabled": False})
        send_command(
            socket,
            {
                "cmd": "add_audio_clip",
                "track_id": TARGET_TRACK_ID,
                "file_path": audio_file,
                "start_time": 0.0,
            },
        )
        send_command(socket, {"cmd": "play"})
        time.sleep(15)
        send_command(socket, {"cmd": "stop"})
    finally:
        socket.close(0)
        context.term()


if __name__ == "__main__":
    main()
