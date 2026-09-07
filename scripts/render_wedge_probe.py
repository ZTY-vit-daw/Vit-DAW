#!/usr/bin/env python3
"""WEDGE-2A diagnostic probe: ZMQ render-probe monitor + live thread stack capture.

Diagnostic-only tool for the WEDGE-2A card. It never writes to production trees;
all evidence goes to the --out-dir given by the orchestrator script
run_render_wedge_evidence.ps1.

Subcommands:
  monitor  - run one full experiment group: open project copy, send the probe
             request, observe PUB/pings/render file, capture A-phase stacks
             before the render watchdog deadline and B-phase stacks if the
             message thread dies after the watchdog.
  stacks   - capture symbolized thread stacks of a live PID (JSON output).
  wct      - capture Wait Chain Traversal info of a live PID (JSON output).
  dump     - write a minidump (stacks + handle data) of a live PID.
"""

from __future__ import annotations

import argparse
import ctypes
import json
import os
import subprocess
import sys
import tempfile
import time
import ctypes.wintypes as wt
from datetime import datetime, timezone

# ---------------------------------------------------------------------------
# Win32 constants
# ---------------------------------------------------------------------------

PROCESS_QUERY_INFORMATION = 0x0400
PROCESS_VM_READ = 0x0010
THREAD_SUSPEND_RESUME = 0x0002
THREAD_GET_CONTEXT = 0x0008
THREAD_QUERY_INFORMATION = 0x0040
TH32CS_SNAPMODULE = 0x00000008
TH32CS_SNAPMODULE32 = 0x00000010
TH32CS_SNAPTHREAD = 0x00000004
CONTEXT_AMD64 = 0x00100000
CONTEXT_CONTROL = CONTEXT_AMD64 | 0x00000001
CONTEXT_INTEGER = CONTEXT_AMD64 | 0x00000002
CONTEXT_FULL_FLAGS = CONTEXT_CONTROL | CONTEXT_INTEGER
IMAGE_FILE_MACHINE_AMD64 = 0x8664
AddrModeFlat = 3
MAX_SYM_NAME = 256
SYMOPT_UNDNAME = 0x00000002
SYMOPT_DEFERRED_LOADS = 0x00000004
SYMOPT_LOAD_LINES = 0x00000010
GENERIC_WRITE = 0x40000000
CREATE_ALWAYS = 2
FILE_ATTRIBUTE_NORMAL = 0x80
INVALID_HANDLE_VALUE = ctypes.c_void_p(-1).value or -1

# MiniDump types
MiniDumpNormal = 0x00000000
MiniDumpWithHandleData = 0x00000004
MiniDumpWithUnloadedModules = 0x00000020
MINIDUMP_TYPE = MiniDumpNormal | MiniDumpWithHandleData | MiniDumpWithUnloadedModules

WCT_MAX_NODE_COUNT = 16
WCT_OBJNAME_LENGTH = 128
WCTP_GETINFO_ALL_FLAGS = 0xF  # WCT_OUT_OF_PROC | COM | CS | NETWORK_IO

kernel32 = ctypes.WinDLL("kernel32.dll", use_last_error=True)
dbghelp = ctypes.WinDLL("dbghelp.dll", use_last_error=True)
ntdll = ctypes.WinDLL("ntdll.dll", use_last_error=True)
advapi32 = ctypes.WinDLL("advapi32.dll", use_last_error=True)


def _now():
    return time.monotonic()


def _wall():
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds")


# ---------------------------------------------------------------------------
# ctypes structures
# ---------------------------------------------------------------------------


class M128A(ctypes.Structure):
    _fields_ = [("Low", ctypes.c_uint64), ("High", ctypes.c_int64)]


class XSAVE_FORMAT(ctypes.Structure):
    _fields_ = [
        ("ControlWord", ctypes.c_uint16),
        ("StatusWord", ctypes.c_uint16),
        ("TagWord", ctypes.c_uint8),
        ("Reserved1", ctypes.c_uint8),
        ("ErrorOpcode", ctypes.c_uint16),
        ("ErrorOffset", ctypes.c_uint32),
        ("ErrorSelector", ctypes.c_uint16),
        ("Reserved2", ctypes.c_uint16),
        ("DataOffset", ctypes.c_uint32),
        ("DataSelector", ctypes.c_uint16),
        ("Reserved3", ctypes.c_uint16),
        ("MxCsr", ctypes.c_uint32),
        ("MxCsr_Mask", ctypes.c_uint32),
        ("FloatRegisters", M128A * 8),
        ("XmmRegisters", M128A * 16),
        ("Reserved4", ctypes.c_uint8 * 96),
    ]


class CONTEXT(ctypes.Structure):
    _fields_ = [
        ("P1Home", ctypes.c_uint64),
        ("P2Home", ctypes.c_uint64),
        ("P3Home", ctypes.c_uint64),
        ("P4Home", ctypes.c_uint64),
        ("P5Home", ctypes.c_uint64),
        ("P6Home", ctypes.c_uint64),
        ("ContextFlags", ctypes.c_uint32),
        ("MxCsr", ctypes.c_uint32),
        ("SegCs", ctypes.c_uint16),
        ("SegDs", ctypes.c_uint16),
        ("SegEs", ctypes.c_uint16),
        ("SegFs", ctypes.c_uint16),
        ("SegGs", ctypes.c_uint16),
        ("SegSs", ctypes.c_uint16),
        ("EFlags", ctypes.c_uint32),
        ("Dr0", ctypes.c_uint64),
        ("Dr1", ctypes.c_uint64),
        ("Dr2", ctypes.c_uint64),
        ("Dr3", ctypes.c_uint64),
        ("Dr6", ctypes.c_uint64),
        ("Dr7", ctypes.c_uint64),
        ("Rax", ctypes.c_uint64),
        ("Rcx", ctypes.c_uint64),
        ("Rdx", ctypes.c_uint64),
        ("Rbx", ctypes.c_uint64),
        ("Rsp", ctypes.c_uint64),
        ("Rbp", ctypes.c_uint64),
        ("Rsi", ctypes.c_uint64),
        ("Rdi", ctypes.c_uint64),
        ("R8", ctypes.c_uint64),
        ("R9", ctypes.c_uint64),
        ("R10", ctypes.c_uint64),
        ("R11", ctypes.c_uint64),
        ("R12", ctypes.c_uint64),
        ("R13", ctypes.c_uint64),
        ("R14", ctypes.c_uint64),
        ("R15", ctypes.c_uint64),
        ("Rip", ctypes.c_uint64),
        ("FltSave", XSAVE_FORMAT),
        ("VectorRegister", M128A * 26),
        ("VectorControl", ctypes.c_uint64),
        ("DebugControl", ctypes.c_uint64),
        ("LastBranchToRip", ctypes.c_uint64),
        ("LastBranchFromRip", ctypes.c_uint64),
        ("LastExceptionToRip", ctypes.c_uint64),
        ("LastExceptionFromRip", ctypes.c_uint64),
    ]


class ADDRESS64(ctypes.Structure):
    _fields_ = [("Offset", ctypes.c_uint64), ("Segment", ctypes.c_uint16), ("Mode", ctypes.c_uint32)]


# Prefix of STACKFRAME64 up to the KDHELP64 member. The trailing buffer slack
# absorbs the real KDHELP64 field sizes which dbghelp writes back.
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


class _SYMBOL_INFO_BASEW(ctypes.Structure):
    """SYMBOL_INFOW with Name[1] — its size (88) is the required SizeOfStruct."""

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
    """SYMBOL_INFOW with a generous name buffer for SymFromAddrW output."""

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


class MODULEENTRY32W(ctypes.Structure):
    _fields_ = [
        ("dwSize", ctypes.c_uint32),
        ("th32ModuleID", ctypes.c_uint32),
        ("th32ProcessID", ctypes.c_uint32),
        ("GlblcntUsage", ctypes.c_uint32),
        ("ProccntUsage", ctypes.c_uint32),
        ("modBaseAddr", ctypes.POINTER(ctypes.c_uint8)),
        ("modBaseSize", ctypes.c_uint32),
        ("hModule", ctypes.c_void_p),
        ("szModule", ctypes.c_wchar * 256),
        ("szExePath", ctypes.c_wchar * 260),
    ]


class THREADENTRY32(ctypes.Structure):
    _fields_ = [
        ("dwSize", ctypes.c_uint32),
        ("cntUsage", ctypes.c_uint32),
        ("th32ThreadID", ctypes.c_uint32),
        ("th32OwnerProcessID", ctypes.c_uint32),
        ("tpBasePri", ctypes.c_long),
        ("tpDeltaPri", ctypes.c_long),
        ("dwFlags", ctypes.c_uint32),
    ]


class _WctLockObject(ctypes.Structure):
    _fields_ = [
        ("ObjectName", ctypes.c_wchar * WCT_OBJNAME_LENGTH),
        ("Timeout", ctypes.c_int64),
        ("Alertable", ctypes.c_uint32),
    ]


class _WctThreadObject(ctypes.Structure):
    _fields_ = [
        ("ProcessId", ctypes.c_uint32),
        ("ThreadId", ctypes.c_uint32),
        ("WaitTime", ctypes.c_uint32),
        ("ContextSwitches", ctypes.c_uint32),
    ]


class _WctUnion(ctypes.Union):
    _fields_ = [("LockObject", _WctLockObject), ("ThreadObject", _WctThreadObject)]


class WCT_NODE_INFO(ctypes.Structure):
    _anonymous_ = ["u"]
    _fields_ = [("ObjectType", ctypes.c_uint32), ("ObjectStatus", ctypes.c_uint32), ("u", _WctUnion)]


WCT_TYPE_NAMES = {
    0: "Unknown0",
    1: "CriticalSection",
    2: "SendMessage",
    3: "Mutex",
    4: "Alpc",
    5: "Com",
    6: "ThreadWait",
    7: "ProcessWait",
    8: "Thread",
    9: "ComActivation",
    10: "Unknown",
    11: "SocketIo",
    12: "SmbIo",
    13: "Max",
}

WCT_STATUS_NAMES = {
    0: "Status0",
    1: "NoAccess",
    2: "Running",
    3: "Blocked",
    4: "PidOnly",
    5: "PidOnlyRpcss",
    6: "Owned",
    7: "NotOwned",
    8: "Abandoned",
    9: "Unknown",
    10: "Error",
    11: "Max",
}

kernel32.OpenProcess.argtypes = [ctypes.c_uint32, ctypes.c_bool, ctypes.c_uint32]
kernel32.OpenProcess.restype = ctypes.c_void_p
kernel32.OpenThread.argtypes = [ctypes.c_uint32, ctypes.c_bool, ctypes.c_uint32]
kernel32.OpenThread.restype = ctypes.c_void_p
kernel32.GetThreadContext.argtypes = [ctypes.c_void_p, ctypes.c_void_p]
kernel32.GetThreadContext.restype = ctypes.c_bool
kernel32.SuspendThread.argtypes = [ctypes.c_void_p]
kernel32.SuspendThread.restype = ctypes.c_uint32
kernel32.ResumeThread.argtypes = [ctypes.c_void_p]
kernel32.ResumeThread.restype = ctypes.c_uint32
kernel32.CreateToolhelp32Snapshot.argtypes = [ctypes.c_uint32, ctypes.c_uint32]
kernel32.CreateToolhelp32Snapshot.restype = ctypes.c_void_p
kernel32.Module32FirstW.argtypes = [ctypes.c_void_p, ctypes.POINTER(MODULEENTRY32W)]
kernel32.Module32FirstW.restype = ctypes.c_bool
kernel32.Module32NextW.argtypes = [ctypes.c_void_p, ctypes.POINTER(MODULEENTRY32W)]
kernel32.Module32NextW.restype = ctypes.c_bool
kernel32.Thread32First.argtypes = [ctypes.c_void_p, ctypes.POINTER(THREADENTRY32)]
kernel32.Thread32First.restype = ctypes.c_bool
kernel32.Thread32Next.argtypes = [ctypes.c_void_p, ctypes.POINTER(THREADENTRY32)]
kernel32.Thread32Next.restype = ctypes.c_bool
kernel32.CloseHandle.argtypes = [ctypes.c_void_p]
kernel32.CloseHandle.restype = ctypes.c_bool
kernel32.ReadProcessMemory.argtypes = [
    ctypes.c_void_p,
    ctypes.c_void_p,
    ctypes.c_void_p,
    ctypes.c_size_t,
    ctypes.POINTER(ctypes.c_size_t),
]
kernel32.ReadProcessMemory.restype = ctypes.c_bool

dbghelp.SymSetOptions.argtypes = [ctypes.c_uint32]
dbghelp.SymSetOptions.restype = ctypes.c_uint32
dbghelp.SymInitializeW.argtypes = [ctypes.c_void_p, ctypes.c_wchar_p, ctypes.c_bool]
dbghelp.SymInitializeW.restype = ctypes.c_bool
dbghelp.SymCleanup.argtypes = [ctypes.c_void_p]
dbghelp.SymCleanup.restype = ctypes.c_bool
dbghelp.SymLoadModuleExW.argtypes = [
    ctypes.c_void_p,
    ctypes.c_void_p,
    ctypes.c_wchar_p,
    ctypes.c_wchar_p,
    ctypes.c_uint64,
    ctypes.c_uint32,
    ctypes.c_void_p,
    ctypes.c_uint32,
]
dbghelp.SymLoadModuleExW.restype = ctypes.c_uint64
dbghelp.SymFromAddrW.argtypes = [
    ctypes.c_void_p,
    ctypes.c_uint64,
    ctypes.POINTER(ctypes.c_uint64),
    ctypes.POINTER(SYMBOL_INFOW),
]
dbghelp.SymFromAddrW.restype = ctypes.c_bool
dbghelp.StackWalk64.argtypes = [
    ctypes.c_uint32,
    ctypes.c_void_p,
    ctypes.c_void_p,
    ctypes.c_void_p,  # LPSTACKFRAME64 (slack buffer)
    ctypes.c_void_p,  # ContextRecord
    ctypes.c_void_p,  # ReadMemoryRoutine (NULL -> ReadProcessMemory on hProcess)
    ctypes.c_void_p,  # FunctionTableAccessRoutine
    ctypes.c_void_p,  # GetModuleBaseRoutine
    ctypes.c_void_p,  # TranslateAddress
]
dbghelp.StackWalk64.restype = ctypes.c_bool
dbghelp.SymFunctionTableAccess64.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
dbghelp.SymFunctionTableAccess64.restype = ctypes.c_void_p
dbghelp.SymGetModuleBase64.argtypes = [ctypes.c_void_p, ctypes.c_uint64]
dbghelp.SymGetModuleBase64.restype = ctypes.c_uint64
dbghelp.MiniDumpWriteDump.argtypes = [
    ctypes.c_void_p,
    ctypes.c_uint32,
    ctypes.c_void_p,
    ctypes.c_uint32,
    ctypes.c_void_p,
    ctypes.c_void_p,
    ctypes.c_void_p,
]
dbghelp.MiniDumpWriteDump.restype = ctypes.c_bool

advapi32.OpenThreadWaitChainSession.argtypes = [ctypes.c_uint32, ctypes.c_void_p]
advapi32.OpenThreadWaitChainSession.restype = ctypes.c_void_p
advapi32.CloseThreadWaitChainSession.argtypes = [ctypes.c_void_p]
advapi32.CloseThreadWaitChainSession.restype = None
advapi32.GetThreadWaitChain.argtypes = [
    ctypes.c_void_p,
    ctypes.c_void_p,  # Context
    ctypes.c_uint32,  # Flags
    ctypes.c_uint32,  # ThreadId
    ctypes.POINTER(ctypes.c_uint32),  # NodeCount
    ctypes.POINTER(WCT_NODE_INFO),  # NodeInfoArray
    ctypes.POINTER(ctypes.c_bool),  # IsCycle
]
advapi32.GetThreadWaitChain.restype = ctypes.c_bool

ntdll.NtQueryInformationThread.argtypes = [
    ctypes.c_void_p,
    ctypes.c_uint32,
    ctypes.c_void_p,
    ctypes.c_uint32,
    ctypes.POINTER(ctypes.c_uint32),
]
ntdll.NtQueryInformationThread.restype = ctypes.c_uint32


# ---------------------------------------------------------------------------
# Process / module enumeration
# ---------------------------------------------------------------------------


def open_process(pid):
    h = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, False, pid)
    if not h:
        raise OSError(f"OpenProcess({pid}) failed: winerr={ctypes.get_last_error()}")
    return h


def list_modules(hproc, pid):
    snap = kernel32.CreateToolhelp32Snapshot(TH32CS_SNAPMODULE | TH32CS_SNAPMODULE32, pid)
    modules = []
    if snap in (None, INVALID_HANDLE_VALUE):
        return modules
    entry = MODULEENTRY32W()
    entry.dwSize = ctypes.sizeof(MODULEENTRY32W)
    ok = kernel32.Module32FirstW(snap, ctypes.byref(entry))
    while ok:
        modules.append(
            {
                "name": entry.szModule,
                "path": entry.szExePath,
                "base": ctypes.cast(entry.modBaseAddr, ctypes.c_void_p).value or 0,
                "size": int(entry.modBaseSize),
            }
        )
        ok = kernel32.Module32NextW(snap, ctypes.byref(entry))
    kernel32.CloseHandle(snap)
    return modules


def list_threads(pid):
    snap = kernel32.CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0)
    threads = []
    if snap in (None, INVALID_HANDLE_VALUE):
        return threads
    entry = THREADENTRY32()
    entry.dwSize = ctypes.sizeof(THREADENTRY32)
    ok = kernel32.Thread32First(snap, ctypes.byref(entry))
    while ok:
        if entry.th32OwnerProcessID == pid:
            threads.append(int(entry.th32ThreadID))
        ok = kernel32.Thread32Next(snap, ctypes.byref(entry))
    kernel32.CloseHandle(snap)
    return threads


# ---------------------------------------------------------------------------
# PDB retrieval (System32 dbghelp has no symsrv client)
# ---------------------------------------------------------------------------


def get_pdb_guid(pe_path):
    """Extract (pdb_name, guid_hex, age) from a PE's CodeView debug directory."""
    import struct

    with open(pe_path, "rb") as fh:
        data = fh.read()
    e_lfanew = struct.unpack_from("<I", data, 0x3C)[0]
    if data[e_lfanew : e_lfanew + 4] != b"PE\x00\x00":
        return None
    coff = e_lfanew + 4
    num_sections = struct.unpack_from("<H", data, coff + 2)[0]
    opt_size = struct.unpack_from("<H", data, coff + 16)[0]
    opt = coff + 20
    magic = struct.unpack_from("<H", data, opt)[0]
    dd_off = opt + (112 if magic == 0x20B else 96)
    debug_rva, debug_size = struct.unpack_from("<II", data, dd_off + 6 * 8)
    sec_off = opt + opt_size
    sections = []
    for i in range(num_sections):
        off = sec_off + i * 40
        vsize, vaddr, rsize, raddr = struct.unpack_from("<IIII", data, off + 8)
        sections.append((vaddr, vsize, raddr, rsize))

    def rva_to_off(rva):
        for vaddr, vsize, raddr, rsize in sections:
            if vaddr <= rva < vaddr + max(vsize, rsize):
                return raddr + (rva - vaddr)
        return None

    d_off = rva_to_off(debug_rva)
    if d_off is None:
        return None
    for i in range(debug_size // 28):
        entry = d_off + i * 28
        typ = struct.unpack_from("<I", data, entry + 12)[0]
        size_data, _rva_raw, ptr_raw = struct.unpack_from("<III", data, entry + 16)
        if typ != 2:  # IMAGE_DEBUG_TYPE_CODEVIEW
            continue
        cv = data[ptr_raw : ptr_raw + size_data]
        if cv[:4] != b"RSDS":
            continue
        d1, d2, d3 = struct.unpack_from("<IHH", cv, 4)
        d4 = cv[12:20]
        age = struct.unpack_from("<I", cv, 20)[0]
        name = cv[24:].split(b"\x00")[0].decode(errors="replace")
        guid_hex = f"{d1:08X}{d2:04X}{d3:04X}" + d4.hex().upper()
        return name, guid_hex, age
    return None


def ensure_pdb(pe_path, cache_dir):
    """Download the PDB for a module from msdl if not already cached."""
    import urllib.request

    try:
        info = get_pdb_guid(pe_path)
        if not info:
            return False, "no CodeView entry"
        name, guid_hex, age = info
        dest = os.path.join(cache_dir, name)
        if os.path.exists(dest) and os.path.getsize(dest) > 0:
            return True, "cached"
        for age_text in (f"{age:X}", str(age)):
            url = f"https://msdl.microsoft.com/download/symbols/{name}/{guid_hex}{age_text}/{name}"
            try:
                req = urllib.request.Request(url, headers={"User-Agent": "vit-wedge-probe"})
                with urllib.request.urlopen(req, timeout=90) as resp, open(dest + ".tmp", "wb") as out:
                    out.write(resp.read())
                os.replace(dest + ".tmp", dest)
                return True, f"downloaded ({age_text})"
            except Exception as exc:  # noqa: BLE001 - try the next age format
                last_err = f"{url}: {exc!r}"
                if os.path.exists(dest + ".tmp"):
                    os.remove(dest + ".tmp")
        return False, last_err
    except Exception as exc:  # noqa: BLE001
        return False, f"{exc!r}"


# ---------------------------------------------------------------------------
# Stack capture
# ---------------------------------------------------------------------------


class SymbolSession:
    """dbghelp symbol session bound to a target process handle.

    System32 dbghelp has no symbol-server client (symsrv), so PDBs for the
    interesting system modules are fetched directly from msdl into a flat
    cache directory and handed to dbghelp as a plain search path.
    """

    PDB_WHITELIST = (
        "ntdll.dll", "kernelbase.dll", "kernel32.dll", "ws2_32.dll", "user32.dll",
        "ucrtbase.dll", "msvcrt.dll", "combase.dll", "rpcrt4.dll", "ole32.dll",
    )

    def __init__(self, hproc, modules, sym_cache_dir):
        self.hproc = hproc
        self.ok = False
        self.error = None
        self.pdb_status = {}
        os.makedirs(sym_cache_dir, exist_ok=True)
        for m in modules:
            name = m["name"].lower()
            if name in self.PDB_WHITELIST:
                ok, info = ensure_pdb(m["path"], sym_cache_dir)
                self.pdb_status[m["name"]] = info
        dbghelp.SymSetOptions(SYMOPT_UNDNAME | SYMOPT_DEFERRED_LOADS | SYMOPT_LOAD_LINES)
        if not dbghelp.SymInitializeW(hproc, sym_cache_dir, False):
            self.error = f"SymInitializeW failed: winerr={ctypes.get_last_error()}"
            return
        for m in modules:
            dbghelp.SymLoadModuleExW(hproc, None, m["path"], None, m["base"], m["size"], None, 0)
        self.ok = True

    def symbolize(self, addr):
        sym = SYMBOL_INFOW()
        sym.SizeOfStruct = _SYMBOL_INFO_BASE_SIZE
        sym.MaxNameLen = MAX_SYM_NAME
        disp = ctypes.c_uint64(0)
        if dbghelp.SymFromAddrW(self.hproc, addr, ctypes.byref(disp), ctypes.byref(sym)):
            name = sym.Name[: sym.NameLen] if sym.NameLen else sym.Name
            return (name or None), int(disp.value)
        return None, None

    def cleanup(self):
        dbghelp.SymCleanup(self.hproc)


def walk_one_stack(hproc, hthread, session, modules, max_frames=160):
    """Suspend one thread, capture context, StackWalk64, resume."""
    ctx = CONTEXT()
    ctx.ContextFlags = CONTEXT_FULL_FLAGS
    ctx_size = ctypes.sizeof(CONTEXT)
    ctx_buf = ctypes.create_string_buffer(ctx_size + 16)
    ctx_addr = (ctypes.addressof(ctx_buf) + 15) // 16 * 16
    ctypes.memmove(ctx_addr, ctypes.addressof(ctx), ctx_size)

    suspend_count = kernel32.SuspendThread(hthread)
    suspend_ok = suspend_count != 0xFFFFFFFF
    got_ctx = kernel32.GetThreadContext(hthread, ctypes.c_void_p(ctx_addr))
    if not got_ctx:
        if suspend_ok:
            kernel32.ResumeThread(hthread)
        return {"error": f"GetThreadContext failed: winerr={ctypes.get_last_error()}", "frames": []}
    ctypes.memmove(ctypes.addressof(ctx), ctx_addr, ctx_size)
    try:
        # Start address (best effort)
        start_addr = 0
        out_start = ctypes.c_uint64(0)
        ret = ntdll.NtQueryInformationThread(hthread, 9, ctypes.byref(out_start), 8, None)
        if ret == 0:
            start_addr = out_start.value

        # STACKFRAME64 with slack buffer
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
                hproc,
                hthread,
                ctypes.c_void_p(ctypes.addressof(frame_buf)),
                ctypes.c_void_p(ctx_addr),
                None,
                ctypes.cast(dbghelp.SymFunctionTableAccess64, ctypes.c_void_p),
                ctypes.cast(dbghelp.SymGetModuleBase64, ctypes.c_void_p),
                None,
            )
            if not ok or frame.AddrPC.Offset == 0:
                break
            pc = frame.AddrPC.Offset
            name, disp = session.symbolize(pc) if session and session.ok else (None, None)
            if name:
                frames.append(f"{name}+0x{disp:x}" if disp else name)
            else:
                mod_name, mod_base = _module_for_addr(modules, pc)
                if mod_name:
                    frames.append(f"{mod_name}+0x{pc - mod_base:x}")
                else:
                    frames.append(f"0x{pc:x}")
    finally:
        if suspend_ok:
            kernel32.ResumeThread(hthread)
    return {
        "rip": f"0x{ctx.Rip:x}",
        "rsp": f"0x{ctx.Rsp:x}",
        "start_addr": f"0x{start_addr:x}",
        "suspend_ok": suspend_ok,
        "frames": frames,
    }


def _module_for_addr(modules, addr):
    for m in modules:
        if m["base"] <= addr < m["base"] + m["size"]:
            return m["name"], m["base"]
    return None, None


def capture_stacks(pid, out_path, sym_cache_dir, note=""):
    """Capture all thread stacks of a live process into out_path (JSON)."""
    started = _wall()
    hproc = open_process(pid)
    result = {
        "pid": pid,
        "captured_at_wall": started,
        "captured_at_monotonic": _now(),
        "note": note,
        "tool": "ctypes dbghelp StackWalk64",
        "threads": [],
        "errors": [],
    }
    try:
        modules = list_modules(hproc, pid)
        result["module_count"] = len(modules)
        result["modules"] = [
            {"name": m["name"], "base": f"0x{m['base']:x}", "size": m["size"]} for m in modules
        ]
        session = SymbolSession(hproc, modules, sym_cache_dir)
        if not session.ok:
            result["errors"].append(session.error)
        for tid in list_threads(pid):
            hthread = kernel32.OpenThread(
                THREAD_SUSPEND_RESUME | THREAD_GET_CONTEXT | THREAD_QUERY_INFORMATION, False, tid
            )
            if not hthread:
                result["errors"].append(f"thread {tid}: OpenThread failed winerr={ctypes.get_last_error()}")
                continue
            try:
                stack = walk_one_stack(hproc, hthread, session, modules)
                stack["tid"] = tid
                result["threads"].append(stack)
            finally:
                kernel32.CloseHandle(hthread)
        if session.ok:
            session.cleanup()
    except Exception as exc:  # noqa: BLE001 - diagnostics must record, not crash
        result["errors"].append(f"exception: {exc!r}")
    finally:
        kernel32.CloseHandle(hproc)
    with open(out_path, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=1, ensure_ascii=False)
    return result


# ---------------------------------------------------------------------------
# Wait chain traversal
# ---------------------------------------------------------------------------


def capture_wct(pid, out_path, note=""):
    result = {
        "pid": pid,
        "captured_at_wall": _wall(),
        "note": note,
        "tool": "Wait Chain Traversal (advapi32)",
        "flags_used": WCTP_GETINFO_ALL_FLAGS,
        "threads": [],
        "errors": [],
    }
    try:
        session = advapi32.OpenThreadWaitChainSession(0, None)
        if not session:
            result["errors"].append(f"OpenThreadWaitChainSession failed winerr={ctypes.get_last_error()}")
        else:
            for tid in list_threads(pid):
                node_count = ctypes.c_uint32(WCT_MAX_NODE_COUNT)
                nodes = (WCT_NODE_INFO * WCT_MAX_NODE_COUNT)()
                is_cycle = ctypes.c_bool(False)
                ok = advapi32.GetThreadWaitChain(
                    session,
                    None,
                    WCTP_GETINFO_ALL_FLAGS,
                    tid,
                    ctypes.byref(node_count),
                    nodes,
                    ctypes.byref(is_cycle),
                )
                if not ok:
                    result["errors"].append(f"thread {tid}: GetThreadWaitChain winerr={ctypes.get_last_error()}")
                    continue
                chain = []
                for i in range(min(node_count.value, WCT_MAX_NODE_COUNT)):
                    node = nodes[i]
                    entry = {
                        "type": WCT_TYPE_NAMES.get(node.ObjectType, str(node.ObjectType)),
                        "status": WCT_STATUS_NAMES.get(node.ObjectStatus, str(node.ObjectStatus)),
                    }
                    if node.ObjectType == 8:  # WctThreadType
                        entry["process_id"] = node.ThreadObject.ProcessId
                        entry["thread_id"] = node.ThreadObject.ThreadId
                        entry["wait_time_ms"] = node.ThreadObject.WaitTime
                        entry["context_switches"] = node.ThreadObject.ContextSwitches
                    elif 1 <= node.ObjectType <= 12:
                        raw = bytes(node.LockObject)[: WCT_OBJNAME_LENGTH * 2]
                        name = raw.decode("utf-16-le", errors="ignore").split("\x00")[0]
                        if name:
                            entry["object_name"] = name[:64]
                    chain.append(entry)
                result["threads"].append({"tid": tid, "is_cycle": bool(is_cycle.value), "chain": chain})
            advapi32.CloseThreadWaitChainSession(session)
    except Exception as exc:  # noqa: BLE001
        result["errors"].append(f"exception: {exc!r}")
    with open(out_path, "w", encoding="utf-8") as fh:
        json.dump(result, fh, indent=1, ensure_ascii=False)
    return result


# ---------------------------------------------------------------------------
# Minidump
# ---------------------------------------------------------------------------


def write_minidump(pid, out_path, note=""):
    result = {"pid": pid, "path": out_path, "captured_at_wall": _wall(), "note": note, "ok": False}
    hproc = None
    hfile = None
    try:
        hproc = open_process(pid)
        hfile = kernel32.CreateFileW(
            out_path, GENERIC_WRITE, 0, None, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, None
        )
        if hfile in (None, INVALID_HANDLE_VALUE):
            raise OSError(f"CreateFileW failed winerr={ctypes.get_last_error()}")
        ok = dbghelp.MiniDumpWriteDump(hproc, pid, ctypes.c_void_p(hfile), MINIDUMP_TYPE, None, None, None)
        if not ok:
            raise OSError(f"MiniDumpWriteDump failed winerr={ctypes.get_last_error()}")
        result["ok"] = True
        result["size_bytes"] = os.path.getsize(out_path)
    except Exception as exc:  # noqa: BLE001
        result["error"] = f"{exc!r}"
    finally:
        if hfile not in (None, INVALID_HANDLE_VALUE):
            kernel32.CloseHandle(ctypes.c_void_p(hfile))
        if hproc:
            kernel32.CloseHandle(hproc)
    return result


# ---------------------------------------------------------------------------
# ZMQ monitor
# ---------------------------------------------------------------------------


class EventLog:
    def __init__(self, out_dir):
        self.path = os.path.join(out_dir, "monitor_events.jsonl")
        self.fh = open(self.path, "a", encoding="utf-8")
        self.t0 = _now()

    def log(self, kind, **fields):
        rec = {"monotonic": round(_now() - self.t0, 3), "wall": _wall(), "kind": kind}
        rec.update(fields)
        self.fh.write(json.dumps(rec, ensure_ascii=False) + "\n")
        self.fh.flush()
        print(f"[{rec['monotonic']:8.2f}s] {kind}: " + " ".join(f"{k}={v}" for k, v in fields.items()))
        return rec

    def close(self):
        self.fh.close()


def classify_pub_event(payload):
    try:
        data = json.loads(payload)
    except (json.JSONDecodeError, UnicodeDecodeError):
        return {"class": "unparsed", "raw": payload[:200]}
    cls = "other"
    if data.get("command") == "l2_render_probe_status" or data.get("command") == "l2_render_probe_ready":
        st = data.get("status")
        if st == "building":
            cls = "probe_building"
        elif st in ("ready", "ok"):
            cls = "probe_success"
        elif st in ("suspect", "error", "failed"):
            cls = "probe_failure"
    elif data.get("topic") == "render":
        sub = data.get("subtopic")
        if sub == "render_progress":
            cls = "render_progress"
        elif sub == "render_done":
            cls = "render_done"
        elif sub == "render_failed":
            cls = "watchdog_failure" if data.get("source") == "render_watchdog" else "render_failed"
    elif data.get("event") == "command_received":
        cls = "command_received_echo"
    data["_class"] = cls
    return data


def render_probe_files(temp_root, since_wall):
    """List l2_probe wav files with mtime at/after the request wall time."""
    found = []
    if not os.path.isdir(temp_root):
        return found
    for name in os.listdir(temp_root):
        if not name.startswith("l2_probe_") or not name.endswith(".wav"):
            continue
        path = os.path.join(temp_root, name)
        try:
            st = os.stat(path)
        except OSError:
            continue
        if st.st_mtime + 2.0 < since_wall:
            continue  # leftover from an earlier run
        found.append({"path": path, "size": st.st_size, "mtime": st.st_mtime})
    return found


def run_monitor(args):
    import zmq

    out_dir = os.path.abspath(args.out_dir)
    os.makedirs(out_dir, exist_ok=True)
    log = EventLog(out_dir)
    group_result = {
        "group": args.group,
        "request_file": os.path.abspath(args.request_file),
        "project_copy": os.path.abspath(args.project_copy),
        "kernel_pid": args.kernel_pid,
        "verdict": None,
        "requests": [],
        "pub_broken": False,
        "watchdog_expected_at_s": args.watchdog_seconds,
        "watchdog_triggered": False,
        "watchdog_event": None,
        "a_stacks": [],
        "b_stacks": [],
        "a_wct": None,
        "b_wct": None,
        "a_dump": None,
        "b_dump": None,
        "missing_evidence": [],
        "errors": [],
    }

    with open(args.request_file, "r", encoding="utf-8-sig") as fh:
        request_payload = json.load(fh)

    ctx = zmq.Context()
    sub = ctx.socket(zmq.SUB)
    sub.setsockopt_string(zmq.SUBSCRIBE, "")
    sub.setsockopt(zmq.RCVHWM, 4096)
    sub.connect(args.pub_endpoint)
    sub_received_any = False

    req = ctx.socket(zmq.REQ)
    req.setsockopt(zmq.RCVTIMEO, 30000)
    req.setsockopt(zmq.LINGER, 0)
    req.connect(args.req_endpoint)

    ping = ctx.socket(zmq.REQ)
    ping.setsockopt(zmq.RCVTIMEO, int(args.ping_timeout * 1000))
    ping.setsockopt(zmq.LINGER, 0)
    ping.connect(args.req_endpoint)

    try:
        # Phase 0: PUB subscription ready
        time.sleep(0.7)
        log.log("pub_subscription_ready", endpoint=args.pub_endpoint)
        group_result["pub_subscription_ready_s"] = round(_now() - log.t0, 3)

        # Phase 1: open project copy
        t_open = _now()
        req.send_string(json.dumps({"cmd": "open_project", "file_path": os.path.abspath(args.project_copy)}))
        try:
            reply = json.loads(req.recv_string())
        except zmq.Again:
            group_result["verdict"] = "protocol_error"
            group_result["errors"].append("open_project reply timed out")
            log.log("open_project_timeout")
            return finish(group_result, log, 2)
        log.log("open_project_reply", status=reply.get("status"), message=reply.get("message"),
                project_path=reply.get("project_path") or reply.get("current_project_path"))
        group_result["open_project_reply"] = reply
        if reply.get("status") != "ok":
            group_result["verdict"] = "protocol_error"
            group_result["errors"].append(f"open_project failed: {reply.get('message')}")
            return finish(group_result, log, 2)

        # Phase 2: send probe request
        request_sent_mono = _now()
        req.send_string(json.dumps(request_payload))
        try:
            reply = json.loads(req.recv_string())
        except zmq.Again:
            group_result["verdict"] = "protocol_error"
            group_result["errors"].append("probe request reply timed out")
            return finish(group_result, log, 2)
        group_result["requests"].append({"request": request_payload, "startup_reply": reply})
        job_id = reply.get("job_id")
        log.log("request_sent", job_id=job_id, status=reply.get("status"),
                message=reply.get("message"), probe_status=reply.get("probe_status"))
        if reply.get("status") != "ok" or not job_id:
            # error reply = request rejected; observable anomaly but not a render attempt
            group_result["verdict"] = "request_rejected"
            return finish(group_result, log, 1)

        temp_root = os.path.join(tempfile.gettempdir(), "Vit_DAW_L2RenderProbe")
        request_sent_wall = time.time()
        watchdog_at = request_sent_mono + args.watchdog_seconds
        a_stack_at = request_sent_mono + args.a_stack_at

        trial = 0
        max_trials = args.second_request_trials + 1  # first + optional confirmation renders
        current_job_sent = request_sent_mono
        current_job_id = job_id
        current_watchdog_at = watchdog_at
        current_a_stack_at = a_stack_at
        current_a_captured = False

        last_ping_mono = _now()
        deadline = request_sent_mono + args.max_seconds
        terminal_seen_for_current = False
        terminal_kind_current = None
        watchdog_event_seen = False

        while _now() < deadline:
            now = _now()
            # drain PUB
            while True:
                try:
                    raw = sub.recv_string(flags=zmq.NOBLOCK)
                except zmq.Again:
                    break
                if not sub_received_any:
                    sub_received_any = True
                    log.log("pub_first_message")
                ev = classify_pub_event(raw)
                ev_job = ev.get("job_id")
                log.log("pub_event", cls=ev.get("_class"), job_id=ev_job, payload=raw[:400])
                if ev_job != current_job_id:
                    continue
                cls = ev.get("_class")
                if cls in ("probe_success", "render_done"):
                    terminal_seen_for_current = True
                    terminal_kind_current = cls
                elif cls == "watchdog_failure":
                    group_result["watchdog_triggered"] = True
                    group_result["watchdog_event"] = ev
                    # Watchdog = the A->cancel transition, not a terminal verdict.
                    # Keep observing pings to discriminate whether B follows.
                    watchdog_event_seen = True
                    if args.stop_at_watchdog:
                        # WEDGE-2B1 sweep mode: the watchdog firing IS the trial
                        # outcome (wedge confirmed); collect and stop. No B-phase
                        # forensics (A->B already pinned by WEDGE-2A).
                        group_result["watchdog_request_index"] = len(group_result["requests"]) - 1
                        group_result["verdict"] = ("watchdog_wedge_second_request" if trial > 0
                                                   else "watchdog_wedge")
                        sizes = render_probe_files(temp_root, request_sent_wall)
                        group_result["final_render_files"] = [
                            {"name": os.path.basename(f["path"]), "size": f["size"]} for f in sizes]
                        log.log("stop_at_watchdog", job_id=current_job_id,
                                verdict=group_result["verdict"],
                                watched_until_s=round(_now() - current_job_sent, 2),
                                final_render_files=group_result["final_render_files"])
                        return finish(group_result, log, None, sweep=True)
                elif cls in ("probe_failure", "render_failed"):
                    terminal_seen_for_current = True
                    terminal_kind_current = cls

            # periodic ping
            if now - last_ping_mono >= args.ping_interval:
                last_ping_mono = now
                t_ping = _now()
                pong = None
                try:
                    ping.send_string(json.dumps({"cmd": "ping"}))
                    pong = json.loads(ping.recv_string())
                    ok = pong.get("status") == "ok"
                except zmq.Again:
                    ok = False
                rtt = round(_now() - t_ping, 3)
                sizes = render_probe_files(temp_root, request_sent_wall)
                log.log("ping", ok=ok, rtt_s=rtt, reply=pong,
                        render_files=[{"name": os.path.basename(f["path"]), "size": f["size"]} for f in sizes])
                if not ok:
                    # Message thread no longer serving commands: either the ZMQ
                    # recv timed out or the gateway returned its 5s "Timed out
                    # waiting for JUCE command handling" error reply. Both are
                    # the B-phase signature -> capture stacks immediately.
                    log.log("b_phase_ping_dead", job_id=current_job_id, reply=pong)
                    if args.stop_at_watchdog:
                        # Sweep mode: no B-phase forensics needed (A->B already
                        # pinned); the ping death itself is the record.
                        group_result["verdict"] = "ping_dead_no_forensics"
                        group_result["final_ping_dead_after_s"] = round(_now() - current_job_sent, 3)
                        group_result["ping_dead_form"] = ("zmq_timeout" if pong is None else "error_reply")
                        sizes = render_probe_files(temp_root, request_sent_wall)
                        group_result["final_render_files"] = [
                            {"name": os.path.basename(f["path"]), "size": f["size"]} for f in sizes]
                        log.log("stop_ping_dead", job_id=current_job_id,
                                final_render_files=group_result["final_render_files"])
                        return finish(group_result, log, None, sweep=True)
                    b1 = os.path.join(out_dir, "b_stacks_1.json")
                    r1 = capture_stacks(args.kernel_pid, b1, args.sym_cache,
                                        note="B phase, immediately after ping failure")
                    group_result["b_stacks"].append(b1)
                    log.log("b_stack_captured", path=b1, threads=len(r1.get("threads", [])), errors=r1.get("errors"))
                    b1wct = os.path.join(out_dir, "b_wct_1.json")
                    capture_wct(args.kernel_pid, b1wct, note="B phase wait chains, first sample")
                    group_result["b_wct"] = [b1wct]
                    time.sleep(args.b_stack_gap)
                    b2 = os.path.join(out_dir, "b_stacks_2.json")
                    r2 = capture_stacks(args.kernel_pid, b2, args.sym_cache,
                                        note="B phase, second sample")
                    group_result["b_stacks"].append(b2)
                    log.log("b_stack_captured", path=b2, threads=len(r2.get("threads", [])), errors=r2.get("errors"))
                    b2wct = os.path.join(out_dir, "b_wct_2.json")
                    capture_wct(args.kernel_pid, b2wct, note="B phase wait chains, second sample")
                    group_result["b_wct"].append(b2wct)
                    # confirmation ping (scene is preserved; process not yet stopped)
                    t_ping2 = _now()
                    try:
                        ping.send_string(json.dumps({"cmd": "ping"}))
                        pong2 = json.loads(ping.recv_string())
                        log.log("b_confirmation_ping", ok=pong2.get("status") == "ok",
                                rtt_s=round(_now() - t_ping2, 3), reply=pong2)
                    except zmq.Again:
                        log.log("b_confirmation_ping", ok=False, rtt_s=round(_now() - t_ping2, 3),
                                reply=None, note="zmq timeout")
                    bdump = os.path.join(out_dir, "b_minidump.dmp")
                    dres = write_minidump(args.kernel_pid, bdump, note="B phase minidump")
                    group_result["b_dump"] = bdump if dres.get("ok") else None
                    if not dres.get("ok"):
                        group_result["missing_evidence"].append(f"b_minidump: {dres.get('error')}")
                    log.log("b_dump", ok=dres.get("ok"), size=dres.get("size_bytes"), error=dres.get("error"))
                    if not group_result["b_stacks"]:
                        group_result["missing_evidence"].append("b_stacks unavailable")
                    group_result["verdict"] = "message_thread_dead_B"
                    group_result["final_ping_dead_after_s"] = round(_now() - current_job_sent, 3)
                    group_result["ping_dead_form"] = ("zmq_timeout" if pong is None else "error_reply")
                    return finish(group_result, log, 1)

            # A-phase stack capture before watchdog deadline
            if (not current_a_captured and not terminal_seen_for_current
                    and current_a_stack_at and now >= current_a_stack_at):
                current_a_captured = True
                log.log("a_phase_capture_start", job_id=current_job_id,
                        watchdog_in_s=round(current_watchdog_at - _now(), 2))
                a1 = os.path.join(out_dir, f"a_stacks_trial{trial}_1.json")
                r1 = capture_stacks(args.kernel_pid, a1, args.sym_cache,
                                    note="A phase, first sample (before watchdog deadline)")
                group_result["a_stacks"].append(a1)
                log.log("a_stack_captured", path=a1, threads=len(r1.get("threads", [])), errors=r1.get("errors"))
                a1wct = os.path.join(out_dir, f"a_wct_trial{trial}_1.json")
                capture_wct(args.kernel_pid, a1wct, note="A phase wait chains, first sample")
                group_result["a_wct"] = [a1wct]
                time.sleep(args.a_stack_gap)
                a2 = os.path.join(out_dir, f"a_stacks_trial{trial}_2.json")
                r2 = capture_stacks(args.kernel_pid, a2, args.sym_cache,
                                    note="A phase, second sample")
                group_result["a_stacks"].append(a2)
                log.log("a_stack_captured", path=a2, threads=len(r2.get("threads", [])), errors=r2.get("errors"))
                a2wct = os.path.join(out_dir, f"a_wct_trial{trial}_2.json")
                capture_wct(args.kernel_pid, a2wct, note="A phase wait chains, second sample")
                group_result["a_wct"].append(a2wct)
                adump = os.path.join(out_dir, f"a_minidump_trial{trial}.dmp")
                dres = write_minidump(args.kernel_pid, adump, note="A phase minidump (pre-watchdog scene)")
                group_result["a_dump"] = adump if dres.get("ok") else None
                if not dres.get("ok"):
                    group_result["missing_evidence"].append(f"a_minidump: {dres.get('error')}")
                log.log("a_dump", ok=dres.get("ok"), size=dres.get("size_bytes"), error=dres.get("error"))

            # watchdog window passed without terminal?
            if not terminal_seen_for_current and now >= current_watchdog_at + args.observe_after_watchdog:
                if not watchdog_event_seen and not group_result["watchdog_triggered"]:
                    group_result["missing_evidence"].append(
                        "no watchdog failure event observed on PUB within observation window"
                    )
                if current_a_captured:
                    group_result["verdict"] = (
                        "watchdog_fired_ping_alive" if watchdog_event_seen else "no_progress_A_captured"
                    )
                else:
                    group_result["verdict"] = "no_progress_A_not_captured"
                    group_result["missing_evidence"].append(
                        "A phase stacks were not captured before watchdog deadline"
                    )
                log.log("no_progress_confirmed", job_id=current_job_id,
                        watched_until_s=round(now - current_job_sent, 2),
                        watchdog_event_seen=watchdog_event_seen)
                break

            # terminal for the current job?
            if terminal_seen_for_current:
                log.log("terminal_seen", job_id=current_job_id, terminal=terminal_kind_current)
                group_result.setdefault("terminals", []).append(
                    {"job_id": current_job_id, "terminal": terminal_kind_current}
                )
                if terminal_kind_current in ("probe_failure", "render_failed"):
                    group_result["verdict"] = "render_error_terminal"
                    break
                # success terminal: per card step 7, request the same params again
                trial += 1
                if trial >= max_trials:
                    group_result["verdict"] = "healthy"
                    break
                log.log("second_same_param_request", trial=trial)
                current_job_sent = _now()
                request_sent_wall = time.time()
                req.send_string(json.dumps(request_payload))
                try:
                    reply2 = json.loads(req.recv_string())
                except zmq.Again:
                    group_result["verdict"] = "protocol_error"
                    group_result["errors"].append("second probe request reply timed out")
                    return finish(group_result, log, 2)
                group_result["requests"].append({"request": request_payload, "startup_reply": reply2})
                current_job_id = reply2.get("job_id")
                current_watchdog_at = current_job_sent + args.watchdog_seconds
                current_a_stack_at = current_job_sent + args.a_stack_at
                current_a_captured = False
                terminal_seen_for_current = False
                terminal_kind_current = None
                watchdog_event_seen = False
                if reply2.get("status") != "ok" or not current_job_id:
                    group_result["verdict"] = "second_request_rejected"
                    group_result["errors"].append(f"second request rejected: {reply2.get('message')}")
                    break
                log.log("second_request_started", job_id=current_job_id)
                last_ping_mono = _now()
                continue

            time.sleep(0.15)

        if group_result["verdict"] is None:
            group_result["verdict"] = "timeout_inconclusive"
            group_result["missing_evidence"].append("monitor hit its own max-seconds deadline without a verdict")
    finally:
        if not sub_received_any:
            group_result["pub_broken"] = True
            group_result["missing_evidence"].append(
                "PUB delivered no messages at all (not even command_received echoes); "
                "completion/failure events could not be observed via PUB"
            )
        sub.close(0)
        req.close(0)
        ping.close(0)
        ctx.term()
    return finish(group_result, log, None, sweep=args.stop_at_watchdog)


def finish(group_result, log, forced_code, sweep=False):
    log.log("group_result", verdict=group_result["verdict"], errors=group_result["errors"])
    out = os.path.normpath(os.path.join(os.path.dirname(log.path), "group_result.json"))
    with open(out, "w", encoding="utf-8") as fh:
        json.dump(group_result, fh, indent=1, ensure_ascii=False)
    log.close()
    if forced_code is not None:
        return forced_code
    verdict = group_result["verdict"] or "timeout_inconclusive"
    if sweep:
        # WEDGE-2B1 sweep semantics: a wedge (watchdog) is valid discriminating
        # data, not a tooling failure. 0 = trial produced a discriminative
        # outcome (wedge / pass / error terminal); 2 = tooling/protocol;
        # 1 = anything else (request rejected, inconclusive).
        if verdict in ("healthy", "watchdog_wedge", "watchdog_wedge_second_request",
                       "ping_dead_no_forensics", "render_error_terminal"):
            return 0
        if verdict in ("protocol_error", "timeout_inconclusive"):
            return 2
        return 1
    if verdict == "healthy":
        return 0
    if verdict in ("protocol_error", "timeout_inconclusive"):
        return 2
    return 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_mon = sub.add_parser("monitor")
    p_mon.add_argument("--out-dir", required=True)
    p_mon.add_argument("--project-copy", required=True)
    p_mon.add_argument("--request-file", required=True)
    p_mon.add_argument("--kernel-pid", type=int, required=True)
    p_mon.add_argument("--req-endpoint", default="tcp://127.0.0.1:5555")
    p_mon.add_argument("--pub-endpoint", default="tcp://127.0.0.1:5556")
    p_mon.add_argument("--group", default="group1")
    p_mon.add_argument("--watchdog-seconds", type=float, default=140.0)
    p_mon.add_argument("--a-stack-at", type=float, default=105.0)
    p_mon.add_argument("--a-stack-gap", type=float, default=5.0)
    p_mon.add_argument("--b-stack-gap", type=float, default=4.0)
    p_mon.add_argument("--ping-interval", type=float, default=10.0)
    p_mon.add_argument("--ping-timeout", type=float, default=15.0)
    p_mon.add_argument("--observe-after-watchdog", type=float, default=45.0)
    p_mon.add_argument("--max-seconds", type=float, default=420.0)
    p_mon.add_argument("--second-request-trials", type=int, default=1)
    p_mon.add_argument("--stop-at-watchdog", action="store_true",
                       help="WEDGE-2B1 sweep mode: treat the render watchdog PUB "
                            "event as the trial outcome and stop immediately "
                            "(no B-phase forensics, no post-watchdog observation)")
    p_mon.add_argument("--sym-cache", default=os.path.join(tempfile.gettempdir(), "vit_wedge_syms"))

    p_stk = sub.add_parser("stacks")
    p_stk.add_argument("--pid", type=int, required=True)
    p_stk.add_argument("--out", required=True)
    p_stk.add_argument("--sym-cache", default=os.path.join(tempfile.gettempdir(), "vit_wedge_syms"))

    p_wct = sub.add_parser("wct")
    p_wct.add_argument("--pid", type=int, required=True)
    p_wct.add_argument("--out", required=True)

    p_dmp = sub.add_parser("dump")
    p_dmp.add_argument("--pid", type=int, required=True)
    p_dmp.add_argument("--out", required=True)

    args = parser.parse_args()
    if args.cmd == "monitor":
        sys.exit(run_monitor(args))
    if args.cmd == "stacks":
        capture_stacks(args.pid, args.out, args.sym_cache, note="standalone capture")
        sys.exit(0)
    if args.cmd == "wct":
        capture_wct(args.pid, args.out, note="standalone capture")
        sys.exit(0)
    if args.cmd == "dump":
        res = write_minidump(args.pid, args.out, note="standalone capture")
        sys.exit(0 if res.get("ok") else 2)


if __name__ == "__main__":
    main()
