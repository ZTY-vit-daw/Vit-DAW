#pragma once

#include <cstddef>
#include <memory>
#include <string>

namespace vit
{

// Publisher-side shared-memory segment abstraction (PORT-A1).
//
// The kernel publishes audio-feature float payloads (waveform envelope tiles,
// spectrogram tiles and the SharedMemoryTester smoke segment) through this
// interface. Readers live outside the kernel: the Go agent opens the name
// carried by the event payload via OpenFileMappingA on Windows and via
// shm_open(O_RDONLY) on macOS after prepending the leading slash. That read
// contract is frozen (PORT-A2 ruling) — the published name is the only
// cross-process surface, and readers resolve it verbatim.
//
// Published-name mapping (explicit, two platforms):
//   Windows — the descriptive name is published verbatim, e.g.
//     "Vit_AudioFeature_waveform_<key>_g<gen>_<tile>",
//     "Vit_Waveform_<key>_g<gen>_<tile>" and the tester's
//     "Vit_Waveform_Test"; identical to the pre-A1 inline CreateFileMappingA
//     call sites (zero behavioural regression).
//   POSIX   — macOS caps shm names at PSHMNAMLEN (31 chars including the
//     leading '/'), which the descriptive baker names exceed. A descriptive
//     name whose body fits in <=30 chars and contains no '/' is published
//     as-is; anything longer collapses to "VAF_" followed by the 16-char
//     lowercase hex FNV-1a-64 digest of the descriptive name (20 body chars,
//     21 including the leading slash). The digest is deterministic, so every
//     descriptive name maps onto exactly one POSIX segment name. The Go
//     reader needs no change: it opens whatever name the payload carries
//     (posixShmName only prepends the slash).
class ISharedMemorySegment
{
public:
    virtual ~ISharedMemorySegment() = default;

    // Maps a descriptive segment name onto the platform-published name (see
    // the mapping note above). Pure function; safe to call before creation.
    static std::string platformPublishedName (const std::string& descriptiveName);

    // Creates (or opens, matching Win32 CreateFileMapping semantics) a
    // read-write segment of byteCount bytes and maps a writable view.
    // Returns nullptr on failure; errorDetail then carries a stage tag plus
    // the platform error code ("create_mapping_failed win_error=8",
    // "shm_open_failed errno=2", ...) for the caller's diagnostics.
    static std::unique_ptr<ISharedMemorySegment> createAndMap (const std::string& descriptiveName,
                                                               size_t byteCount,
                                                               std::string& errorDetail);

    // The writable mapped view (byteCount bytes). Valid from a successful
    // createAndMap until unmapView() or destruction.
    virtual void* writableData() = 0;

    // Drops the writable view while keeping the segment alive for readers —
    // mirrors the bakers' historical UnmapViewOfFile-before-retire pattern.
    virtual void unmapView() = 0;

    // The name to publish in the event payload (already platform-mapped).
    virtual const std::string& publishedName() const = 0;
};

} // namespace vit
