cmake_minimum_required(VERSION 3.25)

# GenShell — 生成插件壳体（GENSHELL-CPP-1 骨架）
#
# VST3 SDK 直实现（不经 JUCE plugin 包装，件一 §7）：SDK 来源=仓库 tracktion_engine
# 子模块内 JUCE 内嵌 VST3_SDK（与 PluginProbe native-host 同源；Mac worktree 需用主仓
# checkout——worktree 内子模块为空目录是已知工件）。该 SDK 为 JUCE 裁剪版：缺
# public.sdk/source/main（模块工厂）与 vstgui4，分别由本工程手写工厂与
# thirdparty/vstgui4 vendoring 补齐。
#
# 用法（mac）：
#   cmake -B build -DGENSHELL_VST3_SDK_ROOT=<repo>/tracktion_engine/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK
#   cmake --build build --config Release
#   # 产物：build/GenShell.vst3（部署：拷至 ~/Library/Audio/Plug-Ins/VST3/）
#   # 单测：./build/tests/genshell_selftest build/GenShell.vst3

project(GenShell VERSION 1.0.0 LANGUAGES C CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_OSX_DEPLOYMENT_TARGET 10.15 CACHE STRING "")

# ---------------------------------------------------------------- SDK 定位
get_filename_component(GENSHELL_ROOT "${CMAKE_CURRENT_LIST_DIR}" ABSOLUTE)
set(GENSHELL_REPO_ROOT "${GENSHELL_ROOT}/.." CACHE PATH "Vit-DAW 仓库根")
set(GENSHELL_VST3_SDK_ROOT
    "${GENSHELL_REPO_ROOT}/tracktion_engine/modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK"
    CACHE PATH "VST3 SDK 根（默认=tracktion_engine 子模块内 JUCE 内嵌 SDK）")

if(NOT EXISTS "${GENSHELL_VST3_SDK_ROOT}/pluginterfaces/vst/ivstaudioprocessor.h")
    message(FATAL_ERROR "VST3 SDK not found at ${GENSHELL_VST3_SDK_ROOT} — pass -DGENSHELL_VST3_SDK_ROOT or -DGENSHELL_REPO_ROOT")
endif()
message(STATUS "GenShell: VST3 SDK = ${GENSHELL_VST3_SDK_ROOT}")

# ---------------------------------------------------------------- VSTGUI
set(VSTGUI_STANDALONE OFF CACHE BOOL "" FORCE)
set(VSTGUI_STANDALONE_EXAMPLES OFF CACHE BOOL "" FORCE)
set(VSTGUI_TOOLS OFF CACHE BOOL "" FORCE)
set(VSTGUI_UISCRIPTING OFF CACHE BOOL "" FORCE)
set(VSTGUI_DISABLE_UNITTESTS ON CACHE BOOL "" FORCE)
add_subdirectory(thirdparty/vstgui4/vstgui)

# ---------------------------------------------------------------- SDK 公共源
set(GENSHELL_SDK_ROOT "${GENSHELL_VST3_SDK_ROOT}")
set(GENSHELL_SDK_SOURCES
    "${GENSHELL_SDK_ROOT}/base/source/baseiids.cpp"
    "${GENSHELL_SDK_ROOT}/base/thread/source/flock.cpp"
    "${GENSHELL_SDK_ROOT}/pluginterfaces/base/funknown.cpp"
    "${GENSHELL_SDK_ROOT}/pluginterfaces/base/coreiids.cpp"
    "${GENSHELL_SDK_ROOT}/base/source/fobject.cpp"
    "${GENSHELL_SDK_ROOT}/base/source/fstreamer.cpp"
    "${GENSHELL_SDK_ROOT}/base/source/fstring.cpp"
    "${GENSHELL_SDK_ROOT}/base/source/updatehandler.cpp"
    "${GENSHELL_SDK_ROOT}/pluginterfaces/base/ustring.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/common/pluginview.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/common/memorystream.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstbus.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstcomponent.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstcomponentbase.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vsteditcontroller.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstinitiids.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstparameters.cpp"
    "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/vstpresetfile.cpp"
)
add_library(genshell_vst3sdk STATIC ${GENSHELL_SDK_SOURCES})
target_include_directories(genshell_vst3sdk PUBLIC
    "${GENSHELL_SDK_ROOT}"
    "${GENSHELL_SDK_ROOT}/base/thread/include"
)
target_compile_definitions(genshell_vst3sdk PUBLIC SMTG_LIB_TYPE_STATIC=1)
if(APPLE)
    target_compile_options(genshell_vst3sdk PRIVATE -Wno-unused-parameter)
endif()

# ---------------------------------------------------------------- 插件 bundle
set(GENSHELL_PLUGIN_SOURCES
    source/genshell_cids.h
    source/genshell_convert.h
    source/genshell_convert.cpp
    source/genshell_state.h
    source/genshell_state.cpp
    source/genshell_params.h
    source/genshell_params.cpp
    source/genshell_processor.h
    source/genshell_processor.cpp
    source/genshell_controller.h
    source/genshell_controller.cpp
    source/genshell_editor.h
    source/genshell_editor.cpp
    source/genshell_factory.cpp
    source/genshell_guiiids.cpp
)
add_library(GenShell MODULE ${GENSHELL_PLUGIN_SOURCES})

set_target_properties(GenShell PROPERTIES
    BUNDLE TRUE
    BUNDLE_EXTENSION "vst3"
    MACOSX_BUNDLE TRUE
    MACOSX_BUNDLE_BUNDLE_VERSION 1.0.0
    MACOSX_BUNDLE_GUI_IDENTIFIER "com.vitaudio.genshell"
    MACOSX_BUNDLE_INFO_PLIST "${CMAKE_CURRENT_LIST_DIR}/resource/Info.plist.mac"
    OUTPUT_NAME "GenShell"
    PREFIX ""
    XCODE_ATTRIBUTE_GENERATE_INFOPLIST_FILE NO
)

target_include_directories(GenShell PRIVATE
    "${CMAKE_CURRENT_LIST_DIR}/source"
    "${CMAKE_CURRENT_LIST_DIR}/thirdparty/vstgui4"   # #include "vstgui/vstgui.h"
    "${CMAKE_CURRENT_LIST_DIR}/thirdparty/json"
)
target_link_libraries(GenShell PRIVATE vstgui genshell_vst3sdk)
target_compile_definitions(GenShell PRIVATE GENSHELL_BUILD)

if(APPLE)
    enable_language(OBJCXX)
    target_compile_options(GenShell PRIVATE -fvisibility=hidden)
endif()

# ---------------------------------------------------------------- 壳单测进程（模拟宿主）
if(APPLE)
    add_executable(genshell_selftest
        tests/genshell_selftest.cpp
        source/genshell_convert.cpp
        "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/hosting/module.cpp"
        "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/hosting/module_mac.mm"
        "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/hosting/pluginterfacesupport.cpp"
        "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/hosting/hostclasses.cpp"
        "${GENSHELL_SDK_ROOT}/public.sdk/source/common/commonstringconvert.cpp"
        "${GENSHELL_SDK_ROOT}/public.sdk/source/vst/utility/stringconvert.cpp"
    )
    set_source_files_properties("${GENSHELL_SDK_ROOT}/public.sdk/source/vst/hosting/module_mac.mm"
        PROPERTIES COMPILE_FLAGS "-fobjc-arc")
    target_include_directories(genshell_selftest PRIVATE
        "${CMAKE_CURRENT_LIST_DIR}/source"
        "${CMAKE_CURRENT_LIST_DIR}/thirdparty/json"
    )
    target_link_libraries(genshell_selftest PRIVATE genshell_vst3sdk "-framework Cocoa" "-framework CoreFoundation")
    target_compile_options(genshell_selftest PRIVATE -Wno-unused-parameter)
endif()

# ---------------------------------------------------------------- 部署（本机标准 VST3 目录）
add_custom_target(deploy_vst3
    COMMAND ${CMAKE_COMMAND} -E remove_directory "$ENV{HOME}/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
    COMMAND ${CMAKE_COMMAND} -E copy_directory
            "${CMAKE_BINARY_DIR}/GenShell.vst3"
            "$ENV{HOME}/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
    COMMAND ${CMAKE_COMMAND} -E echo "deployed to ~/Library/Audio/Plug-Ins/VST3/GenShell.vst3"
    DEPENDS GenShell
    COMMENT "Deploying GenShell.vst3 to user VST3 directory"
    VERBATIM
)
