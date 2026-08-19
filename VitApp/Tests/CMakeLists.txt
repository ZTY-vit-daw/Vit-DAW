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
else()
    message(FATAL_ERROR
        "Tracktion Engine not found. Set -DVIT_TRACKTION_ENGINE_DIR=<path> to a populated Tracktion Engine checkout.")
endif()

if(MSVC)
    target_compile_options(VitAuditionPreviewStateTests PRIVATE /W4 /permissive-)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE /W4 /permissive-)
    endif()
else()
    target_compile_options(VitAuditionPreviewStateTests PRIVATE -Wall -Wextra -Wpedantic)
    if(TARGET VitAuditionPreviewAudioPlaneTests)
        target_compile_options(VitAuditionPreviewAudioPlaneTests PRIVATE -Wall -Wextra -Wpedantic)
    endif()
endif()

add_test(NAME VitAuditionPreviewStateTests COMMAND VitAuditionPreviewStateTests)
