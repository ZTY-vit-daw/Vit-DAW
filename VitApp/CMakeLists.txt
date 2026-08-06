cmake_minimum_required(VERSION 3.22)

project(VitApp VERSION 1.0.0 LANGUAGES CXX)

if (CMAKE_VERSION VERSION_GREATER_EQUAL "4.0")
    set(CMAKE_POLICY_VERSION_MINIMUM 3.5)
endif()

include(FetchContent)

set(VIT_TRACKTION_ENGINE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/../tracktion_engine" CACHE PATH
    "Path to the Tracktion Engine source tree")
set(VIT_WORKSPACE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/Workspace" CACHE PATH
    "Path to the VitApp workspace directory")
set(VIT_DEFAULT_PROJECT_XML_RELATIVE "Workspace/default_project.xml" CACHE STRING
    "Default project XML path relative to the VitApp project root")

get_filename_component(VIT_TRACKTION_ENGINE_DIR "${VIT_TRACKTION_ENGINE_DIR}" ABSOLUTE)
get_filename_component(VIT_WORKSPACE_DIR "${VIT_WORKSPACE_DIR}" ABSOLUTE)
file(MAKE_DIRECTORY "${VIT_WORKSPACE_DIR}")

if(NOT EXISTS "${VIT_TRACKTION_ENGINE_DIR}/CMakeLists.txt")
    message(FATAL_ERROR "tracktion_engine not found at: ${VIT_TRACKTION_ENGINE_DIR}")
endif()

set_property(GLOBAL PROPERTY USE_FOLDERS YES)
option(JUCE_ENABLE_MODULE_SOURCE_GROUPS "Enable Module Source Groups" ON)

set(TE_ADD_EXAMPLES OFF CACHE BOOL "Disable Tracktion examples" FORCE)
set(TE_ADD_VIT_KERNEL OFF CACHE BOOL "Disable legacy VitKernel target" FORCE)

add_subdirectory("${VIT_TRACKTION_ENGINE_DIR}" "${CMAKE_CURRENT_BINARY_DIR}/tracktion_engine")

find_package(cppzmq CONFIG QUIET)
find_package(ZeroMQ CONFIG QUIET)

if (NOT TARGET cppzmq OR
    NOT (TARGET libzmq OR TARGET libzmq-static OR TARGET zmq))
    message(STATUS "ZeroMQ packages not found locally. Fetching libzmq/cppzmq with FetchContent.")

    set(BUILD_TESTS OFF CACHE BOOL "" FORCE)
    set(BUILD_SHARED OFF CACHE BOOL "" FORCE)
    set(ZMQ_BUILD_TESTS OFF CACHE BOOL "" FORCE)
    set(CPPZMQ_BUILD_TESTS OFF CACHE BOOL "" FORCE)
    set(WITH_TLS OFF CACHE BOOL "" FORCE)
    set(WITH_PERF_TOOL OFF CACHE BOOL "" FORCE)
    set(ENABLE_CURVE OFF CACHE BOOL "" FORCE)

    # Tarballs avoid git clone directory-lock issues on some Windows setups.
    FetchContent_Declare(libzmq
        URL https://github.com/zeromq/libzmq/archive/refs/tags/v4.3.5.tar.gz
        URL_HASH SHA256=6c972d1e6a91a0ecd79c3236f04cf0126f2f4dfbbad407d72b4606a7ba93f9c6
        DOWNLOAD_EXTRACT_TIMESTAMP TRUE)
    FetchContent_MakeAvailable(libzmq)

    FetchContent_Declare(cppzmq
        URL https://github.com/zeromq/cppzmq/archive/refs/tags/v4.10.0.tar.gz
        URL_HASH SHA256=c81c81bba8a7644c84932225f018b5088743a22999c6d82a2b5f5cd1e6942b74
        DOWNLOAD_EXTRACT_TIMESTAMP TRUE)
    FetchContent_MakeAvailable(cppzmq)
endif()

if (TARGET libzmq)
    set(VIT_ZMQ_TARGETS cppzmq libzmq)
elseif (TARGET libzmq-static)
    set(VIT_ZMQ_TARGETS cppzmq-static libzmq-static)
elseif (TARGET zmq)
    set(VIT_ZMQ_TARGETS cppzmq zmq)
else()
    message(FATAL_ERROR "ZeroMQ library target not found. Install libzmq/cppzmq locally or allow FetchContent to download them.")
endif()

# libsodium (VitEncryptionCore): unpack VitApp/third_party/libsodium-1.0.20.tar.gz (offline; no FetchContent URL)
set(VIT_LIBSODIUM_ARCHIVE "${CMAKE_CURRENT_SOURCE_DIR}/third_party/libsodium-1.0.20.tar.gz")
if (NOT EXISTS "${VIT_LIBSODIUM_ARCHIVE}")
    message(FATAL_ERROR
        "VitApp: missing libsodium archive. Place libsodium-1.0.20.tar.gz at:\n  ${VIT_LIBSODIUM_ARCHIVE}\n"
        "Release: https://github.com/jedisct1/libsodium/releases/download/1.0.20-RELEASE/libsodium-1.0.20.tar.gz")
endif()
file(SHA256 "${VIT_LIBSODIUM_ARCHIVE}" VIT_LIBSODIUM_SHA256)
string(TOLOWER "${VIT_LIBSODIUM_SHA256}" VIT_LIBSODIUM_SHA256)
if (NOT VIT_LIBSODIUM_SHA256 STREQUAL "ebb65ef6ca439333c2bb41a0c1990587288da07f6c7fd07cb3a18cc18d30ce19")
    message(FATAL_ERROR "VitApp: libsodium archive SHA256 mismatch (corrupt or wrong file). Got: ${VIT_LIBSODIUM_SHA256}")
endif()
set(VIT_LIBSODIUM_EXTRACT_DIR "${CMAKE_BINARY_DIR}/_deps/libsodium_upstream-src")
set(VIT_LIBSODIUM_TREE "${VIT_LIBSODIUM_EXTRACT_DIR}/libsodium-1.0.20")
if (NOT EXISTS "${VIT_LIBSODIUM_TREE}/src/libsodium/include/sodium.h")
    file(MAKE_DIRECTORY "${VIT_LIBSODIUM_EXTRACT_DIR}")
    execute_process(
        COMMAND ${CMAKE_COMMAND} -E tar xzf "${VIT_LIBSODIUM_ARCHIVE}"
        WORKING_DIRECTORY "${VIT_LIBSODIUM_EXTRACT_DIR}"
        RESULT_VARIABLE VIT_LIBSODIUM_EXTRACT_RC)
    if (NOT VIT_LIBSODIUM_EXTRACT_RC EQUAL 0)
        message(FATAL_ERROR "VitApp: failed to extract libsodium archive (cmake -E tar), code ${VIT_LIBSODIUM_EXTRACT_RC}")
    endif()
endif()
if (NOT EXISTS "${VIT_LIBSODIUM_TREE}/src/libsodium/include/sodium.h")
    message(FATAL_ERROR "VitApp: libsodium extract missing expected tree: ${VIT_LIBSODIUM_TREE}")
endif()
set(VIT_LIBSODIUM_ROOT "${VIT_LIBSODIUM_TREE}" CACHE INTERNAL "")
add_subdirectory("${CMAKE_CURRENT_SOURCE_DIR}/cmake/libsodium_bundle" "${CMAKE_BINARY_DIR}/libsodium_bundle")

juce_add_console_app(VitApp
    PRODUCT_NAME "VitHeadlessServer"
    COMPANY_NAME "Vit-DAW")

if (WIN32)
    # VitApp is a console process, but it also hosts native plug-in windows.
    # Declare Per-Monitor V2 awareness in the executable manifest so Windows
    # applies the correct DPI context before JUCE creates its hidden message
    # window. Setting awareness later at runtime is too late for that window.
    target_sources(VitApp PRIVATE
        Resources/Windows/VitApp.manifest)
endif()

juce_generate_juce_header(VitApp)
target_compile_features(VitApp PRIVATE cxx_std_20)

set(VITAPP_SOURCE_FILES
    Source/Main.cpp
    Source/Bridge/VitSemanticBridge.h
    Source/Core/VitAIGCJobRuntime.cpp
    Source/Core/VitAIGCJobRuntime.h
    Source/Core/VitAudioInjectorNode.cpp
    Source/Core/VitAudioInjectorNode.h
    Source/Core/VitAsyncGhostPolicy.cpp
    Source/Core/VitAsyncGhostPolicy.h
    Source/Core/VitBridgeNode.cpp
    Source/Core/VitBridgeNode.h
    Source/Core/VitEngineDevice.cpp
    Source/Core/VitEngineDevice.h
    Source/Core/VitEngineBehaviour.cpp
    Source/Core/VitEngineBehaviour.h
    Source/Core/VitClipRouteRegistry.cpp
    Source/Core/VitClipRouteRegistry.h
    Source/Core/VitDagChecker.cpp
    Source/Core/VitDagChecker.h
    Source/Core/VitGlobalProjectConfig.cpp
    Source/Core/VitGlobalProjectConfig.h
    Source/Core/VitGraphValidator.cpp
    Source/Core/VitGraphValidator.h
    Source/Core/VitKernelUtils.cpp
    Source/Core/VitKernelUtils.h
    Source/Core/VitParallelMergePlanner.cpp
    Source/Core/VitParallelMergePlanner.h
    Source/Core/VitGraphRevisionLedger.cpp
    Source/Core/VitGraphRevisionLedger.h
    Source/Core/VitGraphSwapCoordinator.cpp
    Source/Core/VitGraphSwapCoordinator.h
    Source/Core/VitDeltaProbe.cpp
    Source/Core/VitDeltaProbe.h
    Source/Core/VitEncryptionCore.cpp
    Source/Core/VitEncryptionCore.h
    Source/Core/VitMediaPoolManager.cpp
    Source/Core/VitMediaPoolManager.h
    Source/Core/VitNodeCapabilityManifest.cpp
    Source/Core/VitNodeCapabilityManifest.h
    Source/Core/VitNodeRegistry.cpp
    Source/Core/VitNodeRegistry.h
    Source/Core/VitNodeProfiler.cpp
    Source/Core/VitNodeProfiler.h
    Source/Core/VitPaths.h
    Source/Core/VitProbeNode.cpp
    Source/Core/VitProbeNode.h
    Source/Core/VitProjectHealthCheck.cpp
    Source/Core/VitProjectHealthCheck.h
    Source/Core/VitControlShell.cpp
    Source/Core/VitControlShell.h
    Source/Core/VitConnectorPluginSpec.cpp
    Source/Core/VitConnectorPluginSpec.h
    Source/Core/VitGraphTrace.cpp
    Source/Core/VitGraphTrace.h
    Source/Core/VitMacroNode.cpp
    Source/Core/VitMacroNode.h
    Source/Core/VitParamBinding.cpp
    Source/Core/VitParamBinding.h
    Source/Core/VitParamLinkGraph.cpp
    Source/Core/VitParamLinkGraph.h
    Source/Core/VitParamSurface.cpp
    Source/Core/VitParamSurface.h
    Source/Core/VitPluginGrabber.cpp
    Source/Core/VitPluginGrabber.h
    Source/Core/VitPluginTemplateRegistry.cpp
    Source/Core/VitPluginTemplateRegistry.h
    Source/Core/VitZoneBufferAdapter.cpp
    Source/Core/VitZoneBufferAdapter.h
    Source/Core/VitProjectFile.cpp
    Source/Core/VitProjectFile.h
    Source/Core/VitPortablePropertyStorage.cpp
    Source/Core/VitPortablePropertyStorage.h
    Source/Core/VitPluginUIBehaviour.cpp
    Source/Core/VitPluginUIBehaviour.h
    Source/Core/VitTakeHistoryStack.cpp
    Source/Core/VitTakeHistoryStack.h
    Source/Core/VitWarpBridgeNode.cpp
    Source/Core/VitWarpBridgeNode.h
    Source/Service/VitHeadlessService.cpp
    Source/Service/VitHeadlessService.h
    Source/Service/CommandDispatcher.cpp
    Source/Service/CommandDispatcher.h
    Source/Service/VspKernelReference.cpp
    Source/Service/VspKernelReference.h
    Source/Service/ClipService.cpp
    Source/Service/ClipService.h
    Source/Service/GeneratedAssetService.cpp
    Source/Service/GeneratedAssetService.h
    Source/Service/ImportService.cpp
    Source/Service/ImportService.h
    Source/Service/JobEventService.cpp
    Source/Service/JobEventService.h
    Source/Service/MidiService.cpp
    Source/Service/MidiService.h
    Source/Service/ProjectAudioSettingsService.cpp
    Source/Service/ProjectAudioSettingsService.h
      Source/Service/ProjectMarkerService.cpp
      Source/Service/ProjectMarkerService.h
      Source/Service/ProjectService.cpp
      Source/Service/ProjectService.h
      Source/Service/TrackGroupService.cpp
      Source/Service/TrackGroupService.h
      Source/Service/PluginRackControlService.cpp
    Source/Service/PluginRackControlService.h
    Source/Service/TrackService.cpp
    Source/Service/TrackService.h
    Source/Service/TransportAudioService.cpp
    Source/Service/TransportAudioService.h
    Source/Service/CompressorDualTapEvidence.cpp
    Source/Service/CompressorDualTapEvidence.h
    Source/Service/VitProductionCoordinator.cpp
    Source/Service/VitProductionCoordinator.h
    Source/Service/SharedMemoryTester.cpp
    Source/Service/SharedMemoryTester.h
    Source/Service/AudioFeatureService.cpp
    Source/Service/AudioFeatureService.h
    Source/Service/AudioFeatureTypes.cpp
    Source/Service/AudioFeatureTypes.h
    Source/Service/L3AcousticAnalyzer.cpp
    Source/Service/L3AcousticAnalyzer.h
    Source/Service/TiledSpectrogramBaker.cpp
    Source/Service/TiledSpectrogramBaker.h
    Source/Service/WaveformEnvelopeBaker.cpp
    Source/Service/WaveformEnvelopeBaker.h
    Source/Service/ZmqLogger.cpp
    Source/Service/ZmqLogger.h
    Source/Service/ZmqGateway.cpp
    Source/Service/ZmqGateway.h)

target_sources(VitApp PRIVATE ${VITAPP_SOURCE_FILES})
source_group(TREE "${CMAKE_CURRENT_SOURCE_DIR}" FILES ${VITAPP_SOURCE_FILES})

target_include_directories(VitApp PRIVATE
    "${CMAKE_CURRENT_SOURCE_DIR}/Source")

target_compile_definitions(VitApp PRIVATE
    SODIUM_STATIC
    JUCE_APPLICATION_NAME_STRING="$<TARGET_PROPERTY:VitApp,JUCE_PRODUCT_NAME>"
    JUCE_APPLICATION_VERSION_STRING="$<TARGET_PROPERTY:VitApp,JUCE_VERSION>"
    JUCE_STRICT_REFCOUNTEDPOINTER=1
    JUCE_USE_CURL=0
    JUCE_WEB_BROWSER=0
    JUCE_MODAL_LOOPS_PERMITTED=1
    JUCE_PLUGINHOST_VST3=1
    TRACKTION_ENABLE_TIMESTRETCH_SOUNDTOUCH=1
    VIT_DEFAULT_PROJECT_XML_RELATIVE="${VIT_DEFAULT_PROJECT_XML_RELATIVE}")

target_link_libraries(VitApp
    PRIVATE
        tracktion::tracktion_core
        tracktion::tracktion_engine
        tracktion::tracktion_graph
        juce::juce_audio_devices
        juce::juce_audio_utils
        sodium
        ${VIT_ZMQ_TARGETS}
    PUBLIC
        juce::juce_recommended_config_flags
        juce::juce_recommended_lto_flags
        juce::juce_recommended_warning_flags)

if(MSVC)
    target_compile_options(VitApp PRIVATE /W4 /permissive-)
else()
    target_compile_options(VitApp PRIVATE -Wall -Wextra -Wpedantic)
endif()

if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    target_link_libraries(VitApp PRIVATE atomic)
endif()
