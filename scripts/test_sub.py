import zmq
import json

def listen_to_delta():
    context = zmq.Context()
    socket = context.socket(zmq.SUB)
    socket.connect("tcp://127.0.0.1:5556")
    socket.setsockopt_string(zmq.SUBSCRIBE, "")

    print("🎧 正在监听 5556 端口的 Delta 广播...")
    
    while True:
        message = socket.recv_string()
        try:
            data = json.loads(message)
            # 只过滤出我们的增量事件，屏蔽原有的遥测数据
            if data.get("type") == "delta_update":
                print(f"📦 [Delta 捕获] Seq: {data.get('seq_id')} | Action: {data.get('action')} | UID: {data.get('target_uid')} | Value: {data.get('value')}")
        except json.JSONDecodeError:
            pass

if __name__ == "__main__":
    listen_to_delta()