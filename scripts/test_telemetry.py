import json
import threading
import time

import zmq


def send_command(socket, payload):
    print(f"[REQ] Sending: {json.dumps(payload, ensure_ascii=False)}")
    socket.send_string(json.dumps(payload))
    reply = socket.recv_json()
    print(f"[REQ] Reply: {json.dumps(reply, ensure_ascii=False)}")
    return reply


def telemetry_listener(stop_event):
    context = zmq.Context()
    socket = context.socket(zmq.SUB)
    socket.connect("tcp://127.0.0.1:5556")
    socket.setsockopt_string(zmq.SUBSCRIBE, "")
    socket.setsockopt(zmq.RCVTIMEO, 200)

    try:
        while not stop_event.is_set():
            try:
                message = socket.recv_json()
            except zmq.Again:
                continue

            topic = message.get("topic")

            if topic == "transport":
                is_playing = message.get("is_playing", False)
                position_seconds = message.get("position_seconds", 0.0)
                status = "PLAY" if is_playing else "STOP"
                print(f"\r[Transport] {status} position={position_seconds:8.3f}s", end="", flush=True)
                continue

            if topic == "levels":
                tracks = message.get("tracks", [])
                formatted_tracks = ", ".join(
                    f"{track.get('name', track.get('id', '?'))}={track.get('level_db', -100.0):6.1f} dB"
                    for track in tracks
                )
                print(f"\n[Levels] {formatted_tracks}", flush=True)
    finally:
        print()
        socket.close(0)
        context.term()


def control_worker(stop_event):
    context = zmq.Context()
    socket = context.socket(zmq.REQ)
    socket.connect("tcp://127.0.0.1:5555")

    try:
        # Give the SUB socket a moment to finish subscribing before transport starts.
        time.sleep(0.3)
        send_command(socket, {"cmd": "set_click", "enabled": True})
        send_command(socket, {"cmd": "play"})
        time.sleep(10)
        send_command(socket, {"cmd": "stop"})
    finally:
        stop_event.set()
        socket.close(0)
        context.term()


def main():
    stop_event = threading.Event()

    listener_thread = threading.Thread(target=telemetry_listener, args=(stop_event,), daemon=True)
    control_thread = threading.Thread(target=control_worker, args=(stop_event,))

    listener_thread.start()
    control_thread.start()
    control_thread.join()
    listener_thread.join(timeout=1.0)


if __name__ == "__main__":
    main()
