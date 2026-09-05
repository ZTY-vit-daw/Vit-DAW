cmake_minimum_required(VERSION 3.22)
project(VitAuditionPreviewTests VERSION 1.0.0 LANGUAGES CXX)

enable_testing()

add_executable(VitAuditionPreviewStateTests
    AuditionPreviewStateTests.cpp
    ../Source/Service/AuditionPreviewState.cpp)

target_compile_features(VitAuditionPreviewStateTests PRIVATE cxx_std_20)
target_include_directories(VitAuditionPreviewStateTests PRIVATE ../Source/Service)

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
else()
    message(FATAL_ERROR
        "Tracktion Engine not found. Set -DVIT_TRACKTION_ENGINE_DIR=<path> to a populated Tracktion Engine checkout.")
endif()

if(MSVC)
    target_compile_options(VitAuditionPreviewStateTests PRIVATE /W4 /permissive-)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE /W4 /permissive-)
    endif()
    if(TARGET VitRenderWatchdogTests)
        target_compile_options(VitRenderWatchdogTests PRIVATE /W4 /permissive-)
    endif()
else()
    target_compile_options(VitAuditionPreviewStateTests PRIVATE -Wall -Wextra -Wpedantic)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
    if(TARGET VitRenderWatchdogTests)
        target_compile_options(VitRenderWatchdogTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
endif()

add_test(NAME VitAuditionPreviewStateTests COMMAND VitAuditionPreviewStateTests)
