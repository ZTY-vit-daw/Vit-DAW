import argparse
import json
import socket
import threading
from collections import deque
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path

import zmq


@dataclass
class BridgeConfig:
    zmq_sub_url: str = "tcp://127.0.0.1:5556"
    zmq_req_url: str = "tcp://127.0.0.1:5555"
    godot_ip: str = "127.0.0.1"
    udp_to_godot: int = 4444
    udp_from_godot: int = 4445
    # VitApp scan_plugins / device scans can exceed a couple of seconds; 2s causes false "bridge timeout".
    req_timeout_ms: int = 180000
    req_max_retries: int = 1
    verbose: bool = False
    last_log_path: str = ""
    keep_last_log_lines: int = 300


class BridgeLogger:
    def __init__(self, verbose: bool, last_log_path: str, keep_last_log_lines: int):
        self.verbose = verbose
        self.last_log_path = Path(last_log_path) if last_log_path else None
        self._last_lines = deque(maxlen=max(10, keep_last_log_lines))
        if self.last_log_path:
            self.last_log_path.parent.mkdir(parents=True, exist_ok=True)

    def _safe_print(self, message: str):
        try:
            print(message)
        except UnicodeEncodeError:
            print(message.encode("ascii", errors="ignore").decode("ascii"))

    def _store(self, level: str, message: str):
        ts = datetime.now().isoformat(timespec="seconds")
        self._last_lines.append(f"{ts} [{level}] {message}")
        if self.last_log_path:
            self.last_log_path.write_text("\n".join(self._last_lines), encoding="utf-8")

    def info(self, message: str):
        self._store("INFO", message)
        if self.verbose:
            self._safe_print(message)

    def warn(self, message: str):
        self._store("WARN", message)
        self._safe_print(message if self.verbose else f"[bridge][warn] {message}")

    def error(self, message: str):
        self._store("ERROR", message)
        self._safe_print(f"[bridge][error] {message}")


def _build_req_socket(ctx: zmq.Context, cfg: BridgeConfig):
    sock = ctx.socket(zmq.REQ)
    sock.connect(cfg.zmq_req_url)
    sock.setsockopt(zmq.RCVTIMEO, cfg.req_timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, cfg.req_timeout_ms)
    sock.setsockopt(zmq.LINGER, 0)
    return sock


def run_bridge(cfg: BridgeConfig):
    logger = BridgeLogger(cfg.verbose, cfg.last_log_path, cfg.keep_last_log_lines)
    context = zmq.Context()
    recv_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    recv_sock.bind((cfg.godot_ip, cfg.udp_from_godot))
    zmq_req = _build_req_socket(context, cfg)

    def bridge_telemetry():
        zmq_sub = context.socket(zmq.SUB)
        zmq_sub.connect(cfg.zmq_sub_url)
        zmq_sub.setsockopt_string(zmq.SUBSCRIBE, "")
        send_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        logger.info(f"telemetry thread started: {cfg.zmq_sub_url} -> UDP:{cfg.udp_to_godot}")
        try:
            while True:
                msg = zmq_sub.recv_string()
                try:
                    send_sock.sendto(msg.encode("utf-8"), (cfg.godot_ip, cfg.udp_to_godot))
                except OSError:
                    pass
        except Exception as exc:
            logger.warn(f"telemetry thread exited: {exc}")

    t = threading.Thread(target=bridge_telemetry, daemon=True)
    t.start()
    logger.info(f"control loop started: UDP:{cfg.udp_from_godot} -> {cfg.zmq_req_url}")

    try:
        while True:
            try:
                data, addr = recv_sock.recvfrom(65535)
                if not data:
                    continue
                payload = data.decode("utf-8")
                reply = None
                for _ in range(cfg.req_max_retries + 1):
                    try:
                        zmq_req.send_string(payload)
                        reply = zmq_req.recv_string()
                        break
                    except zmq.Again:
                        try:
                            zmq_req.close(0)
                        except Exception:
                            pass
                        zmq_req = _build_req_socket(context, cfg)
                if reply is None:
                    reply = json.dumps({"status": "error", "message": "bridge timeout to VitApp"})
                try:
                    recv_sock.sendto(reply.encode("utf-8"), addr)
                except OSError:
                    pass
            except ConnectionResetError:
                continue
            except Exception as exc:
                logger.error(f"control loop error: {exc}")
    except KeyboardInterrupt:
        logger.info("bridge stopped by keyboard interrupt")
    finally:
        try:
            zmq_req.close(0)
        except Exception:
            pass
        recv_sock.close()
        context.term()
        logger.info("bridge shutdown complete")


def parse_args():
    parser = argparse.ArgumentParser(description="Vit-DAW bridge core runner")
    parser.add_argument("--verbose", action="store_true", help="enable verbose console logs")
    parser.add_argument("--last-log-path", default="", help="path to keep rolling last logs")
    parser.add_argument("--udp-to-godot", type=int, default=4444)
    parser.add_argument("--udp-from-godot", type=int, default=4445)
    parser.add_argument("--req-timeout-ms", type=int, default=180000)
    parser.add_argument("--req-max-retries", type=int, default=1)
    return parser.parse_args()


def main():
    args = parse_args()
    cfg = BridgeConfig(
        verbose=args.verbose,
        last_log_path=args.last_log_path,
        udp_to_godot=args.udp_to_godot,
        udp_from_godot=args.udp_from_godot,
        req_timeout_ms=args.req_timeout_ms,
        req_max_retries=args.req_max_retries,
    )
    run_bridge(cfg)


if __name__ == "__main__":
    main()
