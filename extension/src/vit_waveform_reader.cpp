#include "vit_waveform_reader.h"

#include <godot_cpp/classes/utility_functions.hpp>

#include <windows.h>

#include <cstring>

using namespace godot;
namespace {
constexpr int kOpenRetries = 4;
constexpr int kMapRetries = 3;
constexpr DWORD kRetrySleepMs = 1;
}

void VitWaveformReader::_bind_methods() {
    ClassDB::bind_method(D_METHOD("read_shared_memory", "memory_name", "float_count"), &VitWaveformReader::read_shared_memory);
}

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
