cmake_minimum_required(VERSION 3.22)
project(VitAuditionPreviewTests VERSION 1.0.0 LANGUAGES C CXX)

enable_testing()

add_executable(VitAuditionPreviewStateTests
    AuditionPreviewStateTests.cpp
    ../Source/Service/AuditionPreviewState.cpp)

target_compile_features(VitAuditionPreviewStateTests PRIVATE cxx_std_20)
target_include_directories(VitAuditionPreviewStateTests PRIVATE ../Source/Service)

# B15: true manual-save semantics — the auto-persist gate that keeps
# agent-governed VSP mutations out of the on-disk working copy.
add_executable(VitWorkingCopyPersistPolicyTests
    WorkingCopyPersistPolicyTests.cpp
    ../Source/Service/VitWorkingCopyPersistPolicy.h)
target_compile_features(VitWorkingCopyPersistPolicyTests PRIVATE cxx_std_20)
target_include_directories(VitWorkingCopyPersistPolicyTests PRIVATE ../Source/Service)

# PORT-A1: publisher-side shm segment interface — platform name policy and
# create/write/unmap/release lifecycle (Win32 mapping semantics / POSIX
# shm_open+ftruncate+mmap). Pure std, no juce/tracktion linkage.
add_executable(VitSharedMemorySegmentTests
    SharedMemorySegmentTests.cpp
    ../Source/Service/SharedMemorySegmentWindows.cpp
    ../Source/Service/SharedMemorySegmentPosix.cpp)
target_compile_features(VitSharedMemorySegmentTests PRIVATE cxx_std_20)
target_include_directories(VitSharedMemorySegmentTests PRIVATE ../Source/Service)

set(_g4_default_tracktion_engine "${CMAKE_CURRENT_SOURCE_DIR}/../../tracktion_engine")
if(NOT EXISTS "${_g4_default_tracktion_engine}/CMakeLists.txt"
   AND EXISTS "D:/Vit_DAW/tracktion_engine/CMakeLists.txt")
    set(_g4_default_tracktion_engine "D:/Vit_DAW/tracktion_engine")
endif()
set(VIT_TRACKTION_ENGINE_DIR "${_g4_default_tracktion_engine}" CACHE PATH
    "Path to the Tracktion Engine source tree for real audio-plane tests")
if(EXISTS "${VIT_TRACKTION_ENGINE_DIR}/CMakeLists.txt")
    set(TE_ADD_EXAMPLES OFF CACHE BOOL "" FORCE)
    add_subdirectory("${VIT_TRACKTION_ENGINE_DIR}" "${CMAKE_CURRENT_BINARY_DIR}/tracktion_engine")

    juce_add_console_app(VitAuditionPreviewAudioPlaneTests PRODUCT_NAME "VitAuditionPreviewAudioPlaneTests")
    juce_generate_juce_header(VitAuditionPreviewAudioPlaneTests)
    target_sources(VitAuditionPreviewAudioPlaneTests PRIVATE
        AuditionPreviewAudioPlaneTests.cpp
        ../Source/Service/AuditionPreviewAudioPlane.cpp
        ../Source/Service/AuditionPreviewAudioPlane.h
        ../Source/Service/AuditionPreviewService.cpp
        ../Source/Service/AuditionPreviewService.h
        ../Source/Service/AuditionPreviewState.cpp
        ../Source/Service/AuditionPreviewState.h)
    target_compile_features(VitAuditionPreviewAudioPlaneTests PRIVATE cxx_std_20)
    target_include_directories(VitAuditionPreviewAudioPlaneTests PRIVATE ../Source)
    target_compile_definitions(VitAuditionPreviewAudioPlaneTests PRIVATE JUCE_USE_CURL=0 JUCE_WEB_BROWSER=0)
    target_link_libraries(VitAuditionPreviewAudioPlaneTests PRIVATE
        tracktion::tracktion_core
        tracktion::tracktion_engine
        tracktion::tracktion_graph
        juce::juce_audio_utils
        juce::juce_recommended_config_flags)
    add_test(NAME VitAuditionPreviewAudioPlaneTests COMMAND VitAuditionPreviewAudioPlaneTests)

    # KERNEL-RENDER-1: render watchdog tests run the real VitProductionCoordinator
    # (plus its CompressorDualTapEvidence dependency), so they need the same
    # tracktion/juce/libsodium linkage as the kernel target. The sodium bundle
    # mirrors the extraction in the top-level VitApp CMakeLists.
    set(VIT_RENDER_WATCHDOG_SODIUM_ARCHIVE "${CMAKE_CURRENT_SOURCE_DIR}/../third_party/libsodium-1.0.20.tar.gz")
    if(NOT EXISTS "${VIT_RENDER_WATCHDOG_SODIUM_ARCHIVE}")
        message(FATAL_ERROR
            "RenderWatchdogTests: missing libsodium archive. Place libsodium-1.0.20.tar.gz at:\n  ${VIT_RENDER_WATCHDOG_SODIUM_ARCHIVE}")
    endif()
    set(VIT_RENDER_WATCHDOG_SODIUM_EXTRACT_DIR "${CMAKE_CURRENT_BINARY_DIR}/_deps/libsodium_upstream-src")
    set(VIT_RENDER_WATCHDOG_SODIUM_TREE "${VIT_RENDER_WATCHDOG_SODIUM_EXTRACT_DIR}/libsodium-1.0.20")
    if(NOT EXISTS "${VIT_RENDER_WATCHDOG_SODIUM_TREE}/src/libsodium/include/sodium.h")
        file(MAKE_DIRECTORY "${VIT_RENDER_WATCHDOG_SODIUM_EXTRACT_DIR}")
        execute_process(
            COMMAND ${CMAKE_COMMAND} -E tar xzf "${VIT_RENDER_WATCHDOG_SODIUM_ARCHIVE}"
            WORKING_DIRECTORY "${VIT_RENDER_WATCHDOG_SODIUM_EXTRACT_DIR}"
            RESULT_VARIABLE VIT_RENDER_WATCHDOG_SODIUM_RC)
        if(NOT VIT_RENDER_WATCHDOG_SODIUM_RC EQUAL 0)
            message(FATAL_ERROR
                "RenderWatchdogTests: failed to extract libsodium archive (cmake -E tar), code ${VIT_RENDER_WATCHDOG_SODIUM_RC}")
        endif()
    endif()
    set(VIT_LIBSODIUM_ROOT "${VIT_RENDER_WATCHDOG_SODIUM_TREE}" CACHE INTERNAL "")
    add_subdirectory("${CMAKE_CURRENT_SOURCE_DIR}/../cmake/libsodium_bundle"
        "${CMAKE_CURRENT_BINARY_DIR}/libsodium_bundle")

    juce_add_console_app(VitRenderWatchdogTests PRODUCT_NAME "VitRenderWatchdogTests")
    juce_generate_juce_header(VitRenderWatchdogTests)
    target_sources(VitRenderWatchdogTests PRIVATE
        RenderWatchdogTests.cpp
        ../Source/Service/VitProductionCoordinator.cpp
        ../Source/Service/VitProductionCoordinator.h
        ../Source/Service/CompressorDualTapEvidence.cpp
        ../Source/Service/CompressorDualTapEvidence.h)
    target_compile_features(VitRenderWatchdogTests PRIVATE cxx_std_20)
    target_include_directories(VitRenderWatchdogTests PRIVATE ../Source)
    target_compile_definitions(VitRenderWatchdogTests PRIVATE JUCE_USE_CURL=0 JUCE_WEB_BROWSER=0)
    target_link_libraries(VitRenderWatchdogTests PRIVATE
        tracktion::tracktion_core
        tracktion::tracktion_engine
        tracktion::tracktion_graph
        juce::juce_audio_utils
        sodium
        juce::juce_recommended_config_flags)
    add_test(NAME VitRenderWatchdogTests COMMAND VitRenderWatchdogTests)

    # FIX-KERNEL-PLUGINLIST-HYGIENE-1: plugin-list hygiene acceptance tests
    # (startup lazy cleanup / user-cancel epilogue / crash-contrast defence).
    # JUCE-only: KnownPluginList + PluginDirectoryScanner, no tracktion linkage.
    juce_add_console_app(VitPluginListHygieneTests PRODUCT_NAME "VitPluginListHygieneTests")
    juce_generate_juce_header(VitPluginListHygieneTests)
    target_sources(VitPluginListHygieneTests PRIVATE
        PluginListHygieneTests.cpp
        ../Source/Service/PluginListHygiene.cpp
        ../Source/Service/PluginListHygiene.h)
    target_compile_features(VitPluginListHygieneTests PRIVATE cxx_std_20)
    target_include_directories(VitPluginListHygieneTests PRIVATE ../Source)
    target_compile_definitions(VitPluginListHygieneTests PRIVATE JUCE_USE_CURL=0 JUCE_WEB_BROWSER=0 JUCE_PLUGINHOST_VST3=1)
    target_link_libraries(VitPluginListHygieneTests PRIVATE
        juce::juce_audio_processors
        juce::juce_recommended_config_flags)
    add_test(NAME VitPluginListHygieneTests COMMAND VitPluginListHygieneTests)

    # L2-2-SEG-1: segmentation primitives v1 — deterministic replay, structural
    # onset/novelty contents, and explicit not_evaluable states. JUCE-only
    # linkage (L3AcousticAnalyzer + AudioFeatureTypes), no tracktion.
    juce_add_console_app(VitSegmentationPrimitivesTests PRODUCT_NAME "VitSegmentationPrimitivesTests")
    juce_generate_juce_header(VitSegmentationPrimitivesTests)
    target_sources(VitSegmentationPrimitivesTests PRIVATE
        SegmentationPrimitivesTests.cpp
        ../Source/Service/L3AcousticAnalyzer.cpp
        ../Source/Service/L3AcousticAnalyzer.h
        ../Source/Service/AudioFeatureTypes.cpp
        ../Source/Service/AudioFeatureTypes.h)
    target_compile_features(VitSegmentationPrimitivesTests PRIVATE cxx_std_20)
    target_include_directories(VitSegmentationPrimitivesTests PRIVATE ../Source)
    target_compile_definitions(VitSegmentationPrimitivesTests PRIVATE JUCE_USE_CURL=0 JUCE_WEB_BROWSER=0)
    target_link_libraries(VitSegmentationPrimitivesTests PRIVATE
        juce::juce_dsp
        juce::juce_audio_utils
        juce::juce_recommended_config_flags)
    add_test(NAME VitSegmentationPrimitivesTests COMMAND VitSegmentationPrimitivesTests)
else()
    message(FATAL_ERROR
        "Tracktion Engine not found. Set -DVIT_TRACKTION_ENGINE_DIR=<path> to a populated Tracktion Engine checkout.")
endif()

if(MSVC)
    target_compile_options(VitAuditionPreviewStateTests PRIVATE /W4 /permissive-)
    target_compile_options(VitWorkingCopyPersistPolicyTests PRIVATE /W4 /permissive-)
    target_compile_options(VitSharedMemorySegmentTests PRIVATE /W4 /permissive-)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE /W4 /permissive-)
    endif()
    if(TARGET VitRenderWatchdogTests)
        target_compile_options(VitRenderWatchdogTests PRIVATE /W4 /permissive-)
    endif()
    if(TARGET VitPluginListHygieneTests)
        target_compile_options(VitPluginListHygieneTests PRIVATE /W4 /permissive-)
    endif()
    if(TARGET VitSegmentationPrimitivesTests)
        target_compile_options(VitSegmentationPrimitivesTests PRIVATE /W4 /permissive-)
    endif()
else()
    target_compile_options(VitAuditionPreviewStateTests PRIVATE -Wall -Wextra -Wpedantic)
    target_compile_options(VitWorkingCopyPersistPolicyTests PRIVATE -Wall -Wextra -Wpedantic)
    target_compile_options(VitSharedMemorySegmentTests PRIVATE -Wall -Wextra -Wpedantic)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
    if(TARGET VitRenderWatchdogTests)
        target_compile_options(VitRenderWatchdogTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
    if(TARGET VitPluginListHygieneTests)
        target_compile_options(VitPluginListHygieneTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
    if(TARGET VitSegmentationPrimitivesTests)
        target_compile_options(VitSegmentationPrimitivesTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
endif()

add_test(NAME VitAuditionPreviewStateTests COMMAND VitAuditionPreviewStateTests)
add_test(NAME VitWorkingCopyPersistPolicyTests COMMAND VitWorkingCopyPersistPolicyTests)
add_test(NAME VitSharedMemorySegmentTests COMMAND VitSharedMemorySegmentTests)
