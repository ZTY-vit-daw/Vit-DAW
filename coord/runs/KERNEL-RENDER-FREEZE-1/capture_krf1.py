# KERNEL-RENDER-FREEZE-1 step-C forensics capture on the frozen kernel.
# Patches rwp.ensure_pdb to cache-only (no network) to avoid the ole32 stall.
import os
import sys

REPO = r"D:\Vit_DAW"
sys.path.insert(0, os.path.join(REPO, "scripts"))
import render_wedge_probe as rwp  # noqa: E402

pid = int(sys.argv[1])
run_dir = sys.argv[2]
sym_cache = sys.argv[3]


def make_patched():
    def ensure_pdb_no_net(pe_path, cache_dir):
        try:
            info = rwp.get_pdb_guid(pe_path)
            if info:
                name = info[0]
                dest = os.path.join(cache_dir, name)
                if os.path.exists(dest) and os.path.getsize(dest) > 0:
                    return True, "cached"
        except Exception:
            pass
        return False, "cache-only: not cached, no download"
    return ensure_pdb_no_net


rwp.ensure_pdb = make_patched()

rwp.capture_stacks(pid, os.path.join(run_dir, "stacks_frozen.json"), sym_cache,
                   note="KERNEL-RENDER-FREEZE-1 frozen capture (cache-only symbols)")
print("STACKS_OK", flush=True)
rwp.capture_wct(pid, os.path.join(run_dir, "wct_frozen.json"),
                note="KERNEL-RENDER-FREEZE-1 frozen wait chains")
print("WCT_OK", flush=True)
res = rwp.write_minidump(pid, os.path.join(run_dir, "minidump_frozen.dmp"),
                         note="KERNEL-RENDER-FREEZE-1 frozen dump")
print("DUMP", res, flush=True)
