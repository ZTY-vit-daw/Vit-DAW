cmake_minimum_required(VERSION 3.22)

# This target is deliberately separate from VitApp.  It links only the JUCE
# VST3 hosting components and never starts, connects to, or mutates Vit.
project(PluginProbeVST3Host VERSION 0.1.0 LANGUAGES CXX)

set(PLUGINPROBE_TRACKTION_ENGINE_DIR "${CMAKE_CURRENT_LIST_DIR}/../../tracktion_engine" CACHE PATH
    "Path to the bundled Tracktion/JUCE source tree")
get_filename_component(PLUGINPROBE_TRACKTION_ENGINE_DIR "${PLUGINPROBE_TRACKTION_ENGINE_DIR}" ABSOLUTE)

if (NOT EXISTS "${PLUGINPROBE_TRACKTION_ENGINE_DIR}/CMakeLists.txt")
    message(FATAL_ERROR "Tracktion Engine source tree was not found: ${PLUGINPROBE_TRACKTION_ENGINE_DIR}")
endif()

set(TE_ADD_EXAMPLES OFF CACHE BOOL "Disable Tracktion examples" FORCE)
set(TE_ADD_VIT_KERNEL OFF CACHE BOOL "Disable VitKernel target" FORCE)
add_subdirectory("${PLUGINPROBE_TRACKTION_ENGINE_DIR}" "${CMAKE_CURRENT_BINARY_DIR}/tracktion_engine")

juce_add_console_app(pluginprobe_vst3_worker
    PRODUCT_NAME "Plugin Probe VST3 Worker"
    COMPANY_NAME "Vit-DAW")

# The witness host is deliberately a separate GUI process. It loads a single
# VST3 editor for user-operated semantic evidence and has no connection to
# Vit or another project runtime.
juce_add_gui_app(pluginprobe_vst3_witness
    PRODUCT_NAME "pluginprobe_vst3_witness"
    COMPANY_NAME "Vit-DAW")

target_compile_features(pluginprobe_vst3_worker PRIVATE cxx_std_20)
target_sources(pluginprobe_vst3_worker PRIVATE
    src/main.cpp
    "${PLUGINPROBE_TRACKTION_ENGINE_DIR}/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK/public.sdk/source/vst/hosting/module.cpp"
    "${PLUGINPROBE_TRACKTION_ENGINE_DIR}/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK/public.sdk/source/vst/hosting/module_win32.cpp")

target_compile_features(pluginprobe_vst3_witness PRIVATE cxx_std_20)
target_sources(pluginprobe_vst3_witness PRIVATE
    src/witness_main.cpp)

target_compile_definitions(pluginprobe_vst3_worker PRIVATE
    JUCE_PLUGINHOST_VST3=1
    JUCE_USE_CURL=0
    JUCE_WEB_BROWSER=0
    JUCE_MODAL_LOOPS_PERMITTED=1
    JUCE_STRICT_REFCOUNTEDPOINTER=1)

target_compile_definitions(pluginprobe_vst3_witness PRIVATE
    JUCE_PLUGINHOST_VST3=1
    JUCE_USE_CURL=0
    JUCE_WEB_BROWSER=0
    JUCE_MODAL_LOOPS_PERMITTED=1
    JUCE_STRICT_REFCOUNTEDPOINTER=1
    JUCE_APPLICATION_NAME_STRING="$<TARGET_PROPERTY:pluginprobe_vst3_witness,JUCE_PRODUCT_NAME>"
    JUCE_APPLICATION_VERSION_STRING="$<TARGET_PROPERTY:pluginprobe_vst3_witness,JUCE_VERSION>")

target_link_libraries(pluginprobe_vst3_worker PRIVATE
    juce::juce_audio_processors_headless
    juce::juce_audio_formats
    juce::juce_events
    bcrypt
    juce::juce_recommended_config_flags
    juce::juce_recommended_lto_flags
    juce::juce_recommended_warning_flags)

target_link_libraries(pluginprobe_vst3_witness PRIVATE
    juce::juce_audio_processors
    juce::juce_audio_formats
    juce::juce_audio_utils
    juce::juce_gui_extra
    juce::juce_recommended_config_flags
    juce::juce_recommended_lto_flags
    juce::juce_recommended_warning_flags)

if (MSVC)
    # The bundled VST3 hosting helper predates C++20 char8_t filesystem return
    # types. Keeping the MSVC compatibility switch local to this worker avoids
    # modifying the vendored SDK or the main Vit target.
    target_compile_options(pluginprobe_vst3_worker PRIVATE /W4 /permissive- /Zc:char8_t-)
    target_compile_options(pluginprobe_vst3_witness PRIVATE /W4 /permissive- /Zc:char8_t-)
else()
    target_compile_options(pluginprobe_vst3_worker PRIVATE -Wall -Wextra -Wpedantic)
    target_compile_options(pluginprobe_vst3_witness PRIVATE -Wall -Wextra -Wpedantic)
endif()
