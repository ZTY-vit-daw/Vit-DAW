@echo off
echo %DATE% %TIME% wrapper-start workdir=%CD% >> "D:\Vit_DAW_wt_g3rc1\coord\runs\G3-REOPEN-COSTCMP-1\.kernel_probe\kernel_wrapper_trace.log"
set "VITPROBE_ORIG_XML=%VIT_PROJECT_XML%"
echo %DATE% %TIME% env VIT_PROJECT_XML=%VIT_PROJECT_XML% >> "D:\Vit_DAW_wt_g3rc1\coord\runs\G3-REOPEN-COSTCMP-1\.kernel_probe\kernel_wrapper_trace.log"
"D:\Vit_DAW\Export\staging\runtime\VitApp.exe"
echo %DATE% %TIME% kernel-exitcode=%ERRORLEVEL% >> "D:\Vit_DAW_wt_g3rc1\coord\runs\G3-REOPEN-COSTCMP-1\.kernel_probe\kernel_wrapper_trace.log"
