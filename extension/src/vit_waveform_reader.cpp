#include "vit_waveform_reader.h"

#if __has_include(<godot_cpp/variant/utility_functions.hpp>)
#include <godot_cpp/variant/utility_functions.hpp>
#else
#include <godot_cpp/classes/utility_functions.hpp>
#endif

#ifdef _WIN32
#include <windows.h>
#else
#include <fcntl.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <unistd.h>
#endif

#include <cstring>
#include <string>

using namespace godot;
namespace {
constexpr int kOpenRetries = 4;
constexpr int kMapRetries = 3;
#ifdef _WIN32
constexpr DWORD kRetrySleepMs = 1;
#else
constexpr useconds_t kRetrySleepUs = 1000;
#endif
}

void VitWaveformReader::_bind_methods() {
    ClassDB::bind_method(D_METHOD("read_shared_memory", "memory_name", "float_count"), &VitWaveformReader::read_shared_memory);
}

#ifdef _WIN32

PackedFloat32Array VitWaveformReader::read_shared_memory(String memory_name, int float_count) {
    PackedFloat32Array result;

    if (float_count <= 0) {
        return result;
    }

    CharString utf8_name = memory_name.utf8();
    const char *name = utf8_name.get_data();

    HANDLE file_mapping = nullptr;
    DWORD open_error = ERROR_SUCCESS;
    for (int i = 0; i < kOpenRetries; ++i) {
        file_mapping = OpenFileMappingA(FILE_MAP_READ, FALSE, name);
        if (file_mapping != nullptr) {
            break;
        }
        open_error = GetLastError();
        if (i + 1 < kOpenRetries) {
            Sleep(kRetrySleepMs);
        }
    }
    if (file_mapping == nullptr) {
        if (open_error == ERROR_FILE_NOT_FOUND) {
            return result;
        }
        UtilityFunctions::push_warning(
            String("OpenFileMappingA failed for shared memory: ")
            + memory_name
            + String(" error=")
            + String::num_int64((int64_t)open_error)
        );
        return result;
    }

    const void *mapped_view = nullptr;
    DWORD map_error = ERROR_SUCCESS;
    for (int i = 0; i < kMapRetries; ++i) {
        mapped_view = MapViewOfFile(file_mapping, FILE_MAP_READ, 0, 0, static_cast<SIZE_T>(float_count) * sizeof(float));
        if (mapped_view != nullptr) {
            break;
        }
        map_error = GetLastError();
        if (i + 1 < kMapRetries) {
            Sleep(kRetrySleepMs);
        }
    }
    if (mapped_view == nullptr) {
        UtilityFunctions::push_warning(
            String("MapViewOfFile failed for shared memory: ")
            + memory_name
            + String(" error=")
            + String::num_int64((int64_t)map_error)
        );
        CloseHandle(file_mapping);
        return result;
    }

    const float *source = static_cast<const float *>(mapped_view);

    result.resize(float_count);
    {
        float *destination = result.ptrw();
        std::memcpy(destination, source, static_cast<size_t>(float_count) * sizeof(float));
    }

    UnmapViewOfFile(mapped_view);
    CloseHandle(file_mapping);

    return result;
}

#else

// POSIX counterpart of the Win32 reader: the kernel publishes its segments as
// shm_open objects with unprefixed names (SharedMemorySegmentPosix.cpp), which
// must resolve through the same read_shared_memory() contract. Semantics mirror
// harness/shm_darwin.go: "/"-prefixed name, read-only descriptor, fstat size
// guard before mapping (a too-short request must fail closed instead of
// SIGBUS on access), PROT_READ|MAP_SHARED of exactly the requested extent.
PackedFloat32Array VitWaveformReader::read_shared_memory(String memory_name, int float_count) {
    PackedFloat32Array result;

    if (float_count <= 0) {
        return result;
    }

    CharString utf8_name = memory_name.utf8();
    std::string name(utf8_name.get_data());
    if (name.empty() || name.front() != '/') {
        name = "/" + name;
    }

    int fd = -1;
    int open_errno = 0;
    for (int i = 0; i < kOpenRetries; ++i) {
        fd = shm_open(name.c_str(), O_RDONLY, 0);
        if (fd >= 0) {
            break;
        }
        open_errno = errno;
        if (i + 1 < kOpenRetries) {
            usleep(kRetrySleepUs);
        }
    }
    if (fd < 0) {
        if (open_errno == ENOENT) {
            return result;
        }
        UtilityFunctions::push_warning(
            String("shm_open failed for shared memory: ")
            + memory_name
            + String(" errno=")
            + String::num_int64((int64_t)open_errno)
        );
        return result;
    }

    struct stat segment_stat {};
    if (fstat(fd, &segment_stat) != 0) {
        UtilityFunctions::push_warning(
            String("fstat failed for shared memory: ")
            + memory_name
            + String(" errno=")
            + String::num_int64((int64_t)errno)
        );
        close(fd);
        return result;
    }

    const size_t byte_count = static_cast<size_t>(float_count) * sizeof(float);
    if (static_cast<size_t>(segment_stat.st_size) < byte_count) {
        UtilityFunctions::push_warning(
            String("shared memory too small: ")
            + memory_name
            + String(" have=")
            + String::num_int64((int64_t)segment_stat.st_size)
            + String(" need=")
            + String::num_int64((int64_t)byte_count)
        );
        close(fd);
        return result;
    }

    void *mapped_view = mmap(nullptr, byte_count, PROT_READ, MAP_SHARED, fd, 0);
    if (mapped_view == MAP_FAILED) {
        UtilityFunctions::push_warning(
            String("mmap failed for shared memory: ")
            + memory_name
            + String(" errno=")
            + String::num_int64((int64_t)errno)
        );
        close(fd);
        return result;
    }

    const float *source = static_cast<const float *>(mapped_view);

    result.resize(float_count);
    {
        float *destination = result.ptrw();
        std::memcpy(destination, source, byte_count);
    }

    munmap(mapped_view, byte_count);
    close(fd);

    return result;
}

#endif
