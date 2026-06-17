cmake_minimum_required(VERSION 3.15)
project(VitDAW_Launcher)

# 设置 C++ 标准
set(CMAKE_CXX_STANDARD 17)

# 关键点：WIN32 标志告诉链接器这是窗口程序，不弹控制台
add_executable(Vit_DAW_Launcher WIN32 launcher.cpp launcher.rc)

# 链接 Windows 基础库
target_link_libraries(Vit_DAW_Launcher kernel32 user32 ws2_32)
