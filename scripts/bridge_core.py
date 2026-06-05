import argparse
import copy
import json
import queue
import socket
import threading
from collections import deque
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any, Deque, Dict, List, Optional, Set

import zmq


def _disable_udp_connreset(sock: socket.socket) -> None:
    """Prevent Windows UDP ICMP port-unreachable resets from killing the bridge."""
    if not hasattr(socket, "SIO_UDP_CONNRESET"):
        return
    try:
        sock.ioctl(socket.SIO_UDP_CONNRESET, False)
    except OSError:
        pass


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

    def debug(self, message: str):
        self._store("DEBUG", message)
        if self.verbose:
            self._safe_print(f"[bridge][debug] {message}")

    def warn(self, message: str):
        self._store("WARN", message)
        self._safe_print(message if self.verbose else f"[bridge][warn] {message}")

    def error(self, message: str):
        self._store("ERROR", message)
        self._safe_print(f"[bridge][error] {message}")


class VitShadowProject:
    """
    Python 侧影子工程：在盲转 ZMQ PUB → UDP 的同时，维护一份可查询的增量视图。
    - state：最近一次 get_project_state 全量快照 + 按 UID 合并的 delta 属性。
    - orphan_deltas：非快照轨 UID 的近期 delta 历史（按 UID 定长 deque，防内存膨胀）；最新值已在 nodes_by_uid[].delta_properties。
    """

    ORPHAN_DELTA_HISTORY_MAXLEN = 20

    def __init__(self, logger: BridgeLogger):
        self._logger = logger
        self._lock = threading.Lock()
        self.state: Dict[str, Any] = {}
        self._initialized: bool = False
        self._orphans_pre_init: List[Dict[str, Any]] = []
        # uid -> 定长历史队列（仅调试/对齐；最新状态见 nodes_by_uid）
        self.orphan_deltas: Dict[str, Deque[Dict[str, Any]]] = {}
        self._bootstrap_uids: Set[str] = set()

    @staticmethod
    def _is_high_frequency_transport_position_noise(uid: str, action: str) -> bool:
        return uid == "TRANSPORT" and action == "property_changed:position"

    def _append_orphan_delta_history(self, uid: str, delta: dict) -> None:
        q = self.orphan_deltas.get(uid)
        if q is None:
            q = deque(maxlen=self.ORPHAN_DELTA_HISTORY_MAXLEN)
            self.orphan_deltas[uid] = q
        q.append(copy.deepcopy(delta))

    def initialize_state(self, full_state_json: dict) -> None:
        """用 C++ 返回的完整工程 JSON 覆盖影子根状态，并建立轨级 UID 索引。"""
        with self._lock:
            self.state = {
                "engine_snapshot": copy.deepcopy(full_state_json),
                "nodes_by_uid": {},
            }
            self._bootstrap_uids = self._collect_track_uids_from_snapshot(full_state_json)
            self._initialized = True
            pending = self._orphans_pre_init[:]
            self._orphans_pre_init.clear()

            for d in pending:
                self._apply_delta_locked(d, from_replay=True)

            tracks_n = len(full_state_json.get("tracks") or [])
            path = full_state_json.get("project_path", "")
            self._logger.info(
                f"[shadow] 影子树已初始化: tracks={tracks_n} project_path={path!r} "
                f"重放待处理 delta={len(pending)} 条"
            )

    def _collect_track_uids_from_snapshot(self, snap: dict) -> Set[str]:
        out: Set[str] = set()
        for t in snap.get("tracks") or []:
            if not isinstance(t, dict):
                continue
            raw = t.get("track_id")
            if raw is None or (isinstance(raw, str) and raw.strip() == ""):
                raw = t.get("id")
            if raw is None:
                continue
            s = str(raw).strip()
            if s:
                out.add(s)
        return out

    def apply_delta(self, delta: dict) -> None:
        """线程安全入口；由 telemetry 线程调用。"""
        with self._lock:
            self._apply_delta_locked(delta, from_replay=False)

    def _apply_delta_locked(self, delta: dict, *, from_replay: bool) -> None:
        if not isinstance(delta, dict):
            return
        if delta.get("type") != "delta_update":
            return

        if not self._initialized:
            self._orphans_pre_init.append(copy.deepcopy(delta))
            uid_pi = str(delta.get("target_uid", "")).strip()
            action_pi = str(delta.get("action", "")).strip()
            if not self._is_high_frequency_transport_position_noise(uid_pi, action_pi):
                self._logger.debug(
                    f"[shadow] 缓冲 delta（尚未全量初始化） seq_id={delta.get('seq_id')} "
                    f"uid={delta.get('target_uid')!r} action={delta.get('action')!r}"
                )
            return

        uid = str(delta.get("target_uid", "")).strip()
        action = str(delta.get("action", "")).strip()
        value = delta.get("value")
        seq_id = delta.get("seq_id")
        timestamp = delta.get("timestamp")

        if not uid:
            return

        nodes: Dict[str, Any] = self.state.setdefault("nodes_by_uid", {})
        entry = nodes.setdefault(
            uid,
            {
                "delta_properties": {},
                "last_seq_id": None,
                "last_timestamp": None,
                "last_action": None,
            },
        )

        prop_key = self._parse_property_key(action)
        if prop_key is not None:
            entry["delta_properties"][prop_key] = value
        entry["last_seq_id"] = seq_id
        entry["last_timestamp"] = timestamp
        entry["last_action"] = action

        if uid not in self._bootstrap_uids:
            self._append_orphan_delta_history(uid, delta)
            if not self._is_high_frequency_transport_position_noise(uid, action):
                tag = "重放" if from_replay else "实时"
                self._logger.debug(
                    f"[shadow] [{tag}] 非快照轨 UID，已写入 nodes 并记入 orphan_deltas: uid={uid!r} action={action!r}"
                )
        else:
            if not self._is_high_frequency_transport_position_noise(uid, action):
                tag = "重放" if from_replay else "实时"
                self._logger.debug(
                    f"[shadow] [{tag}] 打补丁 uid={uid!r} prop={prop_key!r} seq_id={seq_id} action={action!r}"
                )

    @staticmethod
    def _parse_property_key(action: str) -> Optional[str]:
        prefix = "property_changed:"
        if action.startswith(prefix):
            return action[len(prefix) :].strip() or None
        return None

    def snapshot_summary(self) -> str:
        """便于调试的简短描述（持锁复制计数）。"""
        with self._lock:
            n_nodes = len(self.state.get("nodes_by_uid") or {})
            n_orphan_uids = len(self.orphan_deltas)
            init = self._initialized
        return f"initialized={init} nodes_by_uid={n_nodes} orphan_uid_keys={n_orphan_uids}"


def _build_req_socket(ctx: zmq.Context, cfg: BridgeConfig):
    sock = ctx.socket(zmq.REQ)
    sock.connect(cfg.zmq_req_url)
    sock.setsockopt(zmq.RCVTIMEO, cfg.req_timeout_ms)
    sock.setsockopt(zmq.SNDTIMEO, cfg.req_timeout_ms)
    sock.setsockopt(zmq.LINGER, 0)
    return sock


def _shadow_refresh_after_recording_stop(
    shadow: VitShadowProject,
    zmq_req,
    cfg: BridgeConfig,
    logger: BridgeLogger,
) -> None:
    """On recording_stopped PUB: mandatory full get_project_state for AI/shadow parity (V0.6)."""
    payload = json.dumps({"cmd": "get_project_state"}, ensure_ascii=False)
    try:
        zmq_req.send_string(payload)
        reply = zmq_req.recv_string()
    except Exception as exc:
        logger.warn(f"[shadow] recording_stopped -> get_project_state failed: {exc}")
        return
    try:
        reply_obj = json.loads(reply)
        if isinstance(reply_obj, dict) and reply_obj.get("status") == "ok":
            shadow.initialize_state(reply_obj)
            logger.info("[shadow] recording_stopped -> shadow re-initialized from get_project_state")
        else:
            logger.warn(
                f"[shadow] recording_stopped refresh got non-ok: "
                f"{reply_obj.get('status') if isinstance(reply_obj, dict) else reply_obj!r}"
            )
    except json.JSONDecodeError:
        logger.warn("[shadow] recording_stopped refresh: reply JSON parse error")


def run_bridge(cfg: BridgeConfig):
    logger = BridgeLogger(cfg.verbose, cfg.last_log_path, cfg.keep_last_log_lines)
    shadow = VitShadowProject(logger)
    context = zmq.Context()
    recv_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    _disable_udp_connreset(recv_sock)
    recv_sock.bind((cfg.godot_ip, cfg.udp_from_godot))
    zmq_req = _build_req_socket(context, cfg)
    shadow_refresh_queue: queue.Queue = queue.Queue()

    def bridge_telemetry():
        zmq_sub = context.socket(zmq.SUB)
        zmq_sub.connect(cfg.zmq_sub_url)
        zmq_sub.setsockopt_string(zmq.SUBSCRIBE, "")
        send_sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        _disable_udp_connreset(send_sock)
        udp_send_fail_count = 0
        delta_seq_gap_count = 0
        tile_ready_count = 0
        last_delta_seq: Optional[int] = None
        logger.info(f"telemetry thread started: {cfg.zmq_sub_url} -> UDP:{cfg.udp_to_godot}")
        try:
            while True:
                msg = zmq_sub.recv_string()
                out_bytes = msg.encode("utf-8")
                try:
                    d = json.loads(msg)
                    if isinstance(d, dict) and d.get("type") == "delta_update":
                        shadow.apply_delta(d)
                        try:
                            seq = int(d.get("seq_id", 0))
                        except (TypeError, ValueError):
                            seq = 0
                        if seq > 0:
                            if last_delta_seq is not None and seq != last_delta_seq + 1:
                                delta_seq_gap_count += 1
                                logger.warn(
                                    f"[telemetry] delta seq gap #{delta_seq_gap_count}: "
                                    f"last={last_delta_seq} current={seq}"
                                )
                            last_delta_seq = seq
                    elif isinstance(d, dict) and d.get("topic") == "recording" and str(d.get("subtopic", "")).strip() == "recording_stopped":
                        try:
                            shadow_refresh_queue.put_nowait(1)
                        except Exception:
                            pass
                    elif isinstance(d, dict) and d.get("command") == "tile_ready":
                        tile_ready_count += 1
                        logger.debug(
                            "[telemetry] tile_ready forward "
                            f"#{tile_ready_count} track={d.get('track_id', '')!r} "
                            f"clip={d.get('clip_id', '')!r} key={d.get('bake_key', '')!r} "
                            f"gen={d.get('generation', '')!r} tile={d.get('tile_index', '')!r} "
                            f"handles={d.get('handle_count', '')!r} bytes={d.get('shm_bytes', '')!r} "
                            f"shm={d.get('shared_memory', '')!r}"
                        )
                    # 规范化为严格 JSON 单行，减少 JUCE 等与 Godot JSON 解析差异导致的失败
                    out_bytes = json.dumps(d, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
                except (json.JSONDecodeError, TypeError, ValueError):
                    pass
                try:
                    send_sock.sendto(out_bytes, (cfg.godot_ip, cfg.udp_to_godot))
                except OSError as exc:
                    udp_send_fail_count += 1
                    if udp_send_fail_count == 1 or udp_send_fail_count % 100 == 0:
                        logger.warn(
                            f"[telemetry] UDP sendto failed count={udp_send_fail_count} "
                            f"dest={cfg.godot_ip}:{cfg.udp_to_godot} error={exc}"
                        )
        except Exception as exc:
            logger.warn(f"telemetry thread exited: {exc}")

    t = threading.Thread(target=bridge_telemetry, daemon=True)
    t.start()
    logger.info(f"control loop started: UDP:{cfg.udp_from_godot} -> {cfg.zmq_req_url}")
    recv_sock.settimeout(0.05)

    try:
        while True:
            pending_shadow = False
            while True:
                try:
                    shadow_refresh_queue.get_nowait()
                    pending_shadow = True
                except queue.Empty:
                    break
            if pending_shadow:
                _shadow_refresh_after_recording_stop(shadow, zmq_req, cfg, logger)

            try:
                data, addr = recv_sock.recvfrom(65535)
            except socket.timeout:
                continue
            except ConnectionResetError:
                logger.warn("control loop UDP recv reset; continuing")
                continue
            except OSError as exc:
                logger.warn(f"control loop UDP recv error; continuing: {exc}")
                continue

            if not data:
                continue

            try:
                payload = data.decode("utf-8")
                parsed_cmd: Optional[dict] = None
                try:
                    parsed_cmd = json.loads(payload)
                except json.JSONDecodeError:
                    parsed_cmd = None

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

                if (
                    parsed_cmd is not None
                    and str(parsed_cmd.get("cmd", "")).strip() == "get_project_state"
                    and reply is not None
                ):
                    try:
                        reply_obj = json.loads(reply)
                        if isinstance(reply_obj, dict) and reply_obj.get("status") == "ok":
                            shadow.initialize_state(reply_obj)
                        else:
                            logger.debug(
                                f"[shadow] get_project_state 应答非 ok，跳过影子初始化: "
                                f"status={reply_obj.get('status')!r}"
                                if isinstance(reply_obj, dict)
                                else "[shadow] get_project_state 应答非 JSON 对象，跳过"
                            )
                    except json.JSONDecodeError:
                        logger.debug("[shadow] get_project_state 应答 JSON 解析失败，跳过影子初始化")

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
