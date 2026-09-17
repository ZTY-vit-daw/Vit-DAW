#include "SharedMemorySegment.h"

#if !defined(_WIN32)

#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>

#include <cinttypes>
#include <cstdint>
#include <cstdio>

namespace vit
{
namespace
{

// macOS PSHMNAMLEN is 31 including the leading '/'; kernel-published bodies
// must therefore stay at or below 30 characters (A2 empirical constraint).
constexpr size_t kPosixShmNameBodyLimit = 30;

uint64_t fnv1a64 (const std::string& text)
{
    uint64_t hash = 1469598103934665603ull;
    for (const auto c : text)
    {
        hash ^= (uint64_t) (unsigned char) c;
        hash *= 1099511628211ull;
    }
    return hash;
}

std::string describeErrno (const char* stage, int errnum)
{
    return std::string (stage) + " errno=" + std::to_string (errnum);
}

// POSIX counterpart of the Win32 publisher: shm_open(O_CREAT|O_RDWR)
// create-or-open semantics, one ftruncate to size, then a shared writable
// mapping. The fd is kept open so the object outlives the mapping, mirroring
// the Windows pattern of keeping the HANDLE after UnmapViewOfFile.
class PosixSharedMemorySegment final : public ISharedMemorySegment
{
public:
    static std::unique_ptr<PosixSharedMemorySegment> create (const std::string& descriptiveName,
                                                             size_t byteCount,
                                                             std::string& errorDetail)
    {
        const auto publishedName = platformPublishedName (descriptiveName);
        const auto posixName = "/" + publishedName;

        auto fd = shm_open (posixName.c_str(), O_CREAT | O_RDWR, 0666);
        if (fd < 0)
        {
            errorDetail = describeErrno ("shm_open_failed", errno);
            return nullptr;
        }

        struct stat st {};
        if (fstat (fd, &st) != 0)
        {
            errorDetail = describeErrno ("fstat_failed", errno);
            ::close (fd);
            return nullptr;
        }

        // darwin rounds shm storage up to 16 KiB granularity and fstat
        // reports the rounded extent, so only a logical shortfall needs one
        // truncation; an already-large-enough object is used as-is (Win32
        // create-or-open parity). A second ftruncate on an existing object
        // returns EINVAL (A2 finding), so a stale leftover from an unclean
        // kernel exit is self-healed by unlinking once and recreating — a
        // path fresh names never take.
        if ((size_t) st.st_size < byteCount)
        {
            if (ftruncate (fd, (off_t) byteCount) != 0)
            {
                const auto truncateErrno = errno;
                ::close (fd);
                if (shm_unlink (posixName.c_str()) != 0)
                {
                    errorDetail = describeErrno ("ftruncate_failed", truncateErrno);
                    return nullptr;
                }
                fd = shm_open (posixName.c_str(), O_CREAT | O_RDWR, 0666);
                if (fd < 0)
                {
                    errorDetail = describeErrno ("shm_open_after_unlink_failed", errno);
                    return nullptr;
                }
                if (ftruncate (fd, (off_t) byteCount) != 0)
                {
                    errorDetail = describeErrno ("ftruncate_failed", errno);
                    ::close (fd);
                    shm_unlink (posixName.c_str());
                    return nullptr;
                }
            }
        }

        void* mapped = mmap (nullptr, byteCount, PROT_READ | PROT_WRITE, MAP_SHARED, fd, 0);
        if (mapped == MAP_FAILED)
        {
            errorDetail = describeErrno ("mmap_failed", errno);
            ::close (fd);
            shm_unlink (posixName.c_str());
            return nullptr;
        }

        auto segment = std::make_unique<PosixSharedMemorySegment>();
        segment->fd_ = fd;
        segment->mapped_ = mapped;
        segment->byteCount_ = byteCount;
        segment->posixName_ = posixName;
        segment->publishedName_ = publishedName;
        return segment;
    }

    ~PosixSharedMemorySegment() override
    {
        if (mapped_ != nullptr)
            munmap (mapped_, byteCount_);
        if (fd_ >= 0)
            ::close (fd_);
        // Unlink so new readers fail closed (ENOENT), mirroring the Windows
        // section disappearing with its last CloseHandle. Readers that
        // already opened keep their mapping alive (POSIX unlink-while-mapped
        // semantics), so in-flight reads finish undisturbed.
        if (! posixName_.empty())
            shm_unlink (posixName_.c_str());
    }

    void* writableData() override { return mapped_; }

    void unmapView() override
    {
        if (mapped_ == nullptr)
            return;
        munmap (mapped_, byteCount_);
        mapped_ = nullptr;
    }

    const std::string& publishedName() const override { return publishedName_; }

private:
    int fd_ = -1;
    void* mapped_ = nullptr;
    size_t byteCount_ = 0;
    std::string posixName_;
    std::string publishedName_;
};

} // namespace

std::string ISharedMemorySegment::platformPublishedName (const std::string& descriptiveName)
{
    if (! descriptiveName.empty()
        && descriptiveName.size() <= kPosixShmNameBodyLimit
        && descriptiveName.find ('/') == std::string::npos)
        return descriptiveName;

    char digest[17];
    std::snprintf (digest, sizeof (digest), "%016" PRIx64, fnv1a64 (descriptiveName));
    return std::string ("VAF_") + digest;
}

std::unique_ptr<ISharedMemorySegment> ISharedMemorySegment::createAndMap (const std::string& descriptiveName,
                                                                          size_t byteCount,
                                                                          std::string& errorDetail)
{
    return PosixSharedMemorySegment::create (descriptiveName, byteCount, errorDetail);
}

} // namespace vit

#endif // !defined(_WIN32)
