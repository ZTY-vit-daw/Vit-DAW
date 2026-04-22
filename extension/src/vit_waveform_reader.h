#ifndef VIT_WAVEFORM_READER_H
#define VIT_WAVEFORM_READER_H

#include <godot_cpp/classes/ref_counted.hpp>
#include <godot_cpp/core/class_db.hpp>
#include <godot_cpp/variant/packed_float32_array.hpp>
#include <godot_cpp/variant/string.hpp>

namespace godot {

class VitWaveformReader : public RefCounted {
    GDCLASS(VitWaveformReader, RefCounted)

protected:
    static void _bind_methods();

public:
    PackedFloat32Array read_shared_memory(String memory_name, int float_count);
};

} // namespace godot

#endif // VIT_WAVEFORM_READER_H
