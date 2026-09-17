#include "SharedMemorySegment.h"

#if defined(_WIN32)

#include <windows.h>

namespace vit
{
namespace
{

std::string describeWinError (const char* stage)
{
    return std::string (stage) + " win_error=" + std::to_string ((unsigned long long) GetLastError());
}

// Straight transplant of the pre-A1 inline baker call sites:
// CreateFileMappingA(page-file backed, PAGE_READWRITE) + MapViewOfFile
// (FILE_MAP_ALL_ACCESS, exact byte count). Destruction unmaps and closes,
// so the section disappears with the last handle — the same lifetime the
// bakers relied on for generation retirement.
class WindowsSharedMemorySegment final : public ISharedMemorySegment
{
public:
    static std::unique_ptr<WindowsSharedMemorySegment> create (const std::string& descriptiveName,
                                                               size_t byteCount,
                                                               std::string& errorDetail)
    {
        const auto publishedName = platformPublishedName (descriptiveName);
        const auto bytes = (SIZE_T) byteCount;
        HANDLE handle = CreateFileMappingA (INVALID_HANDLE_VALUE,
                                            nullptr,
                                            PAGE_READWRITE,
                                            0,
                                            (DWORD) bytes,
                                            publishedName.c_str());
        if (handle == nullptr)
        {
            errorDetail = describeWinError ("create_mapping_failed");
            return nullptr;
        }

        auto* mapped = MapViewOfFile (handle, FILE_MAP_ALL_ACCESS, 0, 0, bytes);
        if (mapped == nullptr)
        {
            errorDetail = describeWinError ("map_view_failed");
            CloseHandle (handle);
            return nullptr;
        }

        auto segment = std::make_unique<WindowsSharedMemorySegment>();
        segment->handle_ = handle;
        segment->mapped_ = mapped;
        segment->publishedName_ = publishedName;
        return segment;
    }

    ~WindowsSharedMemorySegment() override
    {
        if (mapped_ != nullptr)
            UnmapViewOfFile (mapped_);
        if (handle_ != nullptr)
            CloseHandle (handle_);
    }

    void* writableData() override { return mapped_; }

    void unmapView() override
    {
        if (mapped_ == nullptr)
            return;
        UnmapViewOfFile (mapped_);
        mapped_ = nullptr;
    }

    const std::string& publishedName() const override { return publishedName_; }

private:
    HANDLE handle_ = nullptr;
    void* mapped_ = nullptr;
    std::string publishedName_;
};

} // namespace

std::string ISharedMemorySegment::platformPublishedName (const std::string& descriptiveName)
{
    return descriptiveName;
}

std::unique_ptr<ISharedMemorySegment> ISharedMemorySegment::createAndMap (const std::string& descriptiveName,
                                                                          size_t byteCount,
                                                                          std::string& errorDetail)
{
    return WindowsSharedMemorySegment::create (descriptiveName, byteCount, errorDetail);
}

} // namespace vit

#endif // defined(_WIN32)
