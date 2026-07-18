#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <algorithm>
#include <cwctype>
#include <string>
#include <vector>

#pragma comment(lib, "Ws2_32.lib")

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

static bool DirExists(const std::wstring& p) {
    DWORD attrs = GetFileAttributesW(p.c_str());
    return attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY);
}

static void EnsureDir(const std::wstring& p) {
    if (!DirExists(p)) {
        CreateDirectoryW(p.c_str(), nullptr);
    }
}

static std::wstring GetEnvVar(const wchar_t* name) {
    DWORD needed = GetEnvironmentVariableW(name, nullptr, 0);
    if (needed == 0) {
        return L"";
    }
    std::vector<wchar_t> buf(needed);
    DWORD got = GetEnvironmentVariableW(name, buf.data(), needed);
    if (got == 0 || got >= needed) {
        return L"";
    }
    return std::wstring(buf.data(), got);
}

static std::wstring ResolveDataDir(const std::wstring& fallbackBaseDir) {
    std::wstring localAppData = GetEnvVar(L"LOCALAPPDATA");
    if (!localAppData.empty()) {
        return localAppData + L"\\Vit-DAW";
    }
    return fallbackBaseDir;
}

static bool ProcessIsRunning(HANDLE hProcess) {
    return WaitForSingleObject(hProcess, 0) == WAIT_TIMEOUT;
}

static std::wstring ErrorWithLog(const std::wstring& message, const std::wstring& logPath) {
    return message + L"\n\nRuntime log:\n" + logPath;
}

static bool ContainsJsonField(const std::string& json, const std::string& field, const std::string& value) {
    std::string compact;
    compact.reserve(json.size());
    for (char c : json) {
        if (c != ' ' && c != '\t' && c != '\r' && c != '\n') {
            compact.push_back(c);
        }
    }
    return compact.find("\"" + field + "\":\"" + value + "\"") != std::string::npos;
}

static bool WaitForBridgeReady(HANDLE kernelProcess, HANDLE bridgeProcess, DWORD timeoutMs) {
    WSADATA wsa{};
    if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) {
        return false;
    }

    SOCKET sock = socket(AF_INET, SOCK_DGRAM, IPPROTO_UDP);
    if (sock == INVALID_SOCKET) {
        WSACleanup();
        return false;
    }

    DWORD recvTimeoutMs = 350;
    setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, reinterpret_cast<const char*>(&recvTimeoutMs), sizeof(recvTimeoutMs));

    sockaddr_in local{};
    local.sin_family = AF_INET;
    local.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
    local.sin_port = 0;
    if (bind(sock, reinterpret_cast<sockaddr*>(&local), sizeof(local)) == SOCKET_ERROR) {
        closesocket(sock);
        WSACleanup();
        return false;
    }

    sockaddr_in dest{};
    dest.sin_family = AF_INET;
    dest.sin_port = htons(4445);
    inet_pton(AF_INET, "127.0.0.1", &dest.sin_addr);

    const std::string payload = "{\"cmd\":\"ping\"}";
    const DWORD start = GetTickCount();
    char buf[2048];

    while (GetTickCount() - start < timeoutMs) {
        if (!ProcessIsRunning(kernelProcess) || !ProcessIsRunning(bridgeProcess)) {
            break;
        }

        sendto(sock, payload.c_str(), static_cast<int>(payload.size()), 0, reinterpret_cast<sockaddr*>(&dest), sizeof(dest));

        sockaddr_in from{};
        int fromLen = sizeof(from);
        int got = recvfrom(sock, buf, static_cast<int>(sizeof(buf) - 1), 0, reinterpret_cast<sockaddr*>(&from), &fromLen);
        if (got > 0) {
            buf[got] = '\0';
            std::string reply(buf, got);
            if (ContainsJsonField(reply, "status", "ok") && ContainsJsonField(reply, "message", "pong")) {
                closesocket(sock);
                WSACleanup();
                return true;
            }
        }

        Sleep(250);
    }

    closesocket(sock);
    WSACleanup();
    return false;
}

static bool WaitForVspHubReady(HANDLE kernelProcess, HANDLE hubProcess, DWORD timeoutMs) {
    WSADATA wsa{};
    if (WSAStartup(MAKEWORD(2, 2), &wsa) != 0) {
        return false;
    }

    const std::string request =
        "GET /health HTTP/1.1\r\n"
        "Host: 127.0.0.1:8787\r\n"
        "Connection: close\r\n"
        "\r\n";
    const DWORD start = GetTickCount();
    char buf[4096];

    while (GetTickCount() - start < timeoutMs) {
        if (!ProcessIsRunning(kernelProcess) || !ProcessIsRunning(hubProcess)) {
            break;
        }

        SOCKET sock = socket(AF_INET, SOCK_STREAM, IPPROTO_TCP);
        if (sock == INVALID_SOCKET) {
            Sleep(250);
            continue;
        }

        DWORD timeout = 350;
        setsockopt(sock, SOL_SOCKET, SO_SNDTIMEO, reinterpret_cast<const char*>(&timeout), sizeof(timeout));
        setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, reinterpret_cast<const char*>(&timeout), sizeof(timeout));

        sockaddr_in dest{};
        dest.sin_family = AF_INET;
        dest.sin_port = htons(8787);
        inet_pton(AF_INET, "127.0.0.1", &dest.sin_addr);

        bool ready = false;
        if (connect(sock, reinterpret_cast<sockaddr*>(&dest), sizeof(dest)) != SOCKET_ERROR) {
            send(sock, request.c_str(), static_cast<int>(request.size()), 0);
            int got = recv(sock, buf, static_cast<int>(sizeof(buf) - 1), 0);
            if (got > 0) {
                buf[got] = '\0';
                std::string reply(buf, got);
                ready = ContainsJsonField(reply, "status", "ok") && ContainsJsonField(reply, "service", "VspHub");
            }
        }
        closesocket(sock);
        if (ready) {
            WSACleanup();
            return true;
        }

        Sleep(250);
    }

    WSACleanup();
    return false;
}

// Layered release layout: launcher at root, Godot UI under ui\.
static std::wstring ResolveUiExe(const std::wstring& uiDir) {
    const std::wstring current = uiDir + L"\\Vit DAW v0.7.exe";
    if (FileExists(current)) {
        return current;
    }
    const std::wstring fixed = uiDir + L"\\Vit DAW v0.62.exe";
    if (FileExists(fixed)) {
        return fixed;
    }
    std::wstring pattern = uiDir + L"\\*.exe";
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
        if (lower.find(L"launcher") != std::wstring::npos || lower.find(L".console.") != std::wstring::npos) {
            continue;
        }
        if (lower.rfind(L"vit daw", 0) != 0) {
            continue;
        }
        std::wstring full = uiDir + L"\\" + name;
        size_t rank = name.length();
        if (lower.find(L" v0.7") != std::wstring::npos) {
            rank += 3000;
        } else if (lower.find(L" v0.62") != std::wstring::npos) {
            rank += 2000;
        } else if (lower.find(L" v") != std::wstring::npos) {
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
    std::wstring uiDir = baseDir + L"\\ui";
    std::wstring kernelDir = baseDir + L"\\kernel";
    std::wstring hubDir = baseDir + L"\\hub";
    std::wstring bridgeDir = baseDir + L"\\bridge";
    std::wstring agentDir = baseDir + L"\\agent";
    std::wstring dataDir = ResolveDataDir(baseDir);
    std::wstring logsDir = dataDir + L"\\Logs";
    std::wstring uiExe = ResolveUiExe(uiDir);
    std::wstring kernelExe = kernelDir + L"\\VitApp.exe";
    std::wstring hubExe = hubDir + L"\\VspHub.exe";
    std::wstring agentExe = agentDir + L"\\VitAgent.exe";
    std::wstring bridgeProd = bridgeDir + L"\\bridge_prod.py";
    std::wstring bridgeCore = bridgeDir + L"\\bridge_core.py";
    std::wstring bridgeScript = FileExists(bridgeProd) ? bridgeProd : bridgeCore;
    std::wstring bundledPy = baseDir + L"\\python_embed\\python.exe";
    std::wstring logPath = logsDir + L"\\bridge_last.log";
    std::wstring hubLogPath = logsDir + L"\\vsp_hub_last.log";
    std::wstring agentLogPath = logsDir + L"\\agent_last.log";
    bool useAgent = FileExists(agentExe);

    if (!FileExists(uiExe) || !FileExists(kernelExe) || !FileExists(hubExe)) {
        MessageBoxW(nullptr, L"Missing ui\\Vit DAW*.exe, kernel\\VitApp.exe, or hub\\VspHub.exe", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        return 1;
    }
    if (!useAgent && !FileExists(bridgeScript)) {
        MessageBoxW(nullptr, L"Missing agent\\VitAgent.exe or bridge\\bridge_prod.py/bridge_core.py", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        return 1;
    }

    EnsureDir(dataDir);
    EnsureDir(logsDir);
    SetEnvironmentVariableW(L"VIT_BRIDGE_LAST_LOG_PATH", logPath.c_str());
    SetEnvironmentVariableW(L"VIT_VSP_HUB_LAST_LOG_PATH", hubLogPath.c_str());
    SetEnvironmentVariableW(L"VIT_AGENT_LAST_LOG_PATH", agentLogPath.c_str());
    SetEnvironmentVariableW(L"VIT_AGENT_VSP_HUB_URL", L"http://127.0.0.1:8787/vsp");
    SetEnvironmentVariableW(L"VIT_AGENT_VSP_HUB_REQUIRED", L"1");
    SetEnvironmentVariableW(L"PYTHONPATH", bridgeDir.c_str());
    std::wstring cfgPath = bridgeDir + L"\\bridge_prod.config.json";
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
    if (!CreateProcessW(nullptr, kernelBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, dataDir.c_str(), &siKernel, &piKernel)) {
        MessageBoxW(nullptr, L"Failed to start kernel\\VitApp.exe", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piKernel.hProcess);

    Sleep(250);
    if (!ProcessIsRunning(piKernel.hProcess)) {
        MessageBoxW(nullptr, L"kernel\\VitApp.exe exited during startup", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(hJob);
        return 1;
    }

    STARTUPINFOW siHub = { sizeof(siHub) };
    PROCESS_INFORMATION piHub = {};
    std::wstring hubCmd = L"\"" + hubExe + L"\"";
    std::vector<wchar_t> hubBuf(hubCmd.begin(), hubCmd.end());
    hubBuf.push_back(L'\0');
    if (!CreateProcessW(nullptr, hubBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, hubDir.c_str(), &siHub, &piHub)) {
        MessageBoxW(nullptr, ErrorWithLog(L"Failed to start hub\\VspHub.exe.", hubLogPath).c_str(), L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piHub.hProcess);

    if (!WaitForVspHubReady(piKernel.hProcess, piHub.hProcess, 15000)) {
        MessageBoxW(nullptr, ErrorWithLog(L"VspHub did not become ready within 15 seconds. Expected http://127.0.0.1:8787/health to return ok.", hubLogPath).c_str(), L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(piHub.hProcess);
        CloseHandle(piHub.hThread);
        CloseHandle(hJob);
        return 1;
    }

    STARTUPINFOW siBridge = { sizeof(siBridge) };
    PROCESS_INFORMATION piBridge = {};
    std::wstring bridgeCmd;
    std::wstring bridgeWorkDir = bridgeDir;
    if (useAgent) {
        bridgeCmd = L"\"" + agentExe + L"\"";
        bridgeWorkDir = agentDir;
    } else if (FileExists(bundledPy)) {
        bridgeCmd = L"\"" + bundledPy + L"\" \"" + bridgeScript + L"\"";
    } else {
        bridgeCmd = L"py -3 \"" + bridgeScript + L"\"";
    }
    std::vector<wchar_t> bridgeBuf(bridgeCmd.begin(), bridgeCmd.end());
    bridgeBuf.push_back(L'\0');
    BOOL okBridge = CreateProcessW(nullptr, bridgeBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, bridgeWorkDir.c_str(), &siBridge, &piBridge);
    if (!okBridge && !useAgent && !FileExists(bundledPy)) {
        std::wstring fb = L"python \"" + bridgeScript + L"\"";
        std::vector<wchar_t> fbBuf(fb.begin(), fb.end());
        fbBuf.push_back(L'\0');
        okBridge = CreateProcessW(nullptr, fbBuf.data(), nullptr, nullptr, FALSE, CREATE_NO_WINDOW, nullptr, bridgeDir.c_str(), &siBridge, &piBridge);
    }
    if (!okBridge) {
        MessageBoxW(nullptr, ErrorWithLog(useAgent ? L"Failed to start agent\\VitAgent.exe." : L"Failed to start Python bridge. Ship python_embed\\python.exe or install Python + pyzmq.", useAgent ? agentLogPath : logPath).c_str(), L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(piHub.hProcess);
        CloseHandle(piHub.hThread);
        CloseHandle(hJob);
        return 1;
    }
    AssignProcessToJobObject(hJob, piBridge.hProcess);

    if (!WaitForBridgeReady(piKernel.hProcess, piBridge.hProcess, 60000)) {
        MessageBoxW(nullptr, ErrorWithLog(L"Agent/bridge did not become ready within 60 seconds. Expected ping over UDP 127.0.0.1:4445 to return ok/pong.", useAgent ? agentLogPath : logPath).c_str(), L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(piHub.hProcess);
        CloseHandle(piHub.hThread);
        CloseHandle(piBridge.hProcess);
        CloseHandle(piBridge.hThread);
        CloseHandle(hJob);
        return 1;
    }

    SetEnvironmentVariableW(L"VIT_FROM_LAUNCHER", L"1");

    STARTUPINFOW siGodot = { sizeof(siGodot) };
    PROCESS_INFORMATION piGodot = {};
    std::wstring godotCmd = L"\"" + uiExe + L"\"";
    std::vector<wchar_t> godotBuf(godotCmd.begin(), godotCmd.end());
    godotBuf.push_back(L'\0');
    if (!CreateProcessW(nullptr, godotBuf.data(), nullptr, nullptr, FALSE, 0, nullptr, uiDir.c_str(), &siGodot, &piGodot)) {
        MessageBoxW(nullptr, L"Failed to start selected UI executable under ui\\", L"Vit-DAW Launcher Error", MB_OK | MB_ICONERROR);
        CloseHandle(piKernel.hProcess);
        CloseHandle(piKernel.hThread);
        CloseHandle(piHub.hProcess);
        CloseHandle(piHub.hThread);
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
    CloseHandle(piHub.hProcess);
    CloseHandle(piHub.hThread);
    CloseHandle(piKernel.hProcess);
    CloseHandle(piKernel.hThread);
    CloseHandle(hJob);

    return 0;
}
