# MIX-SENDRETURN-DESIGN-1 底座实证探针构建（取证工件，独立于 VitApp 主构建）
cmake_minimum_required(VERSION 3.22)
project(VitAuxMultiReturnProbe VERSION 1.0.0 LANGUAGES C CXX)

set(TE_ADD_EXAMPLES OFF CACHE BOOL "" FORCE)
# tracktion_engine 是 submodule：worktree 内仅占位。主仓主树 submodule 已检出
# b43974980c480d4d3f378fe84f640ec9b4ad4300，与 worktree 基线(54283662)记录的指针一致
# 且工作树干净——以主树 submodule 为只读编译源，行为等价于 worktree 内初始化 submodule。
set(TRACKTION_ENGINE_DIR "D:/Vit_DAW/tracktion_engine" CACHE PATH "tracktion_engine sources (read-only)")
if(NOT EXISTS "${TRACKTION_ENGINE_DIR}/CMakeLists.txt")
    message(FATAL_ERROR "tracktion_engine not found at: ${TRACKTION_ENGINE_DIR}")
endif()
add_subdirectory("${TRACKTION_ENGINE_DIR}" "${CMAKE_CURRENT_BINARY_DIR}/tracktion_engine")

juce_add_console_app(VitAuxMultiReturnProbe PRODUCT_NAME "VitAuxMultiReturnProbe")
juce_generate_juce_header(VitAuxMultiReturnProbe)
target_sources(VitAuxMultiReturnProbe PRIVATE AuxMultiReturnProbe.cpp)
target_compile_features(VitAuxMultiReturnProbe PRIVATE cxx_std_20)
target_compile_options(VitAuxMultiReturnProbe PRIVATE /utf-8)
target_compile_definitions(VitAuxMultiReturnProbe PRIVATE JUCE_USE_CURL=0 JUCE_WEB_BROWSER=0)
target_link_libraries(VitAuxMultiReturnProbe PRIVATE
    tracktion::tracktion_core
    tracktion::tracktion_engine
    tracktion::tracktion_graph
    juce::juce_audio_utils
    juce::juce_recommended_config_flags)
