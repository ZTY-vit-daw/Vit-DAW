import json
import time

import zmq


def send_command(socket, payload):
    print(f"[PY] Sending: {json.dumps(payload)}")
    socket.send_string(json.dumps(payload))
    reply = socket.recv_json()
    print(f"[PY] Reply:   {json.dumps(reply)}")
    return reply


def main():
    context = zmq.Context()
    socket = context.socket(zmq.REQ)
    socket.connect("tcp://127.0.0.1:5555")

    try:
        send_command(socket, {"cmd": "ping"})
        time.sleep(2)
        send_command(socket, {"cmd": "reload_project"})

        before = send_command(socket, {"cmd": "list_tracks"})

        send_command(socket, {"cmd": "set_tempo", "bpm": 128.0})

        send_command(
            socket,
            {
                "cmd": "append_ghost_track",
                "track_name": "Epic Brass",
                "intent": "needs_reverb",
            },
        )

        after = send_command(socket, {"cmd": "list_tracks"})

        track_names_before = [track["name"] for track in before.get("tracks", [])]
        track_names_after = [track["name"] for track in after.get("tracks", [])]

        print(f"[PY] Tracks before: {track_names_before}")
        print(f"[PY] Tracks after:  {track_names_after}")

        if "Epic Brass" not in track_names_after:
            raise SystemExit("[PY] ERROR: Epic Brass was not found after append_ghost_track")

        print("[PY] SUCCESS: Epic Brass was appended and visible in list_tracks")
    finally:
        socket.close(0)
        context.term()


if __name__ == "__main__":
    main()
