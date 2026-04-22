#include <windows.h>
#include <algorithm>
#include <cwctype>
#include <string>
#include <vector>

static std::wstring GetExeDir() {
    wchar_t path[MAX_PATH] = {0};
    GetModuleFileNameW(nullptr, path, MAX_PATH);
    std::wstring full(path);
    size_t pos = full.find_last_of(L"\\/");
    return (pos == std::wstring::npos) ? L"." : full.substr(0, pos);
}

static bool FileExists(const std::wstring& p) {
    DWORD attrs = GetFileAttributesW(p.c_str());
    return attrs != INVALID_FILE_ATTRIBUTES && !(attrs & FILE_ATTRIBUTE_DIRECTORY);
}

static std::wstring ToLower(std::wstring s) {
    std::transform(s.begin(), s.end(), s.begin(), [](wchar_t c) { return (wchar_t)towlower(c); });
    return s;
}

static std::wstring GetFileName(const std::wstring& p) {
    size_t pos = p.find_last_of(L"\\/");
    if (pos == std::wstring::npos) {
        return p;
    }
    return p.substr(pos + 1);
}

// Prefer Vit_DAW.exe (Godot export name), then any "Vit DAW*.exe" / "Vit_DAW*.exe", excluding this launcher.
static std::wstring ResolveUiExe(const std::wstring& baseDir, const std::wstring& selfPath) {
    const std::wstring fixed = baseDir + L"\\Vit_DAW.exe";
    if (FileExists(fixed)) {
        std::wstring fixedLower = ToLower(GetFileName(fixed));
        std::wstring selfLower = ToLower(GetFileName(selfPath));
        if (fixedLower != selfLower) {
            return fixed;
        }
    }
    std::wstring selfName = ToLower(GetFileName(selfPath));
    std::wstring pattern = baseDir + L"\\*.exe";
    WIN32_FIND_DATAW fd{};
    HANDLE h = FindFirstFileW(pattern.c_str(), &fd);
    if (h == INVALID_HANDLE_VALUE) {
        return L"";
    }
    std::wstring best;
    size_t bestRank = 0;
    do {
        if (fd.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) {
            continue;
        }
        std::wstring name = fd.cFileName;
        std::wstring lower = ToLower(name);
        if (lower == selfName || lower.find(L"launcher") != std::wstring::npos) {
            continue;
        }
        if (lower.rfind(L"vit daw", 0) != 0 && lower.rfind(L"vit_daw", 0) != 0) {
            continue;
        }
        std::wstring full = baseDir + L"\\" + name;
        size_t rank = name.length();
        if (lower.find(L" v") != std::wstring::npos) {
            rank += 1000;
        }
        if (rank > bestRank) {
            bestRank = rank;
            best = full;
        }
    } while (FindNextFileW(h, &fd));
    FindClose(h);
    return best;
}

int APIENTRY WinMain(HINSTANCE hInstance, HINSTANCE hPrevInstance, LPSTR lpCmdLine, int nCmdShow) {
    (void)hInstance;
    (void)hPrevInstance;
    (void)lpCmdLine;
    (void)nCmdShow;

    std::wstring baseDir = GetExeDir();
    wchar_t selfPathBuf[MAX_PATH] = {0};
    GetModuleFileNameW(nullptr, selfPathBuf, MAX_PATH);
    std::wstring selfPath(selfPathBuf);
    std::wstring uiExe = ResolveUiExe(baseDir, selfPath);
    std::wstring runtimeDir = baseDir + L"\\runtime";
    std::wstring kernelExe = runtimeDir + L"\\VitApp.exe";
    std::wstring bridgeProd = runtimeDir + L"\\bridge_prod.py";
    std::wstring bridgeCore = runtimeDir + L"\\bridge_core.py";
    std::wstring bridgeScript = FileExists(bridgeProd) ? bridgeProd : bridgeCore;
    std::wstring bundledPy = baseDir + L"\\python_embed\\python.exe";

    if (!FileExists(uiExe) || !FileExists(kernelExe)) {
        MessageBoxW(nullptr, L"Missing exported UI (.exe) or runtime\\VitApp.exe", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        return 1;
    }
    if (!FileExists(bridgeScript)) {
        MessageBoxW(nullptr, L"Missing runtime\\bridge_prod.py or runtime\\bridge_core.py", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        return 1;
    }

    std::wstring logPath = runtimeDir + L"\\bridge_last.log";
    SetEnvironmentVariableW(L"VIT_BRIDGE_LAST_LOG_PATH", logPath.c_str());
    SetEnvironmentVariableW(L"PYTHONPATH", runtimeDir.c_str());
    std::wstring cfgPath = runtimeDir + L"\\bridge_prod.config.json";
    if (FileExists(cfgPath)) {
        SetEnvironmentVariableW(L"VIT_BRIDGE_CONFIG", cfgPath.c_str());
    }

    HANDLE hJob = CreateJobObject(NULL, NULL);
    if (hJob == NULL) return 1;

    JOBOBJECT_EXTENDED_LIMIT_INFORMATION jeli = { 0 };
    jeli.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    SetInformationJobObject(hJob, JobObjectExtendedLimitInformation, &jeli, sizeof(jeli));

    STARTUPINFOW siKernel = { sizeof(siKernel) };
    PROCESS_INFORMATION piKernel = {};
    std::wstring kernelCmd = L"\"" + kernelExe + L"\"";
    std::vector<wchar_t> kernelBuf(kernelCmd.begin(), kernelCmd.end());
    kernelBuf.push_back(L'\0');
    if (!CreateProcessW(nullptr, kernelBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, runtimeDir.c_str(), &siKernel, &piKernel)) {
        MessageBoxW(nullptr, L"Failed to start runtime\\VitApp.exe", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piKernel.hProcess);

    Sleep(500);

    STARTUPINFOW siBridge = { sizeof(siBridge) };
    PROCESS_INFORMATION piBridge = {};
    std::wstring bridgeCmd;
    if (FileExists(bundledPy)) {
        bridgeCmd = L"\"" + bundledPy + L"\" \"" + bridgeScript + L"\"";
    } else {
        bridgeCmd = L"py -3 \"" + bridgeScript + L"\"";
    }
    std::vector<wchar_t> bridgeBuf(bridgeCmd.begin(), bridgeCmd.end());
    bridgeBuf.push_back(L'\0');
    BOOL okBridge = CreateProcessW(nullptr, bridgeBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, runtimeDir.c_str(), &siBridge, &piBridge);
    if (!okBridge && !FileExists(bundledPy)) {
        std::wstring fb = L"python \"" + bridgeScript + L"\"";
        std::vector<wchar_t> fbBuf(fb.begin(), fb.end());
        fbBuf.push_back(L'\0');
        okBridge = CreateProcessW(nullptr, fbBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, runtimeDir.c_str(), &siBridge, &piBridge);
    }
    if (!okBridge) {
        MessageBoxW(nullptr, L"Failed to start Python bridge. Ship python_embed\\python.exe or install Python + pyzmq.", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piBridge.hProcess);

    Sleep(700);

    SetEnvironmentVariableW(L"VIT_FROM_LAUNCHER", L"1");

    STARTUPINFOW siGodot = { sizeof(siGodot) };
    PROCESS_INFORMATION piGodot = {};
    std::wstring godotCmd = L"\"" + uiExe + L"\"";
    std::vector<wchar_t> godotBuf(godotCmd.begin(), godotCmd.end());
    godotBuf.push_back(L'\0');
    if (!CreateProcessW(nullptr, godotBuf.data(), nullptr, nullptr, FALSE, 0, nullptr, baseDir.c_str(), &siGodot, &piGodot)) {
        MessageBoxW(nullptr, L"Failed to start Vit_DAW.exe", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(piBridge.hProcess);
        CloseHandle(piBridge.hThread);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piGodot.hProcess);

    WaitForSingleObject(piGodot.hProcess, INFINITE);

    CloseHandle(piGodot.hProcess);
    CloseHandle(piGodot.hThread);
    CloseHandle(piBridge.hProcess);
    CloseHandle(piBridge.hThread);
    CloseHandle(piKernel.hProcess);
    CloseHandle(piKernel.hThread);
    CloseHandle(hJob);

    return 0;
}
