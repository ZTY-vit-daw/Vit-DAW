import json
import time

import zmq


def send_command(socket, payload):
    print(f"[PY] Sending: {json.dumps(payload, ensure_ascii=False)}")
    socket.send_string(json.dumps(payload))
    reply = socket.recv_json()
    print(f"[PY] Reply status: {reply.get('status')} | message: {reply.get('message')}")
    return reply


def print_project_state(state):
    print("[PY] Project state:")
    print(json.dumps(state, indent=2, ensure_ascii=False))


def main():
    context = zmq.Context()
    socket = context.socket(zmq.REQ)
    socket.connect("tcp://127.0.0.1:5555")

    try:
        project_state = send_command(socket, {"cmd": "get_project_state"})
        print_project_state(project_state)

        send_command(socket, {"cmd": "toggle_click"})
        send_command(socket, {"cmd": "play"})
        time.sleep(5)
        send_command(socket, {"cmd": "stop"})
    finally:
        socket.close(0)
        context.term()


if __name__ == "__main__":
    main()
