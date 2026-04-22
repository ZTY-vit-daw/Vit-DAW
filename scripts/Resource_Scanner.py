import hashlib
import json
import os
import re
from pathlib import Path
from typing import Dict, Iterable, List


DEFAULT_SUPPORTED_FORMATS = {
    "version": "1.0",
    "categories": {
        "aud": [".wav", ".mp3", ".flac", ".aiff", ".ogg", ".m4a", ".aac", ".w64"],
        "vst": [".vst3", ".dll", ".component", ".vst", ".clap", ".so"],
        "mid": [".mid", ".midi"],
    },
}


def generate_uid(file_path: str, prefix: str) -> str:
    base_name = os.path.splitext(os.path.basename(file_path))[0]

    # 强制转小写，非字母数字全替换为下划线，清理连续下划线
    safe_name = re.sub(r"[^a-z0-9]", "_", base_name.lower())
    safe_name = re.sub(r"_+", "_", safe_name).strip("_")
    if not safe_name:
        safe_name = "unnamed"

    # 绝对防碰撞：路径哈希
    abs_path = os.path.abspath(file_path)
    path_hash = hashlib.md5(abs_path.encode("utf-8")).hexdigest()[:4]

    return f"{prefix}:unknown:{safe_name}_{path_hash}"


class ResourceScanner:
    def __init__(self, scan_directories: Iterable[str]) -> None:
        self.scan_directories = [Path(directory) for directory in scan_directories]
        self.script_dir = Path(__file__).resolve().parent
        self.config_path = self.script_dir / "supported_formats.json"
        self.manifest_path = self.script_dir / "manifest.json"
        self.supported_formats = self._load_or_create_supported_formats()
        self.extension_map = self._build_extension_map()

    def _load_or_create_supported_formats(self) -> Dict[str, object]:
        if not self.config_path.exists():
            self._write_json(self.config_path, DEFAULT_SUPPORTED_FORMATS)

        with self.config_path.open("r", encoding="utf-8") as file:
            return json.load(file)

    def _build_extension_map(self) -> Dict[str, str]:
        extension_map: Dict[str, str] = {}
        categories = self.supported_formats.get("categories", {})

        for prefix, extensions in categories.items():
            for extension in extensions:
                normalized_extension = str(extension).lower()
                extension_map[normalized_extension] = str(prefix)

        return extension_map

    def _write_json(self, output_path: Path, data: Dict[str, object]) -> None:
        with output_path.open("w", encoding="utf-8") as file:
            json.dump(data, file, ensure_ascii=False, indent=2)

    def _normalize_path(self, file_path: Path) -> str:
        return file_path.resolve().as_posix()

    def _iter_files(self) -> Iterable[Path]:
        seen_paths = set()

        for scan_directory in self.scan_directories:
            resolved_directory = scan_directory.expanduser().resolve()
            if not resolved_directory.exists() or not resolved_directory.is_dir():
                print(f"[WARN] 跳过无效目录: {resolved_directory}")
                continue

            for root, _, files in os.walk(resolved_directory, followlinks=False):
                root_path = Path(root)
                for file_name in files:
                    file_path = root_path / file_name
                    normalized_path = str(file_path.resolve())
                    if normalized_path in seen_paths:
                        continue
                    seen_paths.add(normalized_path)
                    yield file_path

    def _build_resource_entry(self, file_path: Path) -> Dict[str, object]:
        absolute_path = self._normalize_path(file_path)
        suffix = file_path.suffix.lower()
        prefix = self.extension_map.get(suffix, "sys")
        subclass = "unknown" if prefix != "sys" else "unsupported"
        uid = generate_uid(absolute_path, prefix)
        if prefix == "sys":
            uid = uid.replace(":unknown:", ":unsupported:", 1)

        return {
            "uid": uid,
            "file_path": absolute_path,
            "display_name": file_path.stem,
            "type": prefix,
            "subclass": subclass,
            "meta_tags": ["auto_scanned"],
            "user_rating": 0,
            "state": "active",
        }

    def scan(self) -> Dict[str, object]:
        resources: Dict[str, Dict[str, object]] = {}

        for file_path in self._iter_files():
            resource_entry = self._build_resource_entry(file_path)
            resources[resource_entry["uid"]] = resource_entry

        manifest = {
            "manifest_version": str(self.supported_formats.get("version", "1.0")),
            "resources": resources,
        }
        self._write_json(self.manifest_path, manifest)
        return manifest


if __name__ == "__main__":
    current_script_dir = Path(__file__).resolve().parent
    test_directories: List[str] = [
        str(current_script_dir),
        str(current_script_dir.parent),
    ]

    scanner = ResourceScanner(test_directories)
    manifest = scanner.scan()
    print(f"[INFO] 扫描完成，共写入 {len(manifest['resources'])} 条资源。")
    print(f"[INFO] 配置文件: {scanner.config_path}")
    print(f"[INFO] 清单文件: {scanner.manifest_path}")
