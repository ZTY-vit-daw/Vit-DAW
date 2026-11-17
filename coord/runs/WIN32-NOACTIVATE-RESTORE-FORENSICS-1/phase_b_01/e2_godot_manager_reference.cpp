// VitShell Godot manager — implementation (spawn with direct-launch contract,
// kill-on-close job containment, exit watch, graceful close).

#include "godot_manager.h"

#include <cstdio>
#include <tlhelp32.h>
#include <vector>
#include <cwctype>
#include <algorithm>

namespace vitshell {

namespace {

std::string LastErrorText(const char *label, DWORD error) {
    char buf[256] = {};
    std::snprintf(buf, sizeof(buf), "%s failed: %lu", label, static_cast<unsigned long>(error));
    return buf;
}

struct EnumContext {
    DWORD pid;
    HWND best = nullptr;
    bool include_hidden = false;
};

BOOL CALLBACK EnumWindowsProc(HWND hwnd, LPARAM lparam) {
    auto *ctx = reinterpret_cast<EnumContext *>(lparam);
    DWORD pid = 0;
    GetWindowThreadProcessId(hwnd, &pid);
    if (pid != ctx->pid) {
        return TRUE;
    }
    if (!ctx->include_hidden && !IsWindowVisible(hwnd)) {
        return TRUE;
    }
    // Skip untitled/cloaked helper windows; the DAW main window has a title.
    wchar_t title[256] = {};
    int len = GetWindowTextW(hwnd, title, 256);
    if (len <= 0) {
        return TRUE;
    }
    if (GetWindowLongW(hwnd, GWL_EXSTYLE) & WS_EX_TOOLWINDOW) {
        return TRUE;
    }
    if (ctx->include_hidden) {
        wchar_t class_name[64] = {};
        GetClassNameW(hwnd, class_name, 64);
        if (wcscmp(class_name, L"Engine") != 0 || GetWindow(hwnd, GW_OWNER) != nullptr) return TRUE;
    }
    ctx->best = hwnd;
    return FALSE; // first match wins (main window is created first by Godot)
}

// Only the foreground-affecting startup flags are new here. Existing direct
// launch env is unchanged. Restore both child-only feature flags on every exit.
class ScopedChildEnvironment {
public:
    explicit ScopedChildEnvironment(const wchar_t *key) : key_(key) {
        const DWORD size = GetEnvironmentVariableW(key, nullptr, 0);
        present_ = size > 0;
        if (size > 0) {
            previous_.resize(size);
            previous_.resize(GetEnvironmentVariableW(key, previous_.data(), size));
        }
    }
    bool Set(const std::wstring &value) { return SetEnvironmentVariableW(key_.c_str(), value.c_str()) != FALSE; }
    const std::wstring &Previous() const { return previous_; }
    ~ScopedChildEnvironment() { SetEnvironmentVariableW(key_.c_str(), present_ ? previous_.c_str() : nullptr); }
private:
    std::wstring key_, previous_;
    bool present_ = false;
};

bool IsGuiExecutable(const std::wstring &path) {
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, 0, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;
    IMAGE_DOS_HEADER dos = {};
    DWORD got = 0, signature = 0;
    IMAGE_FILE_HEADER header = {};
    IMAGE_OPTIONAL_HEADER64 optional = {};
    bool ok = ReadFile(file, &dos, sizeof(dos), &got, nullptr) && got == sizeof(dos) && dos.e_magic == IMAGE_DOS_SIGNATURE
            && dos.e_lfanew > 0 && SetFilePointer(file, dos.e_lfanew, nullptr, FILE_BEGIN) != INVALID_SET_FILE_POINTER
            && ReadFile(file, &signature, sizeof(signature), &got, nullptr) && signature == IMAGE_NT_SIGNATURE
            && ReadFile(file, &header, sizeof(header), &got, nullptr) && header.Machine == IMAGE_FILE_MACHINE_AMD64
            && ReadFile(file, &optional, sizeof(optional), &got, nullptr) && got == sizeof(optional)
            && optional.Magic == IMAGE_NT_OPTIONAL_HDR64_MAGIC && optional.Subsystem == IMAGE_SUBSYSTEM_WINDOWS_GUI;
    CloseHandle(file);
    return ok;
}

bool FileContainsBytes(const std::wstring &path, const std::string &needle) {
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, 0, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;
    std::string carry;
    char chunk[65536];
    DWORD got = 0;
    bool found = false;
    while (ReadFile(file, chunk, sizeof(chunk), &got, nullptr) && got != 0) {
        carry.append(chunk, got);
        if (carry.find(needle) != std::string::npos) { found = true; break; }
        if (carry.size() > needle.size()) carry.erase(0, carry.size() - needle.size());
    }
    CloseHandle(file);
    return found;
}

bool ProjectHasHiddenBootMode(const std::wstring &directory) {
    const std::wstring path = directory + L"/project.godot";
    HANDLE file = CreateFileW(path.c_str(), GENERIC_READ, FILE_SHARE_READ, nullptr, OPEN_EXISTING, 0, nullptr);
    if (file == INVALID_HANDLE_VALUE) return false;
    const DWORD size = GetFileSize(file, nullptr);
    if (size == INVALID_FILE_SIZE || size > 1024 * 1024) { CloseHandle(file); return false; }
    std::string source(size, '\0');
    DWORD got = 0;
    const bool read = ReadFile(file, source.data(), size, &got, nullptr) && got == size;
    CloseHandle(file);
    if (!read) return false;
    bool display_section = false, mode_present = false;
    for (size_t begin = 0; begin < source.size();) {
        const size_t end = source.find('\n', begin);
        std::string line = source.substr(begin, end == std::string::npos ? end : end - begin);
        begin = end == std::string::npos ? source.size() : end + 1;
        const size_t comment = line.find(';');
        if (comment != std::string::npos) line.resize(comment);
        line.erase(std::remove_if(line.begin(), line.end(), [](unsigned char c) { return c == ' ' || c == '\t' || c == '\r'; }), line.end());
        if (!line.empty() && line.front() == '[') display_section = line == "[display]";
        if (display_section && line.rfind("window/size/mode.vit_shell_hidden_boot=", 0) == 0) {
            mode_present = line == "window/size/mode.vit_shell_hidden_boot=1";
        }
    }
    return mode_present;
}

std::string JsonQuoted(const std::string &value) {
    std::string out = "\"";
    for (unsigned char c : value) {
        if (c == '"' || c == '\\') { out += '\\'; out += static_cast<char>(c); }
        else if (c < 0x20) {
            char escaped[8] = {};
            std::snprintf(escaped, sizeof(escaped), "\\u%04x", c);
            out += escaped;
        } else out += static_cast<char>(c);
    }
    return out + "\"";
}

} // namespace

std::wstring GodotManager::DirectGodotExecutable(const std::wstring &configured) {
    std::wstring lower = configured;
    for (auto &c : lower) c = static_cast<wchar_t>(std::towlower(c));
    for (const auto *suffix : {L"_console.exe", L".console.exe"}) {
        const size_t length = wcslen(suffix);
        if (lower.size() >= length && lower.compare(lower.size() - length, length, suffix) == 0) {
            return configured.substr(0, configured.size() - length) + L".exe";
        }
    }
    return configured;
}

bool GodotManager::HiddenBootPreflight(const std::wstring &exe, const std::wstring &project_dir, std::string *reason) {
    if (!IsGuiExecutable(exe)) {
        if (reason) *reason = "hidden_boot_direct_gui_executable_unavailable";
        return false;
    }
    if (!FileContainsBytes(exe, "GODOT_EDITOR_CUSTOM_FEATURES") || !ProjectHasHiddenBootMode(project_dir)) {
        if (reason) *reason = "hidden_boot_editor_capability_or_project_override_missing";
        return false;
    }
    return true;
}

HWND GodotManager::FindMainWindowByPid(DWORD pid) {
    EnumContext ctx;
    ctx.pid = pid;
    EnumWindows(EnumWindowsProc, reinterpret_cast<LPARAM>(&ctx));
    return ctx.best;
}

// Collects the whole descendant-pid subtree of `root` (Toolhelp32 snapshot;
// the console wrapper spawns the real editor as a child which owns the DAW
// window, so graceful close must reach the entire tree).
static void CollectDescendantPids(DWORD root, std::vector<DWORD> &out) {
    HANDLE snap = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snap == INVALID_HANDLE_VALUE) {
        out.push_back(root);
        return;
    }
    std::vector<std::pair<DWORD, DWORD>> pairs; // (pid, parent)
    PROCESSENTRY32W entry = {};
    entry.dwSize = sizeof(entry);
    if (Process32FirstW(snap, &entry)) {
        do {
            pairs.emplace_back(entry.th32ProcessID, entry.th32ParentProcessID);
        } while (Process32NextW(snap, &entry));
    }
    CloseHandle(snap);
    out.push_back(root);
    for (size_t i = 0; i < out.size(); ++i) {
        for (const auto &pair : pairs) {
            if (pair.second == out[i]) {
                bool known = false;
                for (const DWORD existing : out) {
                    if (existing == pair.first) {
                        known = true;
                        break;
                    }
                }
                if (!known) {
                    out.push_back(pair.first);
                }
            }
        }
    }
}

static bool WindowBelongsToTree(HWND window, DWORD root) {
    if (window == nullptr || root == 0) return false;
    DWORD pid = 0;
    GetWindowThreadProcessId(window, &pid);
    std::vector<DWORD> pids;
    CollectDescendantPids(root, pids);
    for (DWORD candidate : pids) {
        if (candidate == pid) return true;
    }
    return false;
}

void GodotManager::ConfigureWindowGuard(HWND shell, bool event_hooks_ready) {
    std::lock_guard<std::mutex> lock(window_lock_);
    window_policy_.shell = shell;
    event_hooks_ready_ = event_hooks_ready;
}

bool GodotManager::OnWindowEvent(DWORD event, HWND window, LONG object_id, LONG child_id,
        DWORD event_thread, DWORD event_time) {
    const ULONGLONG received_tick = GetTickCount64();
    DWORD target_pid = 0;
    if (window != nullptr) GetWindowThreadProcessId(window, &target_pid);
    std::lock_guard<std::mutex> lock(window_lock_);
    // Record the original hook target/time before any policy branch. The
    // target is not replaced with GetForegroundWindow's later current state.
    raw_window_events_.push_back({++raw_window_seq_, event, event_thread, event_time,
            target_pid, Pid(), window, object_id, child_id, received_tick,
            launch_generation_, window_policy_.active, observed_launch_id_});
    if (raw_window_events_.size() > 128) { raw_window_events_.pop_front(); ++raw_window_dropped_; }
    if (window_policy_.active && event == EVENT_SYSTEM_FOREGROUND) {
        if (target_pid == 0) launch_foreground_unknown_ = true;
        else if (target_pid == Pid()) launch_foreground_seen_ = true;
    }
    if (window == nullptr) return false;
    if (!window_policy_.active) return false;
    const bool is_daw = WindowBelongsToTree(window, Pid());
    // Out-of-context events are observations, not a first-show barrier. Never
    // restore on CREATE/SHOW: doing so before Godot's first minimized show
    // changes its cached mode and can send it through the activating branch.
    if (hidden_boot_required_) return is_daw;
    if (event == EVENT_SYSTEM_FOREGROUND) {
        window_policy_.ObserveForeground(window, is_daw, false);
    }
    return is_daw;
}

void GodotManager::ReleaseLoadingWindowGuard(const std::string &reason) {
    bool close_unpresented = false;
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        if (window_policy_.active) {
            guard_released_tick_ = GetTickCount64();
            guard_released_seq_ = raw_window_seq_;
        }
        window_policy_.Release(reason);
        if (hidden_boot_ticket_) {
            hidden_boot_ticket_->canceled = true;
            SetEvent(hidden_boot_ticket_->done);
        }
        if (hidden_boot_required_ && hidden_boot_state_ != "prepared") {
            close_unpresented = Pid() != 0;
            hidden_boot_state_ = "canceled";
        }
        for (const auto &entry : protected_windows_) {
            DWORD current_pid = 0;
            GetWindowThreadProcessId(entry.window, &current_pid);
            if (!entry.added_noactivate || current_pid != entry.pid || !IsWindow(entry.window)) continue;
            const LONG_PTR style = GetWindowLongPtrW(entry.window, GWL_EXSTYLE);
            SetWindowLongPtrW(entry.window, GWL_EXSTYLE, style & ~static_cast<LONG_PTR>(WS_EX_NOACTIVATE));
            SetWindowPos(entry.window, nullptr, 0, 0, 0, 0,
                    SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
        }
        protected_windows_.clear();
    }
    if (close_unpresented) RequestClose(0); // no hidden orphan after a failed bootstrap
}

bool GodotManager::HiddenBootstrapRequired() const {
    std::lock_guard<std::mutex> lock(window_lock_);
    return hidden_boot_required_;
}

bool GodotManager::HiddenBootstrapPrepared() const {
    std::lock_guard<std::mutex> lock(window_lock_);
    return hidden_boot_required_ && hidden_boot_state_ == "prepared" && window_policy_.active;
}

std::shared_ptr<HiddenBootstrap> GodotManager::BeginHiddenBootstrap(DWORD child_pid, bool feature,
        int initial_mode, std::string *reason) {
    std::lock_guard<std::mutex> lock(window_lock_);
    auto reject = [&](const char *why) -> std::shared_ptr<HiddenBootstrap> { if (reason) *reason = why; return {}; };
    if (!hidden_boot_required_ || !window_policy_.active) return reject("not_waiting_for_hidden_boot");
    if (child_pid == 0 || child_pid != Pid()) return reject("wrong_direct_child_pid");
    if (!feature || initial_mode != 1) return reject("initial_feature_mode_not_minimized");
    if (hidden_boot_state_ != "waiting") return reject("duplicate_bootstrap");
    auto ticket = std::make_shared<HiddenBootstrap>(child_pid, launch_generation_, GetTickCount64() + 1500);
    if (ticket->done == nullptr) return reject("bootstrap_event_unavailable");
    hidden_boot_ticket_ = ticket;
    hidden_boot_state_ = "queued";
    return ticket;
}

void GodotManager::CancelHiddenBootstrap(const std::shared_ptr<HiddenBootstrap> &ticket, const char *reason) {
    if (!ticket) return;
    ticket->Reject(reason);
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        if (hidden_boot_ticket_ == ticket) hidden_boot_state_ = ticket->FailureReason();
    }
    SetEvent(ticket->done);
}

void GodotManager::PresentHiddenBootstrap(const std::shared_ptr<HiddenBootstrap> &ticket) {
    if (ticket) {
        std::lock_guard<std::mutex> lock(ticket->trace_lock);
        if (ticket->ui_enter_tick == 0) ticket->ui_enter_tick = GetTickCount64();
    }
    const auto capture = [&](HWND window, bool before) {
        HiddenBootstrap::WindowFacts facts;
        facts.window = window;
        facts.foreground = GetForegroundWindow();
        facts.captured = true;
        facts.visible = IsWindowVisible(window) != FALSE;
        facts.iconic = IsIconic(window) != FALSE;
        const LONG_PTR style = GetWindowLongPtrW(window, GWL_EXSTYLE);
        facts.topmost = (style & WS_EX_TOPMOST) != 0;
        facts.noactivate = (style & WS_EX_NOACTIVATE) != 0;
        if (window != nullptr) GetWindowThreadProcessId(window, &facts.pid);
        facts.tick = GetTickCount64();
        std::lock_guard<std::mutex> lock(ticket->trace_lock);
        (before ? ticket->before : ticket->after) = facts;
        return facts;
    };
    HWND shell = nullptr;
    unsigned long long generation = 0;
    bool active = false;
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        shell = window_policy_.shell;
        generation = launch_generation_;
        active = window_policy_.active && hidden_boot_ticket_ == ticket;
    }
    if (!ticket || !ticket->CanPresent(Pid(), generation, active, GetTickCount64(), GetForegroundWindow() != nullptr)) {
        CancelHiddenBootstrap(ticket, "late_or_inactive_bootstrap");
        return;
    }
    EnumContext ctx;
    ctx.pid = ticket->pid;
    ctx.include_hidden = true;
    EnumWindows(EnumWindowsProc, reinterpret_cast<LPARAM>(&ctx));
    HWND window = ctx.best;
    const auto before = capture(window, true);
    const bool foreground_is_daw = WindowBelongsToTree(before.foreground, ticket->pid);
    bool complete_hook_interval = false, daw_foreground_seen = false;
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        complete_hook_interval = event_hooks_ready_ && guard_started_seq_ != 0 && !launch_foreground_unknown_
                && (raw_window_events_.empty() || raw_window_events_.front().seq <= guard_started_seq_);
        daw_foreground_seen = launch_foreground_seen_;
    }
    if (!IsWindow(shell) || !IsWindowVisible(shell) || IsIconic(shell) || window == nullptr || before.pid != ticket->pid) {
        CancelHiddenBootstrap(ticket, "bootstrap_window_or_shell_unavailable");
        return;
    }
    if (!HiddenBootstrap::SafePrestate(before.visible, before.iconic, before.topmost,
            foreground_is_daw, complete_hook_interval, daw_foreground_seen)) {
        CancelHiddenBootstrap(ticket, !complete_hook_interval ? "bootstrap_hook_interval_incomplete"
                : daw_foreground_seen ? "bootstrap_daw_foreground_previously_observed"
                : foreground_is_daw ? "bootstrap_daw_already_foreground"
                : before.topmost ? "bootstrap_window_is_topmost" : "bootstrap_visible_noniconic_window");
        return;
    }
    const LONG_PTR style = GetWindowLongPtrW(window, GWL_EXSTYLE);
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        if (!ticket->CanPresent(Pid(), launch_generation_, window_policy_.active, GetTickCount64(), GetForegroundWindow() != nullptr)) {
            ticket->canceled = true;
        } else {
            protected_windows_.push_back({window, ticket->pid, (style & WS_EX_NOACTIVATE) == 0});
        }
    }
    if (ticket->canceled) { CancelHiddenBootstrap(ticket, "canceled_before_native_style"); return; }
    SetLastError(0);
    const LONG_PTR previous = SetWindowLongPtrW(window, GWL_EXSTYLE, style | WS_EX_NOACTIVATE);
    if ((previous == 0 && GetLastError() != 0) || (GetWindowLongPtrW(window, GWL_EXSTYLE) & WS_EX_NOACTIVATE) == 0) {
        capture(window, false);
        CancelHiddenBootstrap(ticket, "noactivate_style_failed"); return;
    }
    if (before.visible) {
        // Stock Godot may have consumed the startup SW_HIDE before its final
        // minimized show. Accept only a proven nonforeground iconic window,
        // protect it first, and hide it without activating or changing z.
        if (!SetWindowPos(window, nullptr, 0, 0, 0, 0,
                SWP_HIDEWINDOW | SWP_NOACTIVATE | SWP_NOZORDER | SWP_NOMOVE | SWP_NOSIZE | SWP_NOOWNERZORDER)
                || IsWindowVisible(window)) {
            capture(window, false);
            CancelHiddenBootstrap(ticket, "bootstrap_iconic_hide_failed"); return;
        }
    }
    MONITORINFO monitor = {};
    monitor.cbSize = sizeof(monitor);
    if (!GetMonitorInfoW(MonitorFromWindow(shell, MONITOR_DEFAULTTONEAREST), &monitor)
            || !ticket->CanPresent(Pid(), generation, true, GetTickCount64(), GetForegroundWindow() != nullptr)) {
        capture(window, false);
        CancelHiddenBootstrap(ticket, "monitor_or_bootstrap_deadline_failed"); return;
    }
    // The child's first ShowWindow has ALREADY returned before its script ACK.
    // Configure the still-hidden ordinary large window before its first visible
    // frame. Godot observes native WM_WINDOWPOSCHANGED; no internal flags are
    // patched and no activating ShowWindow/maximize fallback is used.
    SetWindowLongPtrW(window, GWL_STYLE, GetWindowLongPtrW(window, GWL_STYLE) & ~(static_cast<LONG_PTR>(WS_MINIMIZE) | WS_MAXIMIZE));
    const RECT work = monitor.rcWork;
    const bool positioned = SetWindowPos(window, shell, work.left, work.top, work.right - work.left, work.bottom - work.top,
            SWP_NOACTIVATE | SWP_NOOWNERZORDER | SWP_FRAMECHANGED) != FALSE;
    if (!positioned || IsWindowVisible(window) || IsIconic(window)
            || !ticket->CanPresent(Pid(), generation, true, GetTickCount64(), GetForegroundWindow() != nullptr)) {
        capture(window, false);
        CancelHiddenBootstrap(ticket, "hidden_geometry_preparation_failed"); return;
    }
    // SHOWWINDOW + explicit insertion position + NOACTIVATE is one native
    // operation. Never first show in front and move behind afterward.
    const bool shown = SetWindowPos(window, shell, 0, 0, 0, 0,
            SWP_SHOWWINDOW | SWP_NOACTIVATE | SWP_NOMOVE | SWP_NOSIZE | SWP_NOOWNERZORDER) != FALSE;
    if (!shown || !IsWindowVisible(window) || IsIconic(window) || WindowBelongsToTree(GetForegroundWindow(), ticket->pid)
            || (GetWindowLongPtrW(window, GWL_EXSTYLE) & WS_EX_TOPMOST)
            || (GetWindowLongPtrW(window, GWL_EXSTYLE) & WS_EX_NOACTIVATE) == 0) {
        capture(window, false);
        CancelHiddenBootstrap(ticket, "native_noactivate_presentation_failed"); return;
    }
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        if (!ticket->CanPresent(Pid(), launch_generation_, window_policy_.active, GetTickCount64(), GetForegroundWindow() != nullptr)) {
            ticket->canceled = true;
        } else {
            hidden_boot_state_ = "prepared";
            ticket->outcome = HiddenBootstrap::prepared;
        }
    }
    if (ticket->canceled) { CancelHiddenBootstrap(ticket, "canceled_during_native_presentation"); return; }
    capture(window, false);
    {
        std::lock_guard<std::mutex> lock(ticket->trace_lock);
        ticket->completed_tick = GetTickCount64();
    }
    SetEvent(ticket->done);
}

std::string GodotManager::WindowStateJson() const {
    HWND shell = nullptr;
    bool active = false;
    bool hooks_ready = false;
    std::string boot_state;
    std::string reason;
    std::deque<RawWindowEvent> raw_events;
    unsigned long long raw_seq = 0, raw_dropped = 0;
    ULONGLONG guard_started = 0, guard_released = 0;
    unsigned long long started_seq = 0, released_seq = 0;
    std::string launch_id;
    std::shared_ptr<HiddenBootstrap> boot_ticket;
    bool foreground_seen = false, foreground_unknown = false;
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        shell = window_policy_.shell;
        active = window_policy_.active;
        hooks_ready = event_hooks_ready_;
        reason = window_policy_.release_reason;
        boot_state = hidden_boot_state_;
        raw_events = raw_window_events_;
        raw_seq = raw_window_seq_; raw_dropped = raw_window_dropped_;
        guard_started = guard_started_tick_; guard_released = guard_released_tick_;
        started_seq = guard_started_seq_; released_seq = guard_released_seq_;
        launch_id = observed_launch_id_;
        boot_ticket = hidden_boot_ticket_;
        foreground_seen = launch_foreground_seen_; foreground_unknown = launch_foreground_unknown_;
    }
    HWND daw = nullptr;
    std::vector<DWORD> pids;
    const DWORD root = Pid();
    if (root != 0) CollectDescendantPids(root, pids);
    for (DWORD pid : pids) {
        daw = FindMainWindowByPid(pid);
        if (daw != nullptr) break;
    }
    const HWND foreground = GetForegroundWindow();
    const char *face = foreground == nullptr ? "none"
            : (foreground == shell ? "shell" : (WindowBelongsToTree(foreground, root) ? "daw" : "other"));
    bool order_known = IsWindow(shell) && daw != nullptr && !IsIconic(shell) && !IsIconic(daw)
            && IsWindowVisible(shell) && IsWindowVisible(daw);
    bool shell_above = false;
    if (order_known) {
        for (HWND next = GetTopWindow(nullptr); next != nullptr; next = GetWindow(next, GW_HWNDNEXT)) {
            if (next == shell) { shell_above = true; break; }
            if (next == daw) break;
        }
    }
    auto handle_json = [](HWND window) -> std::string {
        return IsWindow(window) ? ("\"" + std::to_string(reinterpret_cast<uintptr_t>(window)) + "\"") : "null";
    };
    std::string json = "{\"schema\":\"vitshell.window_state.v1\",\"loading_guard_active\":";
    json += active ? "true" : "false";
    json += ",\"release_reason\":\"" + reason + "\",\"event_hooks_ready\":" + (hooks_ready ? "true" : "false");
    json += ",\"shell_hwnd\":" + handle_json(shell) + ",\"daw_hwnd\":" + handle_json(daw);
    json += ",\"foreground\":\"" + std::string(face) + "\",\"foreground_hwnd\":" + handle_json(foreground);
    json += ",\"shell_above_daw\":" + std::string(order_known ? (shell_above ? "true" : "false") : "null");
    json += ",\"daw_noactivate\":" + std::string(daw != nullptr
            ? ((GetWindowLongPtrW(daw, GWL_EXSTYLE) & WS_EX_NOACTIVATE) ? "true" : "false") : "null");
    json += ",\"daw_topmost\":" + std::string(daw != nullptr
            ? ((GetWindowLongPtrW(daw, GWL_EXSTYLE) & WS_EX_TOPMOST) ? "true" : "false") : "null");
    json += ",\"daw_visible\":" + std::string(daw != nullptr ? (IsWindowVisible(daw) ? "true" : "false") : "null");
    json += ",\"daw_iconic\":" + std::string(daw != nullptr ? (IsIconic(daw) ? "true" : "false") : "null");
    json += ",\"hidden_boot_state\":\"" + boot_state + "\"";
    json += ",\"direct_child_pid\":" + std::to_string(root);
    json += ",\"hidden_boot_trace\":{\"daw_foreground_seen\":" + std::string(foreground_seen ? "true" : "false")
            + ",\"foreground_identity_unknown\":" + (foreground_unknown ? "true" : "false");
    if (boot_ticket) {
        std::lock_guard<std::mutex> lock(boot_ticket->trace_lock);
        json += ",\"queued_tick_ms\":" + std::to_string(boot_ticket->queued_tick)
                + ",\"ui_enter_tick_ms\":" + std::to_string(boot_ticket->ui_enter_tick)
                + ",\"completed_tick_ms\":" + std::to_string(boot_ticket->completed_tick)
                + ",\"outcome\":" + std::to_string(boot_ticket->outcome.load())
                + ",\"cancel_reason\":" + JsonQuoted(boot_ticket->cancel_reason);
        auto facts_json = [](const HiddenBootstrap::WindowFacts &facts) {
            if (!facts.captured) return std::string("null");
            return "{\"hwnd\":\"" + std::to_string(reinterpret_cast<uintptr_t>(facts.window))
                    + "\",\"pid\":" + std::to_string(facts.pid) + ",\"tick_ms\":" + std::to_string(facts.tick)
                    + ",\"foreground_hwnd\":\"" + std::to_string(reinterpret_cast<uintptr_t>(facts.foreground))
                    + "\",\"visible\":" + (facts.visible ? "true" : "false") + ",\"iconic\":" + (facts.iconic ? "true" : "false")
                    + ",\"topmost\":" + (facts.topmost ? "true" : "false") + ",\"noactivate\":" + (facts.noactivate ? "true" : "false") + "}";
        };
        json += ",\"before\":" + facts_json(boot_ticket->before) + ",\"after\":" + facts_json(boot_ticket->after);
    }
    json += "}";
    json += ",\"win_event_log\":{\"schema\":\"vitshell.win_event_log.v1\",\"last_seq\":" + std::to_string(raw_seq)
            + ",\"dropped_count\":" + std::to_string(raw_dropped) + ",\"truncated\":" + (raw_dropped ? "true" : "false")
            + ",\"guard_started_tick_ms\":" + std::to_string(guard_started)
            + ",\"guard_released_tick_ms\":" + std::to_string(guard_released) + ",\"launch_id\":" + JsonQuoted(launch_id) + ",\"events\":[";
    // Started seq is the first possible event; released seq is the last one
    // captured before release. start>end is an empty captured interval.
    const size_t events_pos = json.rfind(",\"events\":[");
    json.insert(events_pos, ",\"guard_started_seq\":" + std::to_string(started_seq)
            + ",\"guard_released_seq\":" + std::to_string(released_seq));
    bool first = true;
    for (const auto &e : raw_events) {
        if (!first) json += ",";
        first = false;
        json += "{\"seq\":" + std::to_string(e.seq) + ",\"event\":" + std::to_string(e.event)
                + ",\"target_hwnd\":\"" + std::to_string(reinterpret_cast<uintptr_t>(e.target)) + "\",\"target_pid\":" + std::to_string(e.target_pid)
                + ",\"event_thread\":" + std::to_string(e.event_thread) + ",\"event_time_ms\":" + std::to_string(e.event_time)
                + ",\"received_tick_ms\":" + std::to_string(e.received_tick) + ",\"object_id\":" + std::to_string(e.object_id)
                + ",\"child_id\":" + std::to_string(e.child_id) + ",\"launch_pid\":" + std::to_string(e.launch_pid)
                + ",\"launch_generation\":" + std::to_string(e.generation) + ",\"launch_id\":" + JsonQuoted(e.launch_id)
                + ",\"guard_active_at_receive\":" + (e.guard_active ? "true" : "false") + "}";
    }
    json += "]}";
    json += ",\"observed_at_tick_ms\":" + std::to_string(GetTickCount64()) + "}";
    return json;
}

GodotManager::~GodotManager() {
    Shutdown(0);
}

void GodotManager::SetEnv(const wchar_t *key, const std::wstring &value) {
    SetEnvironmentVariableW(key, value.c_str());
}

void GodotManager::EnsureJob() {
    if (job_ != nullptr) {
        return;
    }
    job_ = CreateJobObjectW(nullptr, nullptr);
    if (job_ == nullptr) {
        return;
    }
    JOBOBJECT_EXTENDED_LIMIT_INFORMATION info = {};
    info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    SetInformationJobObject(job_, JobObjectExtendedLimitInformation, &info, sizeof(info));
}

bool GodotManager::Start(const GodotLaunch &launch, std::string *error) {
    if (launch.exe_path.empty() || launch.project_dir.empty()) {
        if (error != nullptr) {
            *error = "godot exe path or project dir is empty";
        }
        return false;
    }
    DWORD attrs = GetFileAttributesW(launch.exe_path.c_str());
    if (attrs == INVALID_FILE_ATTRIBUTES || (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
        if (error != nullptr) {
            *error = "godot exe not found: " + std::string(launch.exe_path.begin(), launch.exe_path.end());
        }
        return false;
    }
    attrs = GetFileAttributesW(launch.project_dir.c_str());
    if (attrs == INVALID_FILE_ATTRIBUTES || !(attrs & FILE_ATTRIBUTE_DIRECTORY)) {
        if (error != nullptr) {
            *error = "godot project dir not found";
        }
        return false;
    }
    const std::wstring direct_exe = DirectGodotExecutable(launch.exe_path);
    // The deployed target is the official 4.6.1 editor binary running a
    // project. Export templates omit this TOOLS_ENABLED capability. Refuse
    // before spawn if either compiled feature support or its initial-mode
    // configuration is absent; a post-show script rejection is too late.
    if (!HiddenBootPreflight(direct_exe, launch.project_dir, error)) return false;

    {
        std::lock_guard<std::mutex> lock(window_lock_);
        if (!event_hooks_ready_ || !IsWindow(window_policy_.shell)) {
            if (error != nullptr) *error = "loading window event hooks or shell window unavailable";
            return false;
        }
    }
    // A previous exit watcher must finish before its handle is reused. It may
    // otherwise observe/close the next launch or dereference this after exit.
    Shutdown(0);
    launch_ = launch;

    // Direct-launch contract env (see header). Additive: the child inherits the
    // rest of VitShell's environment.
    SetEnv(L"VIT_SHELL_DIRECT", L"1");
    SetEnv(L"VIT_SHELL_LAUNCH_ACTION", launch_.launch_action.empty() ? L"open" : launch_.launch_action);
    SetEnv(L"VIT_SHELL_OPEN_PROJECT", launch_.open_project_path);
    SetEnv(L"VIT_SHELL_PROJECT_NAME", launch_.project_name);
    if (launch_.status_port > 0) {
        wchar_t port_buf[16] = {};
        std::swprintf(port_buf, 16, L"%d", launch_.status_port);
        SetEnv(L"VIT_SHELL_STATUS_PORT", port_buf);
    }
    // Deliberately NOT set here (documented in the header):
    //   VIT_AGENT_WINDOW_MODE (343 window host stays dormant)
    //   VIT_FROM_LAUNCHER     (SHELL-4: kernel/hub are owned by the VitShell
    //                          agent tree and are already online before any
    //                          direct launch, so Godot's autostart is a probe
    //                          fallback that attaches instead of spawning;
    //                          agent 7878 already answers, so
    //                          _try_boot_local_bridge skips)

    std::wstring cmdline = L"\"" + direct_exe + L"\" --path \"" + launch_.project_dir + L"\"";

    EnsureJob();
    STARTUPINFOW si = {};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;
    PROCESS_INFORMATION pi = {};
    bool created = false;
    DWORD create_error = ERROR_SUCCESS;
    {
        ScopedChildEnvironment features(L"GODOT_EDITOR_CUSTOM_FEATURES");
        ScopedChildEnvironment required(L"VIT_SHELL_REQUIRE_HIDDEN_BOOT");
        const std::wstring child_features = features.Previous().empty() ? L"vit_shell_hidden_boot"
                : features.Previous() + L",vit_shell_hidden_boot";
        if (features.Set(child_features) && required.Set(L"1")) {
            created = CreateProcessW(nullptr, cmdline.data(), nullptr, nullptr, FALSE,
                    CREATE_SUSPENDED | CREATE_NO_WINDOW, nullptr, launch_.project_dir.c_str(), &si, &pi) != FALSE;
        }
        if (!created) create_error = GetLastError();
    }
    if (!created) {
        if (error != nullptr) {
            *error = LastErrorText("CreateProcessW(godot)", create_error);
        }
        return false;
    }
    if (job_ != nullptr) {
        AssignProcessToJobObject(job_, pi.hProcess);
    }
    {
        std::lock_guard<std::mutex> lock(process_lock_);
        process_ = pi.hProcess;
    }
    {
        std::lock_guard<std::mutex> lock(window_lock_);
        pid_ = pi.dwProcessId;
        window_policy_.Begin(window_policy_.shell, GetForegroundWindow());
        guard_started_tick_ = GetTickCount64();
        guard_released_tick_ = 0;
        guard_started_seq_ = raw_window_seq_ + 1;
        guard_released_seq_ = 0;
        launch_foreground_seen_ = false; launch_foreground_unknown_ = false;
        const DWORD nonce_size = GetEnvironmentVariableA("VIT_SHELL_LAUNCH_ID", nullptr, 0);
        observed_launch_id_.assign(nonce_size, '\0');
        if (nonce_size) observed_launch_id_.resize(GetEnvironmentVariableA("VIT_SHELL_LAUNCH_ID", observed_launch_id_.data(), nonce_size));
        ++launch_generation_;
        hidden_boot_required_ = true;
        hidden_boot_state_ = "waiting";
        hidden_boot_ticket_.reset();
    }
    ResumeThread(pi.hThread);
    CloseHandle(pi.hThread);

    // Exit watcher thread: goal 5 — the host senses the DAW process exiting
    // (user closes the DAW window) and the caller flips React back to the
    // launcher surface. Shutdown joins before closing/reusing process_, so a
    // queued exit from an older launch cannot be assigned to the new launch.
    InterlockedExchange(&watch_stop_, 0);
    watch_thread_ = CreateThread(nullptr, 0,
            [](LPVOID param) -> DWORD {
                auto *self = static_cast<GodotManager *>(param);
                HANDLE proc = self->process_;
                while (proc != nullptr
                        && InterlockedCompareExchange(&self->watch_stop_, 0, 0) == 0) {
                    const DWORD wait = WaitForSingleObject(proc, 500);
                    if (wait == WAIT_OBJECT_0) {
                        DWORD exit_code = 0;
                        GetExitCodeProcess(proc, &exit_code);
                        if (self->exit_handler_) {
                            self->exit_handler_(exit_code);
                        }
                        break;
                    }
                }
                return 0;
            },
            this, 0, nullptr);
    return true;
}

void GodotManager::StopWatcher() {
    InterlockedExchange(&watch_stop_, 1);
    if (watch_thread_ != nullptr) {
        WaitForSingleObject(watch_thread_, INFINITE); // watcher waits at most 500ms
        CloseHandle(watch_thread_);
        watch_thread_ = nullptr;
    }
}

bool GodotManager::IsRunning() const {
    std::lock_guard<std::mutex> lock(process_lock_);
    if (process_ == nullptr) {
        return false;
    }
    DWORD code = 0;
    return GetExitCodeProcess(process_, &code) && code == STILL_ACTIVE;
}

void GodotManager::RequestClose(int wait_ms) {
    if (process_ == nullptr) {
        return;
    }
    // Graceful path first: WM_CLOSE on the DAW main window == user's X button
    // (Godot's own exit cleanup runs: VitIpcClient kills kernel/hub children).
    // The tracked pid is the console WRAPPER; the editor window belongs to a
    // spawned child - post to the whole descendant subtree.
    std::vector<DWORD> pids;
    CollectDescendantPids(pid_, pids);
    for (const DWORD pid : pids) {
        EnumContext context;
        context.pid = pid;
        context.include_hidden = true;
        EnumWindows(EnumWindowsProc, reinterpret_cast<LPARAM>(&context));
        HWND window = context.best;
        if (window != nullptr) {
            PostMessageW(window, WM_CLOSE, 0, 0);
        }
    }
    if (wait_ms > 0) {
        const DWORD deadline = GetTickCount() + static_cast<DWORD>(wait_ms);
        while (IsRunning() && GetTickCount() < deadline) {
            Sleep(50);
        }
        if (IsRunning()) {
            TerminateProcess(process_, 1);
        }
    }
}

void GodotManager::Shutdown(int wait_ms) {
    ReleaseLoadingWindowGuard("shutdown");
    StopWatcher();
    if (wait_ms > 0) RequestClose(0);
    std::lock_guard<std::mutex> lock(process_lock_);
    if (process_ != nullptr) {
        TerminateProcess(process_, 1);
        if (wait_ms > 0) {
            WaitForSingleObject(process_, static_cast<DWORD>(wait_ms));
        }
        CloseHandle(process_);
        process_ = nullptr;
    }
    pid_ = 0;
    if (job_ != nullptr) {
        CloseHandle(job_); // kill-on-close -> whole Godot tree terminated
        job_ = nullptr;
    }
}

} // namespace vitshell
