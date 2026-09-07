#!/usr/bin/env python3
"""SYMBOL-1 offline forensics: PE layout compare, frame symbolization, minidump walk.

Diagnostic-only tool for the SYMBOL-1 card (WEDGE-2 follow-up). It never starts
the kernel and never touches live processes: everything is file-based.

Subcommands:
  compare - section-by-section byte compare of two PE images (offset
            mappability gate: Release binary vs /Zi rebuild).
  frames  - resolve VitApp+offset frame list to module!function+line using the
            rebuilt PDB (dbghelp SymFromAddrW + SymGetLineFromAddrW64).
  dump    - parse a minidump: walk every thread (StackWalk64 against dump
            memory), symbolize, and correlate stack-resident handle values
            against the HandleDataStream to type the object a waiter blocks on.
"""

from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import os
import struct
import sys

MAX_SYM_NAME = 256
SYMOPT_UNDNAME = 0x00000002
SYMOPT_DEFERRED_LOADS = 0x00000004
SYMOPT_LOAD_LINES = 0x00000010
IMAGE_FILE_MACHINE_AMD64 = 0x8664
AddrModeFlat = 3

dbghelp = ctypes.WinDLL("dbghelp.dll", use_last_error=True)

# ---------------------------------------------------------------------------
# dbghelp ctypes plumbing (mirrors render_wedge_probe.py)
# ---------------------------------------------------------------------------


class _SYMBOL_INFO_BASEW(ctypes.Structure):
    _fields_ = [
        ("SizeOfStruct", ctypes.c_uint32),
        ("TypeIndex", ctypes.c_uint32),
        ("Reserved", ctypes.c_uint64 * 2),
        ("Index", ctypes.c_uint32),
        ("Size", ctypes.c_uint32),
        ("ModBase", ctypes.c_uint64),
        ("Flags", ctypes.c_uint32),
        ("Value", ctypes.c_uint64),
        ("Address", ctypes.c_uint64),
        ("Register", ctypes.c_uint32),
        ("Scope", ctypes.c_uint32),
        ("Tag", ctypes.c_uint32),
        ("NameLen", ctypes.c_uint32),
        ("MaxNameLen", ctypes.c_uint32),
        ("Name", ctypes.c_wchar * 1),
    ]


_SYMBOL_INFO_BASE_SIZE = ctypes.sizeof(_SYMBOL_INFO_BASEW)


class SYMBOL_INFOW(ctypes.Structure):
    _fields_ = [
        ("SizeOfStruct", ctypes.c_uint32),
        ("TypeIndex", ctypes.c_uint32),
        ("Reserved", ctypes.c_uint64 * 2),
        ("Index", ctypes.c_uint32),
        ("Size", ctypes.c_uint32),
        ("ModBase", ctypes.c_uint64),
        ("Flags", ctypes.c_uint32),
        ("Value", ctypes.c_uint64),
        ("Address", ctypes.c_uint64),
        ("Register", ctypes.c_uint32),
        ("Scope", ctypes.c_uint32),
        ("Tag", ctypes.c_uint32),
        ("NameLen", ctypes.c_uint32),
        ("MaxNameLen", ctypes.c_uint32),
        ("Name", ctypes.c_wchar * MAX_SYM_NAME),
    ]


class IMAGEHLP_LINEW64(ctypes.Structure):
    _fields_ = [
        ("SizeOfStruct", ctypes.c_uint32),
        ("Key", ctypes.c_void_p),
        ("LineNumber", ctypes.c_uint32),
        ("FileName", ctypes.c_wchar_p),
        ("Address", ctypes.c_uint64),
    ]


class ADDRESS64(ctypes.Structure):
    _fields_ = [("Offset", ctypes.c_uint64), ("Segment", ctypes.c_uint16), ("Mode", ctypes.c_uint32)]


class STACKFRAME64_PREFIX(ctypes.Structure):
    _fields_ = [
        ("AddrPC", ADDRESS64),
        ("AddrReturn", ADDRESS64),
        ("AddrFrame", ADDRESS64),
        ("AddrStack", ADDRESS64),
        ("AddrBStore", ADDRESS64),
        ("FuncTableEntry", ctypes.c_void_p),
        ("Params", ctypes.c_uint64 * 4),
        ("Far", ctypes.c_bool),
        ("Virtual", ctypes.c_bool),
        ("Reserved", ctypes.c_uint64 * 3),
    ]


STACKFRAME64_SLACK = 512


class M128A(ctypes.Structure):
    _fields_ = [("Low", ctypes.c_uint64), ("High", ctypes.c_int64)]


class XSAVE_FORMAT(ctypes.Structure):
    _fields_ = [
        ("ControlWord", ctypes.c_uint16), ("StatusWord", ctypes.c_uint16),
        ("TagWord", ctypes.c_uint8), ("Reserved1", ctypes.c_uint8),
        ("ErrorOpcode", ctypes.c_uint16), ("ErrorOffset", ctypes.c_uint32),
        ("ErrorSelector", ctypes.c_uint16), ("Reserved2", ctypes.c_uint16),
        ("DataOffset", ctypes.c_uint32), ("DataSelector", ctypes.c_uint16),
        ("Reserved3", ctypes.c_uint16), ("MxCsr", ctypes.c_uint32),
        ("MxCsr_Mask", ctypes.c_uint32),
        ("FloatRegisters", M128A * 8), ("XmmRegisters", M128A * 16),
        ("Reserved4", ctypes.c_uint8 * 96),
    ]


class CONTEXT(ctypes.Structure):
    _fields_ = [
        ("P1Home", ctypes.c_uint64), ("P2Home", ctypes.c_uint64),
        ("P3Home", ctypes.c_uint64), ("P4Home", ctypes.c_uint64),
        ("P5Home", ctypes.c_uint64), ("P6Home", ctypes.c_uint64),
        ("ContextFlags", ctypes.c_uint32), ("MxCsr", ctypes.c_uint32),
        ("SegCs", ctypes.c_uint16), ("SegDs", ctypes.c_uint16),
        ("SegEs", ctypes.c_uint16), ("SegFs", ctypes.c_uint16),
        ("SegGs", ctypes.c_uint16), ("SegSs", ctypes.c_uint16),
        ("EFlags", ctypes.c_uint32),
        ("Dr0", ctypes.c_uint64), ("Dr1", ctypes.c_uint64), ("Dr2", ctypes.c_uint64),
        ("Dr3", ctypes.c_uint64), ("Dr6", ctypes.c_uint64), ("Dr7", ctypes.c_uint64),
        ("Rax", ctypes.c_uint64), ("Rcx", ctypes.c_uint64), ("Rdx", ctypes.c_uint64),
        ("Rbx", ctypes.c_uint64), ("Rsp", ctypes.c_uint64), ("Rbp", ctypes.c_uint64),
        ("Rsi", ctypes.c_uint64), ("Rdi", ctypes.c_uint64),
        ("R8", ctypes.c_uint64), ("R9", ctypes.c_uint64), ("R10", ctypes.c_uint64),
        ("R11", ctypes.c_uint64), ("R12", ctypes.c_uint64), ("R13", ctypes.c_uint64),
        ("R14", ctypes.c_uint64), ("R15", ctypes.c_uint64), ("Rip", ctypes.c_uint64),
        ("FltSave", XSAVE_FORMAT),
        ("VectorRegister", M128A * 26), ("VectorControl", ctypes.c_uint64),
        ("DebugControl", ctypes.c_uint64), ("LastBranchToRip", ctypes.c_uint64),
        ("LastBranchFromRip", ctypes.c_uint64), ("LastExceptionToRip", ctypes.c_uint64),
        ("LastExceptionFromRip", ctypes.c_uint64),
    ]


READ_MEM_CALLBACK = ctypes.WINFUNCTYPE(
    ctypes.c_bool,
    ctypes.c_void_p, ctypes.c_uint64, ctypes.c_void_p, ctypes.c_uint32,
    ctypes.POINTER(ctypes.c_uint32),
)

dbghelp.SymSetOptions.argtypes = [ctypes.c_uint32]
dbghelp.SymSetOptions.restype = ctypes.c_uint32
dbghelp.SymInitializeW.argtypes = [ctypes.c_void_p, ctypes.c_wchar_p, ctypes.c_bool]
dbghelp.SymInitializeW.restype = ctypes.c_bool
dbghelp.SymCleanup.argtypes = [ctypes.c_void_p]
dbghelp.SymCleanup.restype = ctypes.c_bool
dbghelp.SymLoadModuleExW.argtypes = [
    ctypes.c_void_p, ctypes.c_void_p, ctypes.c_wchar_p, ctypes.c_wchar_p,
    ctypes.c_uint64, ctypes.c_uint32, ctypes.c_void_p, ctypes.c_uint32,
]
dbghelp.SymLoadModuleExW.restype = ctypes.c_uint64
dbghelp.SymFromAddrW.argtypes = [
    ctypes.c_void_p, ctypes.c_uint64, ctypes.POINTER(ctypes.c_uint64),
    ctypes.POINTER(SYMBOL_INFOW),
]
dbghelp.SymFromAddrW.restype = ctypes.c_bool
dbghelp.SymGetLineFromAddrW64.argtypes = [
    ctypes.c_void_p, ctypes.c_uint64, ctypes.POINTER(ctypes.c_uint32),
    ctypes.POINTER(IMAGEHLP_LINEW64),
]
dbghelp.SymGetLineFromAddrW64.restype = ctypes.c_bool
dbghelp.StackWalk64.argtypes = [
    ctypes.c_uint32, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p,
    ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p,
]
dbghelp.StackWalk64.restype = ctypes.c_bool
dbghelp.SymFunctionTableAccess64.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
dbghelp.SymFunctionTableAccess64.restype = ctypes.c_void_p
dbghelp.SymGetModuleBase64.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
dbghelp.SymGetModuleBase64.restype = ctypes.c_uint64

kernel32 = ctypes.WinDLL("kernel32.dll", use_last_error=True)
kernel32.GetCurrentProcess.restype = ctypes.c_void_p


# ---------------------------------------------------------------------------
# Symbol session bound to a plain module list (no live process needed)
# ---------------------------------------------------------------------------


class SymbolSession:
    """dbghelp session over the current-process pseudo handle, invade=False."""

    def __init__(self, modules):
        # modules: list of (path, base, size)
        self.hproc = kernel32.GetCurrentProcess()
        self.ok = False
        self.error = None
        dbghelp.SymSetOptions(SYMOPT_UNDNAME | SYMOPT_DEFERRED_LOADS | SYMOPT_LOAD_LINES)
        if not dbghelp.SymInitializeW(self.hproc, None, False):
            self.error = f"SymInitializeW failed winerr={ctypes.get_last_error()}"
            return
        self.loaded = []
        for path, base, size in modules:
            if not os.path.exists(path):
                self.loaded.append({"path": path, "ok": False, "err": "file missing"})
                continue
            loaded_base = dbghelp.SymLoadModuleExW(self.hproc, None, path, None, base, size, None, 0)
            self.loaded.append({"path": path, "base": f"0x{base:x}", "ok": bool(loaded_base)})
        self.ok = True

    def symbolize(self, addr):
        """Return (name, disp, line_dict|None) for addr."""
        sym = SYMBOL_INFOW()
        sym.SizeOfStruct = _SYMBOL_INFO_BASE_SIZE
        sym.MaxNameLen = MAX_SYM_NAME
        disp = ctypes.c_uint64(0)
        name = None
        if dbghelp.SymFromAddrW(self.hproc, addr, ctypes.byref(disp), ctypes.byref(sym)):
            name = sym.Name[: sym.NameLen] if sym.NameLen else sym.Name
        line_disp = ctypes.c_uint32(0)
        line = IMAGEHLP_LINEW64()
        line.SizeOfStruct = ctypes.sizeof(IMAGEHLP_LINEW64)
        line_info = None
        if dbghelp.SymGetLineFromAddrW64(self.hproc, addr, ctypes.byref(line_disp), ctypes.byref(line)):
            line_info = {
                "file": os.path.basename(line.FileName) if line.FileName else None,
                "full_path": line.FileName,
                "line": int(line.LineNumber),
                "disp": int(line_disp.value),
            }
        return name, (int(disp.value) if name else None), line_info

    def cleanup(self):
        dbghelp.SymCleanup(self.hproc)


# ---------------------------------------------------------------------------
# PE parsing
# ---------------------------------------------------------------------------


def pe_sections(path):
    """Return (image_base, [ {name, va, vsize, raw_size, raw_ptr, sha256} ])."""
    with open(path, "rb") as fh:
        data = fh.read()
    if data[:2] != b"MZ":
        raise ValueError(f"{path}: not MZ")
    e_lfanew = struct.unpack_from("<I", data, 0x3C)[0]
    if data[e_lfanew : e_lfanew + 4] != b"PE\x00\x00":
        raise ValueError(f"{path}: not PE")
    coff = e_lfanew + 4
    num_sections = struct.unpack_from("<H", data, coff + 2)[0]
    opt_size = struct.unpack_from("<H", data, coff + 16)[0]
    opt = coff + 20
    magic = struct.unpack_from("<H", data, opt)[0]
    if magic != 0x20B:
        raise ValueError(f"{path}: not PE32+")
    image_base = struct.unpack_from("<Q", data, opt + 24)[0]
    sec_off = opt + opt_size
    sections = []
    for i in range(num_sections):
        off = sec_off + i * 40
        name = data[off : off + 8].split(b"\x00")[0].decode(errors="replace")
        vsize, vaddr, rsize, raddr = struct.unpack_from("<IIII", data, off + 8)
        raw = data[raddr : raddr + rsize]
        sections.append({
            "name": name,
            "va": vaddr,
            "vsize": vsize,
            "raw_size": rsize,
            "raw_ptr": raddr,
            "sha256": hashlib.sha256(raw).hexdigest().upper() if rsize else None,
        })
    return image_base, sections


def cmd_compare(args):
    base_a, secs_a = pe_sections(args.old)
    base_b, secs_b = pe_sections(args.new)
    result = {
        "old": {"path": args.old, "image_base": f"0x{base_a:x}", "file_sha256": hashlib.sha256(open(args.old, "rb").read()).hexdigest().upper()},
        "new": {"path": args.new, "image_base": f"0x{base_b:x}", "file_sha256": hashlib.sha256(open(args.new, "rb").read()).hexdigest().upper()},
        "sections": [],
        "text_identical": None,
        "mappable": None,
    }
    by_name_b = {s["name"]: s for s in secs_b}
    all_ok = True
    for s in secs_a:
        t = by_name_b.get(s["name"])
        row = {
            "section": s["name"],
            "old_raw_size": s["raw_size"],
            "new_raw_size": t["raw_size"] if t else None,
            "old_va": f"0x{s['va']:x}",
            "new_va": f"0x{t['va']:x}" if t else None,
            "identical_bytes": None,
        }
        if t and s["raw_size"] == t["raw_size"]:
            row["identical_bytes"] = s["sha256"] == t["sha256"]
        elif t:
            row["identical_bytes"] = False
        if s["name"] == ".text":
            result["text_identical"] = row["identical_bytes"]
        if row["identical_bytes"] is not True and s["name"] in (".text", ".rdata", ".data"):
            all_ok = all_ok and False if s["name"] == ".text" else all_ok
        result["sections"].append(row)
    # The gate for frame alignment is .text byte identity.
    result["mappable"] = bool(result["text_identical"])
    out = json.dumps(result, indent=1, ensure_ascii=False)
    print(out)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            fh.write(out)
    return 0 if result["mappable"] else 3


# ---------------------------------------------------------------------------
# Frame offset symbolization
# ---------------------------------------------------------------------------


def cmd_frames(args):
    base = int(args.base, 0)
    size = os.path.getsize(args.pdb_exe)
    session = SymbolSession([(args.pdb_exe, base, size)])
    if not session.ok:
        print(json.dumps({"error": session.error}))
        return 1
    offsets = []
    for tok in args.offsets.split(","):
        tok = tok.strip()
        if not tok:
            continue
        offsets.append(int(tok, 0))
    out = {"exe": args.pdb_exe, "base": f"0x{base:x}", "loads": session.loaded, "frames": []}
    for off in offsets:
        addr = base + off
        name, disp, line = session.symbolize(addr)
        out["frames"].append({
            "offset": f"0x{off:x}",
            "function": name,
            "disp": f"0x{disp:x}" if disp is not None else None,
            "line": line,
        })
    session.cleanup()
    text = json.dumps(out, indent=1, ensure_ascii=False)
    print(text)
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            fh.write(text)
    resolved = sum(1 for f in out["frames"] if f["function"])
    return 0 if resolved == len(offsets) else 4


# ---------------------------------------------------------------------------
# Minidump parsing + full thread walk
# ---------------------------------------------------------------------------

MINIDUMP_SIGNATURE = 0x504D444D  # 'MDMP'

# Stream types
ThreadListStream = 3
ModuleListStream = 4
MemoryListStream = 5
Memory64ListStream = 9
HandleDataStream = 11
UnloadedModuleListStream = 13
MemoryInfoListStream = 15


def read_dump_streams(dump_path):
    data = open(dump_path, "rb").read()
    if struct.unpack_from("<I", data, 0)[0] != MINIDUMP_SIGNATURE:
        raise ValueError("not a minidump")
    num_streams, directory_rva = struct.unpack_from("<II", data, 8)
    flags = struct.unpack_from("<Q", data, 8 + 8 + 8)[0]
    streams = {}
    for i in range(num_streams):
        stype, dsize, drva = struct.unpack_from("<III", data, directory_rva + i * 12)
        if stype and stype not in streams:
            streams[stype] = (dsize, drva)
    return data, streams, flags


def minidump_string(data, rva):
    if not rva:
        return None
    ln = struct.unpack_from("<I", data, rva)[0]
    return data[rva + 4 : rva + 4 + ln].decode("utf-16-le", errors="replace")


def parse_modules(data, streams):
    if ModuleListStream not in streams:
        return []
    _, rva = streams[ModuleListStream]
    n = struct.unpack_from("<I", data, rva)[0]
    mods = []
    off = rva + 4
    for _ in range(n):
        base, size_of_image, _cks, _tds, name_rva = struct.unpack_from("<QIIII", data, off)
        cv_size, cv_rva = struct.unpack_from("<II", data, off + 76)  # after VS_FIXEDFILEINFO(52)
        mods.append({
            "base": base,
            "size": size_of_image,
            "name": minidump_string(data, name_rva),
            "cv_rva": cv_rva,
            "cv_size": cv_size,
        })
        off += 108  # MINIDUMP_MODULE
    return mods


def parse_threads(data, streams):
    if ThreadListStream not in streams:
        return []
    _, rva = streams[ThreadListStream]
    n = struct.unpack_from("<I", data, rva)[0]
    threads = []
    off = rva + 4
    for _ in range(n):
        tid, suspend, pri_class, pri, teb = struct.unpack_from("<IIIIQ", data, off)
        stack_start, stack_size, stack_rva = struct.unpack_from("<QII", data, off + 24)
        ctx_size, ctx_rva = struct.unpack_from("<II", data, off + 40)
        threads.append({
            "tid": tid,
            "suspend_count": suspend,
            "teb": teb,
            "stack_start": stack_start,
            "stack_size": stack_size,
            "stack_rva": stack_rva,
            "ctx_size": ctx_size,
            "ctx_rva": ctx_rva,
        })
        off += 48  # MINIDUMP_THREAD
    return threads


def parse_memory_ranges(data, streams):
    """Return flat list of {start,size,file_rva} from MemoryList or Memory64List."""
    if MemoryListStream in streams:
        _, rva = streams[MemoryListStream]
        n = struct.unpack_from("<I", data, rva)[0]
        ranges = []
        off = rva + 4
        for _ in range(n):
            start, size, frva = struct.unpack_from("<QII", data, off)
            ranges.append({"start": start, "size": size, "file_rva": frva})
            off += 16
        return ranges
    if Memory64ListStream in streams:
        _, rva = streams[Memory64ListStream]
        n, base_rva = struct.unpack_from("<QQ", data, rva)
        ranges = []
        off = rva + 16
        cur = base_rva
        for _ in range(n):
            start, size = struct.unpack_from("<QQ", data, off)
            ranges.append({"start": start, "size": size, "file_rva": cur})
            cur += size
            off += 16
        return ranges
    return []


def parse_handle_stream(data, streams):
    if HandleDataStream not in streams:
        return []
    _, rva = streams[HandleDataStream]
    n = struct.unpack_from("<I", data, rva)[0]
    handles = []
    off = rva + 4
    for _ in range(n):
        handle, type_rva, obj_rva = struct.unpack_from("<QII", data, off)
        handles.append({
            "handle": handle,
            "type": minidump_string(data, type_rva) or "?",
            "object_name": minidump_string(data, obj_rva),
        })
        off += 32  # MINIDUMP_HANDLE_DESCRIPTOR_2
    return handles


class DumpMemory:
    """Read memory out of the dump's memory streams.

    MiniDumpNormal captures only thread stacks. StackWalk64 still needs to read
    loaded module image memory (unwind/code) from the target, so image-range
    reads fall back to the module's file on disk (RVA -> file offset via its
    section table). For VitApp the rebuilt /Zi binary is instruction-identical
    (verified by the compare gate) and is used in place of the dumped one.
    """

    def __init__(self, ranges, dump_data, module_files=None):
        self.ranges = sorted(ranges, key=lambda r: r["start"])
        self.dump_data = dump_data
        self.module_files = []  # list of (base, size, bytes, sections)
        for mf in module_files or []:
            if len(mf) == 3:
                self.add_module_file(*mf)
            else:
                self.module_files.append(mf)
        self.sections = []

    def add_module_file(self, base, size, path):
        try:
            with open(path, "rb") as fh:
                data = fh.read()
        except OSError:
            return False
        e = struct.unpack_from("<I", data, 0x3C)[0]
        coff = e + 4
        nsec = struct.unpack_from("<H", data, coff + 2)[0]
        optsz = struct.unpack_from("<H", data, coff + 16)[0]
        sec_off = coff + 20 + optsz
        secs = []
        for i in range(nsec):
            off = sec_off + i * 40
            _vsz, va, rsz, rptr = struct.unpack_from("<IIII", data, off + 8)
            secs.append((va, rsz, rptr))
        self.module_files.append((base, size, data, secs))
        return True

    def _read_module_image(self, addr, size):
        for base, msize, data, secs in self.module_files:
            if base <= addr < base + msize:
                rva = addr - base
                for va, rsz, rptr in secs:
                    if va <= rva < va + max(rsz, 1):
                        off = rptr + (rva - va)
                        return data[off : off + size]
                return b""
        return b""

    def read(self, addr, size):
        out = bytearray()
        need = addr
        while size > 0:
            rng = next((r for r in self.ranges if r["start"] <= need < r["start"] + r["size"]), None)
            if rng is None:
                break
            off = need - rng["start"]
            take = min(size, rng["size"] - off)
            chunk = self.dump_data[rng["file_rva"] + off : rng["file_rva"] + off + take]
            if not chunk:
                break
            out.extend(chunk)
            need += take
            size -= take
        if out:
            return bytes(out)
        return self._read_module_image(addr, size)


def walk_dump_threads(dump_path, data, threads, mem_ranges, session, max_frames=64, module_files=None):
    mem = DumpMemory(mem_ranges, data, module_files)

    def read_mem_cb(hproc, base_addr, buf, size, read_bytes):
        b = mem.read(base_addr, size)
        if b:
            ctypes.memmove(buf, b, len(b))
            if read_bytes:
                read_bytes[0] = len(b)
            return True
        if read_bytes:
            read_bytes[0] = 0
        return False

    cb = READ_MEM_CALLBACK(read_mem_cb)
    results = []
    for th in threads:
        ctx_bytes = data[th["ctx_rva"] : th["ctx_rva"] + th["ctx_size"]]
        ctx = CONTEXT.from_buffer_copy(ctx_bytes[: ctypes.sizeof(CONTEXT)] if len(ctx_bytes) >= ctypes.sizeof(CONTEXT) else ctx_bytes)
        ctx_buf = ctypes.create_string_buffer(bytes(ctx_bytes) + b"\x00" * 64)
        ctx_addr = (ctypes.addressof(ctx_buf) + 15) // 16 * 16
        ctypes.memmove(ctx_addr, bytes(ctx_bytes), len(ctx_bytes))

        frame_buf = ctypes.create_string_buffer(ctypes.sizeof(STACKFRAME64_PREFIX) + STACKFRAME64_SLACK)
        frame = ctypes.cast(ctypes.addressof(frame_buf), ctypes.POINTER(STACKFRAME64_PREFIX)).contents
        frame.AddrPC.Offset = ctx.Rip
        frame.AddrPC.Segment = 0
        frame.AddrPC.Mode = AddrModeFlat
        frame.AddrFrame.Offset = ctx.Rbp
        frame.AddrFrame.Segment = 0
        frame.AddrFrame.Mode = AddrModeFlat
        frame.AddrStack.Offset = ctx.Rsp
        frame.AddrStack.Segment = 0
        frame.AddrStack.Mode = AddrModeFlat
        frame.AddrBStore.Offset = 0
        frame.AddrBStore.Segment = 0
        frame.AddrBStore.Mode = AddrModeFlat

        frames = []
        while len(frames) < max_frames:
            ok = dbghelp.StackWalk64(
                IMAGE_FILE_MACHINE_AMD64,
                session.hproc,
                session.hproc,  # hThread: unused by our read callback
                ctypes.c_void_p(ctypes.addressof(frame_buf)),
                ctypes.c_void_p(ctx_addr),
                cb,
                ctypes.cast(dbghelp.SymFunctionTableAccess64, ctypes.c_void_p),
                ctypes.cast(dbghelp.SymGetModuleBase64, ctypes.c_void_p),
                None,
            )
            if not ok or frame.AddrPC.Offset == 0:
                break
            pc = frame.AddrPC.Offset
            name, disp, line = session.symbolize(pc)
            frames.append({
                "addr": f"0x{pc:x}",
                "function": name,
                "disp": f"0x{disp:x}" if disp is not None else None,
                "line": line,
            })
        results.append({
            "tid": th["tid"],
            "suspend_count": th["suspend_count"],
            "rip": f"0x{ctx.Rip:x}",
            "rsp": f"0x{ctx.Rsp:x}",
            "rcx": f"0x{ctx.Rcx:x}",
            "rdx": f"0x{ctx.Rdx:x}",
            "stack_start": f"0x{th['stack_start']:x}",
            "stack_size": th["stack_size"],
            "frames": frames,
        })
    return results


def cmd_dump(args):
    data, streams, flags = read_dump_streams(args.dump)
    modules = parse_modules(data, streams)
    threads = parse_threads(data, streams)
    mem_ranges = parse_memory_ranges(data, streams)
    handles = parse_handle_stream(data, streams)

    vit = next((m for m in modules if "vitapp" in os.path.basename(m["name"] or "").lower()), None)
    session_modules = []
    module_files = []  # (base, size, file path) for image-memory fallback
    if vit and args.pdb_exe:
        session_modules.append((args.pdb_exe, vit["base"], vit["size"]))
        module_files.append((vit["base"], vit["size"], args.pdb_exe))
    # Load every other dump module from its recorded path (read-only) so
    # StackWalk64 can find unwind info for system frames (ntdll/kernelbase/...).
    for m in modules:
        if m is vit:
            continue
        path = m["name"] or ""
        if path and os.path.exists(path):
            session_modules.append((path, m["base"], m["size"]))
            module_files.append((m["base"], m["size"], path))
    session = SymbolSession(session_modules)
    if not session.ok:
        print(json.dumps({"error": session.error}))
        return 1

    walked = walk_dump_threads(args.dump, data, threads, mem_ranges, session, module_files=module_files)

    # Handle correlation: scan each thread's stack memory for handle values.
    handle_types = {}
    for h in handles:
        handle_types[h["type"]] = handle_types.get(h["type"], 0) + 1
    interesting = {"Event", "Semaphore", "Mutant", "Section", "Job", "Timer"}
    by_value = {}
    for h in handles:
        by_value.setdefault(h["handle"], []).append(h["type"])

    def scan_stack(th):
        found = []
        for rng in sorted(mem_ranges, key=lambda r: r["start"]):
            if not (rng["start"] <= th["stack_start"] < rng["start"] + rng["size"]):
                continue
            stack_bytes = data[rng["file_rva"] : rng["file_rva"] + rng["size"]]
            for k in range(0, len(stack_bytes) - 8 + 1, 8):
                v = struct.unpack_from("<Q", stack_bytes, k)[0]
                if v in by_value and by_value[v][0] in interesting:
                    found.append({"stack_off": k, "value": f"0x{v:x}", "types": by_value[v]})
            break
        return found

    for w in walked:
        th = next(t for t in threads if t["tid"] == w["tid"])
        if handles:
            w["stack_handle_candidates"] = scan_stack(th)[:24]

    session.cleanup()
    out = {
        "dump": args.dump,
        "dump_flags": f"0x{flags:x}",
        "streams_present": sorted(k for k, v in streams.items() if v[0] > 0),
        "modules_count": len(modules),
        "vitapp": {
            "name": vit["name"] if vit else None,
            "base": f"0x{vit['base']:x}" if vit else None,
            "size": f"0x{vit['size']:x}" if vit else None,
            "pdb_exe_used": args.pdb_exe,
        },
        "thread_count": len(threads),
        "handle_stream_present": HandleDataStream in streams,
        "handle_type_histogram": handle_types,
        "threads": walked,
        "sym_load_ok": sum(1 for l in session.loaded if l.get("ok")),
        "sym_load_total": len(session.loaded),
        "errors": [],
    }
    text = json.dumps(out, indent=1, ensure_ascii=False)
    print(text[:4000])
    if args.json:
        with open(args.json, "w", encoding="utf-8") as fh:
            fh.write(text)
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("compare")
    p.add_argument("--old", required=True)
    p.add_argument("--new", required=True)
    p.add_argument("--json")

    p = sub.add_parser("frames")
    p.add_argument("--pdb-exe", required=True, help="rebuilt VitApp.exe (PDB located beside it)")
    p.add_argument("--base", required=True, help="module base used in the recorded stacks (0x...)")
    p.add_argument("--offsets", required=True, help="comma separated hex offsets")
    p.add_argument("--json")

    p = sub.add_parser("dump")
    p.add_argument("--dump", required=True)
    p.add_argument("--pdb-exe", required=True)
    p.add_argument("--json")
    p.add_argument("--all-handles", action="store_true")

    args = parser.parse_args()
    if args.cmd == "compare":
        sys.exit(cmd_compare(args))
    if args.cmd == "frames":
        sys.exit(cmd_frames(args))
    if args.cmd == "dump":
        sys.exit(cmd_dump(args))


if __name__ == "__main__":
    main()
