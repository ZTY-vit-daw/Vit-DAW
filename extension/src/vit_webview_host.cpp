#include "vit_webview_host.h"

#include <godot_cpp/core/class_db.hpp>

namespace godot {

void VitWebViewHost::_bind_methods() {
    ClassDB::bind_method(D_METHOD("create", "parent_hwnd"), &VitWebViewHost::create);
    ClassDB::bind_method(D_METHOD("create_window", "parent_hwnd", "title", "width", "height"), &VitWebViewHost::create_window);
    ClassDB::bind_method(D_METHOD("create_overlay_window", "parent_hwnd", "title", "width", "height"), &VitWebViewHost::create_overlay_window);
    ClassDB::bind_method(D_METHOD("create_composition", "parent_hwnd"), &VitWebViewHost::create_composition);
    ClassDB::bind_method(D_METHOD("create_probe", "parent_hwnd"), &VitWebViewHost::create_probe);
    ClassDB::bind_method(D_METHOD("close"), &VitWebViewHost::close);
    ClassDB::bind_method(D_METHOD("set_bounds", "x", "y", "width", "height"), &VitWebViewHost::set_bounds);
    ClassDB::bind_method(D_METHOD("set_screen_bounds", "x", "y", "width", "height"), &VitWebViewHost::set_screen_bounds);
    ClassDB::bind_method(D_METHOD("set_visible", "visible"), &VitWebViewHost::set_visible);
    ClassDB::bind_method(D_METHOD("focus"), &VitWebViewHost::focus);
    ClassDB::bind_method(D_METHOD("send_mouse_input", "event_kind", "virtual_keys", "mouse_data", "x", "y"), &VitWebViewHost::send_mouse_input);
    ClassDB::bind_method(D_METHOD("navigate", "url"), &VitWebViewHost::navigate);
    ClassDB::bind_method(D_METHOD("navigate_html", "html"), &VitWebViewHost::navigate_html);
    ClassDB::bind_method(D_METHOD("go_back"), &VitWebViewHost::go_back);
    ClassDB::bind_method(D_METHOD("go_forward"), &VitWebViewHost::go_forward);
    ClassDB::bind_method(D_METHOD("reload"), &VitWebViewHost::reload);
    ClassDB::bind_method(D_METHOD("stop"), &VitWebViewHost::stop);
    ClassDB::bind_method(D_METHOD("eval_js", "script"), &VitWebViewHost::eval_js);
    ClassDB::bind_method(D_METHOD("post_web_message", "message"), &VitWebViewHost::post_web_message);
    ClassDB::bind_method(D_METHOD("set_user_data_subdir", "subdir"), &VitWebViewHost::set_user_data_subdir);
    ClassDB::bind_method(D_METHOD("debug_state"), &VitWebViewHost::debug_state);

    ADD_SIGNAL(MethodInfo("ready"));
    ADD_SIGNAL(MethodInfo("navigation_started", PropertyInfo(Variant::STRING, "url")));
    ADD_SIGNAL(MethodInfo("page_state_changed", PropertyInfo(Variant::DICTIONARY, "state")));
    ADD_SIGNAL(MethodInfo("load_error", PropertyInfo(Variant::STRING, "message")));
    ADD_SIGNAL(MethodInfo("web_message_received", PropertyInfo(Variant::STRING, "message")));
}

} // namespace godot

#if defined(_WIN32) && defined(VIT_WITH_WEBVIEW2)

#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>

#include <dcomp.h>
#include <WebView2.h>
#include <godot_cpp/classes/display_server.hpp>
#include <godot_cpp/variant/char_string.hpp>
#include <wrl/client.h>
#include <wrl/event.h>

#include <algorithm>
#include <cstdarg>
#include <cstdio>
#include <functional>
#include <string>
#include <unordered_map>
#include <vector>

EXTERN_C IMAGE_DOS_HEADER __ImageBase;

namespace godot {

using Microsoft::WRL::Callback;
using Microsoft::WRL::ComPtr;

struct VitWebViewHost::NativeState {
    using CreateEnvironmentFn = HRESULT(STDAPICALLTYPE *)(PCWSTR, PCWSTR, ICoreWebView2EnvironmentOptions *,
            ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler *);

    HMODULE loader = nullptr;
    CreateEnvironmentFn create_environment = nullptr;
    HWND parent = nullptr;
    HWND child_window = nullptr;
    RECT bounds = {0, 0, 800, 600};
    std::wstring window_title = L"Vit Browser";
    bool detached_window = false;
    bool composition_mode = false;
    bool overlay_window = false;
    bool com_initialized = false;
    std::wstring user_data_subdir;
    ComPtr<IDCompositionDevice> dcomp_device;
    ComPtr<IDCompositionTarget> dcomp_target;
    ComPtr<IDCompositionVisual> dcomp_root_visual;
    ComPtr<IDCompositionVisual> dcomp_webview_visual;
    ComPtr<ICoreWebView2CompositionController> composition_controller;
    ComPtr<ICoreWebView2Environment> environment;
    ComPtr<ICoreWebView2Controller> controller;
    ComPtr<ICoreWebView2> webview;
};

namespace {

std::unordered_map<HWND, ICoreWebView2Controller *> g_webview_controllers;
std::unordered_map<HWND, COLORREF> g_webview_probe_colors;

std::string wide_to_utf8(const std::wstring &value) {
    if (value.empty()) {
        return "";
    }
    const int size = WideCharToMultiByte(CP_UTF8, 0, value.c_str(), -1, nullptr, 0, nullptr, nullptr);
    if (size <= 1) {
        return "";
    }
    std::string out(static_cast<size_t>(size - 1), '\0');
    WideCharToMultiByte(CP_UTF8, 0, value.c_str(), -1, out.data(), size, nullptr, nullptr);
    return out;
}

std::wstring utf8_to_wide(const char *value) {
    if (value == nullptr || value[0] == '\0') {
        return L"";
    }
    const int size = MultiByteToWideChar(CP_UTF8, 0, value, -1, nullptr, 0);
    if (size <= 1) {
        return L"";
    }
    std::wstring out(static_cast<size_t>(size - 1), L'\0');
    MultiByteToWideChar(CP_UTF8, 0, value, -1, out.data(), size);
    return out;
}

std::wstring godot_to_wide(const String &value) {
    CharString utf8 = value.utf8();
    return utf8_to_wide(utf8.get_data());
}

String wide_to_godot(const std::wstring &value) {
    return String::utf8(wide_to_utf8(value).c_str());
}

std::wstring directory_of(const std::wstring &path) {
    const size_t slash = path.find_last_of(L"\\/");
    if (slash == std::wstring::npos) {
        return L"";
    }
    return path.substr(0, slash);
}

std::wstring module_directory() {
    wchar_t path[MAX_PATH] = {};
    const DWORD len = GetModuleFileNameW(reinterpret_cast<HMODULE>(&__ImageBase), path, MAX_PATH);
    if (len == 0 || len >= MAX_PATH) {
        return L"";
    }
    return directory_of(path);
}

std::wstring default_user_data_dir() {
    wchar_t local_app_data[MAX_PATH] = {};
    DWORD len = GetEnvironmentVariableW(L"LOCALAPPDATA", local_app_data, MAX_PATH);
    std::wstring base;
    if (len > 0 && len < MAX_PATH) {
        base = local_app_data;
    } else {
        base = module_directory();
    }
    std::wstring vit_dir = base + L"\\VitDAW";
    CreateDirectoryW(vit_dir.c_str(), nullptr);
    std::wstring webview_dir = vit_dir + L"\\WebView2";
    CreateDirectoryW(webview_dir.c_str(), nullptr);
    return webview_dir;
}

std::wstring sanitize_user_data_subdir(const std::wstring &value) {
    std::wstring out;
    out.reserve(std::min<size_t>(value.size(), 96));
    for (wchar_t ch : value) {
        if ((ch >= L'a' && ch <= L'z') || (ch >= L'A' && ch <= L'Z') || (ch >= L'0' && ch <= L'9') || ch == L'_' || ch == L'-') {
            out.push_back(ch);
        } else if (!out.empty() && out.back() != L'_') {
            out.push_back(L'_');
        }
        if (out.size() >= 96) {
            break;
        }
    }
    while (!out.empty() && out.back() == L'_') {
        out.pop_back();
    }
    return out.empty() ? L"default" : out;
}

std::wstring scoped_user_data_dir(const std::wstring &subdir) {
    std::wstring root = default_user_data_dir();
    if (subdir.empty()) {
        return root;
    }
    std::wstring scoped_root = root + L"\\Profiles";
    CreateDirectoryW(scoped_root.c_str(), nullptr);
    std::wstring scoped = scoped_root + L"\\" + sanitize_user_data_subdir(subdir);
    CreateDirectoryW(scoped.c_str(), nullptr);
    return scoped;
}

std::wstring hresult_text(const char *label, HRESULT hr) {
    char buf[160] = {};
    std::snprintf(buf, sizeof(buf), "%s failed: 0x%08X", label, static_cast<unsigned int>(hr));
    return utf8_to_wide(buf);
}

void webview_debug_log(const char *format, ...) {
    static int enabled = -1;
    if (enabled < 0) {
        wchar_t value[8] = {};
        DWORD len = GetEnvironmentVariableW(L"VIT_WEBVIEW_DEBUG", value, 8);
        enabled = (len > 0 && (value[0] == L'1' || value[0] == L't' || value[0] == L'T')) ? 1 : 0;
    }
    if (!enabled) {
        return;
    }
    char body[768] = {};
    va_list args;
    va_start(args, format);
    std::vsnprintf(body, sizeof(body), format, args);
    va_end(args);

    char line[840] = {};
    std::snprintf(line, sizeof(line), "[VitWebViewHost] %s\n", body);
    std::fprintf(stderr, "%s", line);
    std::fflush(stderr);
    OutputDebugStringA(line);
}

void apply_child_window_parent_style(HWND hwnd) {
    if (hwnd == nullptr) {
        return;
    }
    LONG_PTR style = GetWindowLongPtrW(hwnd, GWL_STYLE);
    style |= WS_CLIPCHILDREN | WS_CLIPSIBLINGS;
    SetWindowLongPtrW(hwnd, GWL_STYLE, style);
    SetWindowPos(hwnd, nullptr, 0, 0, 0, 0,
            SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
}

void resize_controller_to_client(HWND hwnd) {
    auto it = g_webview_controllers.find(hwnd);
    if (it == g_webview_controllers.end() || it->second == nullptr) {
        return;
    }
    RECT client = {};
    if (GetClientRect(hwnd, &client)) {
        it->second->put_Bounds(client);
    }
}

LRESULT CALLBACK webview_child_wnd_proc(HWND hwnd, UINT msg, WPARAM wparam, LPARAM lparam) {
    switch (msg) {
        case WM_ERASEBKGND: {
            auto color_it = g_webview_probe_colors.find(hwnd);
            if (color_it != g_webview_probe_colors.end()) {
                RECT client = {};
                GetClientRect(hwnd, &client);
                HBRUSH brush = CreateSolidBrush(color_it->second);
                FillRect(reinterpret_cast<HDC>(wparam), &client, brush);
                DeleteObject(brush);
                return 1;
            }
            break;
        }
        case WM_PAINT: {
            auto color_it = g_webview_probe_colors.find(hwnd);
            if (color_it != g_webview_probe_colors.end()) {
                PAINTSTRUCT ps = {};
                HDC hdc = BeginPaint(hwnd, &ps);
                RECT client = {};
                GetClientRect(hwnd, &client);
                HBRUSH brush = CreateSolidBrush(color_it->second);
                FillRect(hdc, &client, brush);
                DeleteObject(brush);
                EndPaint(hwnd, &ps);
                return 0;
            }
            break;
        }
        case WM_SIZE:
            resize_controller_to_client(hwnd);
            break;
        case WM_CLOSE:
            ShowWindow(hwnd, SW_HIDE);
            return 0;
        case WM_DESTROY:
            g_webview_controllers.erase(hwnd);
            g_webview_probe_colors.erase(hwnd);
            break;
    }
    return DefWindowProcW(hwnd, msg, wparam, lparam);
}

const wchar_t *webview_child_class_name() {
    return L"VitWebViewHostChildWindow";
}

bool ensure_webview_child_class() {
    static ATOM atom = 0;
    if (atom != 0) {
        return true;
    }
    WNDCLASSW wc = {};
    wc.lpfnWndProc = webview_child_wnd_proc;
    wc.hInstance = reinterpret_cast<HINSTANCE>(&__ImageBase);
    wc.lpszClassName = webview_child_class_name();
    wc.hCursor = LoadCursor(nullptr, IDC_ARROW);
    atom = RegisterClassW(&wc);
    return atom != 0 || GetLastError() == ERROR_CLASS_ALREADY_EXISTS;
}

RECT controller_bounds_for_child(HWND child, const RECT &bounds, bool detached_window = false) {
    if (detached_window && child != nullptr) {
        RECT client = {};
        if (GetClientRect(child, &client)) {
            return client;
        }
    }
    if (child != nullptr) {
        const int width = std::max<int>(bounds.right - bounds.left, 1);
        const int height = std::max<int>(bounds.bottom - bounds.top, 1);
        return RECT{0, 0, width, height};
    }
    return bounds;
}

RECT zero_origin_bounds(const RECT &bounds) {
    const int width = std::max<int>(bounds.right - bounds.left, 1);
    const int height = std::max<int>(bounds.bottom - bounds.top, 1);
    return RECT{0, 0, width, height};
}

void move_webview_child_window(HWND child_window, HWND parent, const RECT &bounds, bool overlay_window) {
    if (child_window == nullptr) {
        return;
    }
    const int width = std::max<int>(bounds.right - bounds.left, 1);
    const int height = std::max<int>(bounds.bottom - bounds.top, 1);
    int x = bounds.left;
    int y = bounds.top;
    if (overlay_window && parent != nullptr) {
        POINT screen_pos{x, y};
        ClientToScreen(parent, &screen_pos);
        x = screen_pos.x;
        y = screen_pos.y;
    }
    SetWindowPos(child_window, HWND_TOP, x, y, width, height, SWP_NOACTIVATE);
}

HWND create_webview_host_window(HWND parent,
        const RECT &bounds,
        bool detached_window,
        bool overlay_window,
        bool visible,
        const std::wstring &title) {
    const int child_width = std::max<int>(bounds.right - bounds.left, 1);
    const int child_height = std::max<int>(bounds.bottom - bounds.top, 1);
    DWORD ex_style = overlay_window ? WS_EX_TOOLWINDOW : 0;
    DWORD window_style = detached_window
            ? (WS_OVERLAPPEDWINDOW | WS_CLIPSIBLINGS | WS_CLIPCHILDREN | (visible ? WS_VISIBLE : 0))
            : overlay_window
            ? (WS_POPUP | WS_CLIPSIBLINGS | WS_CLIPCHILDREN | (visible ? WS_VISIBLE : 0))
            : (WS_CHILD | WS_CLIPSIBLINGS | WS_CLIPCHILDREN | (visible ? WS_VISIBLE : 0));
    const int initial_x = detached_window ? CW_USEDEFAULT : bounds.left;
    const int initial_y = detached_window ? CW_USEDEFAULT : bounds.top;
    HWND owner_or_parent = detached_window ? nullptr : parent;
    HWND child_window = CreateWindowExW(ex_style,
            webview_child_class_name(),
            detached_window ? title.c_str() : L"",
            window_style,
            initial_x,
            initial_y,
            child_width,
            child_height,
            owner_or_parent,
            nullptr,
            reinterpret_cast<HINSTANCE>(&__ImageBase),
            nullptr);
    webview_debug_log("CreateWindowExW child=%p parent=%p hwnd_parent_arg=%p detached=%d composition_host=%d bounds=(%ld,%ld %dx%d) error=%lu",
            child_window,
            parent,
            owner_or_parent,
            detached_window ? 1 : 0,
            (!detached_window && !overlay_window) ? 1 : 0,
            bounds.left,
            bounds.top,
            child_width,
            child_height,
            GetLastError());
    return child_window;
}

HRESULT setup_direct_composition(HWND parent,
        ComPtr<IDCompositionDevice> &device,
        ComPtr<IDCompositionTarget> &target,
        ComPtr<IDCompositionVisual> &root_visual,
        ComPtr<IDCompositionVisual> &webview_visual) {
    HRESULT hr = DCompositionCreateDevice(nullptr, __uuidof(IDCompositionDevice), reinterpret_cast<void **>(device.GetAddressOf()));
    if (FAILED(hr)) {
        return hr;
    }
    hr = device->CreateTargetForHwnd(parent, TRUE, &target);
    if (FAILED(hr)) {
        return hr;
    }
    hr = device->CreateVisual(&root_visual);
    if (FAILED(hr)) {
        return hr;
    }
    hr = device->CreateVisual(&webview_visual);
    if (FAILED(hr)) {
        return hr;
    }
    hr = root_visual->AddVisual(webview_visual.Get(), TRUE, nullptr);
    if (FAILED(hr)) {
        return hr;
    }
    hr = target->SetRoot(root_visual.Get());
    if (FAILED(hr)) {
        return hr;
    }
    return device->Commit();
}

void position_composition_visual(IDCompositionDevice *device, IDCompositionVisual *visual, const RECT &bounds) {
    if (device == nullptr || visual == nullptr) {
        return;
    }
    visual->SetOffsetX(static_cast<float>(bounds.left));
    visual->SetOffsetY(static_cast<float>(bounds.top));
    device->Commit();
}

bool parse_mouse_event_kind(const String &value, COREWEBVIEW2_MOUSE_EVENT_KIND &kind) {
    const std::wstring wide = godot_to_wide(value);
    if (wide == L"move") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_MOVE;
    } else if (wide == L"leave") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_LEAVE;
    } else if (wide == L"wheel") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_WHEEL;
    } else if (wide == L"horizontal_wheel") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_HORIZONTAL_WHEEL;
    } else if (wide == L"left_button_down") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_LEFT_BUTTON_DOWN;
    } else if (wide == L"left_button_up") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_LEFT_BUTTON_UP;
    } else if (wide == L"left_button_double_click") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_LEFT_BUTTON_DOUBLE_CLICK;
    } else if (wide == L"right_button_down") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_RIGHT_BUTTON_DOWN;
    } else if (wide == L"right_button_up") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_RIGHT_BUTTON_UP;
    } else if (wide == L"right_button_double_click") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_RIGHT_BUTTON_DOUBLE_CLICK;
    } else if (wide == L"middle_button_down") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_MIDDLE_BUTTON_DOWN;
    } else if (wide == L"middle_button_up") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_MIDDLE_BUTTON_UP;
    } else if (wide == L"middle_button_double_click") {
        kind = COREWEBVIEW2_MOUSE_EVENT_KIND_MIDDLE_BUTTON_DOUBLE_CLICK;
    } else {
        return false;
    }
    return true;
}

std::string json_string_unescape(const std::string &raw) {
    if (raw.size() < 2 || raw.front() != '"' || raw.back() != '"') {
        return raw;
    }
    std::string out;
    out.reserve(raw.size());
    for (size_t i = 1; i + 1 < raw.size(); ++i) {
        char c = raw[i];
        if (c != '\\' || i + 1 >= raw.size() - 1) {
            out.push_back(c);
            continue;
        }
        char esc = raw[++i];
        switch (esc) {
            case '"': out.push_back('"'); break;
            case '\\': out.push_back('\\'); break;
            case '/': out.push_back('/'); break;
            case 'b': out.push_back('\b'); break;
            case 'f': out.push_back('\f'); break;
            case 'n': out.push_back('\n'); break;
            case 'r': out.push_back('\r'); break;
            case 't': out.push_back('\t'); break;
            default:
                out.push_back('\\');
                out.push_back(esc);
                break;
        }
    }
    return out;
}

bool pump_until(const std::function<bool()> &done, DWORD timeout_ms) {
    const ULONGLONG deadline = GetTickCount64() + timeout_ms;
    MSG msg;
    while (!done()) {
        while (PeekMessageW(&msg, nullptr, 0, 0, PM_REMOVE)) {
            TranslateMessage(&msg);
            DispatchMessageW(&msg);
        }
        if (GetTickCount64() > deadline) {
            return done();
        }
        Sleep(5);
    }
    return true;
}

LPWSTR take_string(LPWSTR value) {
    return value;
}

void append_navigation_capabilities(ICoreWebView2 *webview, Dictionary &state) {
    if (webview == nullptr) {
        state["can_go_back"] = false;
        state["can_go_forward"] = false;
        return;
    }
    BOOL can_go_back = FALSE;
    BOOL can_go_forward = FALSE;
    if (SUCCEEDED(webview->get_CanGoBack(&can_go_back))) {
        state["can_go_back"] = can_go_back ? true : false;
    }
    if (SUCCEEDED(webview->get_CanGoForward(&can_go_forward))) {
        state["can_go_forward"] = can_go_forward ? true : false;
    }
}

} // namespace

VitWebViewHost::VitWebViewHost() {
    native_ = new NativeState();
}

VitWebViewHost::~VitWebViewHost() {
    close();
}

void VitWebViewHost::close() {
    if (native_ == nullptr) {
        return;
    }
    if (native_->controller) {
        native_->controller->Close();
    }
    native_->webview.Reset();
    native_->composition_controller.Reset();
    native_->controller.Reset();
    native_->dcomp_webview_visual.Reset();
    native_->dcomp_root_visual.Reset();
    native_->dcomp_target.Reset();
    native_->dcomp_device.Reset();
    native_->environment.Reset();
    if (native_->child_window != nullptr) {
        g_webview_controllers.erase(native_->child_window);
        DestroyWindow(native_->child_window);
        native_->child_window = nullptr;
    }
    if (native_->loader != nullptr) {
        FreeLibrary(native_->loader);
    }
    if (native_->com_initialized) {
        CoUninitialize();
    }
    delete native_;
    native_ = nullptr;
    ready_ = false;
    visible_ = false;
    current_url_ = "";
    last_error_ = "";
}

void VitWebViewHost::set_visible(bool visible) {
    visible_ = visible;
    if (native_ != nullptr && native_->controller) {
        native_->controller->put_IsVisible(visible ? TRUE : FALSE);
    }
    if (native_ != nullptr && native_->child_window != nullptr) {
        ShowWindow(native_->child_window, visible ? SW_SHOWNA : SW_HIDE);
    }
}

void VitWebViewHost::focus() {
    if (!ready_ || native_ == nullptr || !native_->controller) {
        return;
    }
    native_->controller->MoveFocus(COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC);
}

bool VitWebViewHost::post_web_message(String message) {
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return false;
    }
    const std::wstring wide = godot_to_wide(message);
    HRESULT hr = native_->webview->PostWebMessageAsJson(wide.c_str());
    if (FAILED(hr)) {
        hr = native_->webview->PostWebMessageAsString(wide.c_str());
    }
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("PostWebMessage", hr));
        emit_signal("load_error", last_error_);
        return false;
    }
    return true;
}

void VitWebViewHost::set_user_data_subdir(String subdir) {
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        return;
    }
    native_->user_data_subdir = godot_to_wide(subdir);
}

bool VitWebViewHost::send_mouse_input(String event_kind, int virtual_keys, int mouse_data, int x, int y) {
    if (!ready_ || native_ == nullptr || !native_->composition_controller) {
        return false;
    }
    COREWEBVIEW2_MOUSE_EVENT_KIND kind = COREWEBVIEW2_MOUSE_EVENT_KIND_MOVE;
    if (!parse_mouse_event_kind(event_kind, kind)) {
        last_error_ = "Unknown WebView2 mouse event kind: " + event_kind;
        emit_signal("load_error", last_error_);
        return false;
    }
    POINT point{std::max(x, 0), std::max(y, 0)};
    HRESULT hr = native_->composition_controller->SendMouseInput(
            kind,
            static_cast<COREWEBVIEW2_MOUSE_EVENT_VIRTUAL_KEYS>(virtual_keys),
            static_cast<UINT32>(mouse_data),
            point);
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("CompositionController::SendMouseInput", hr));
        emit_signal("load_error", last_error_);
        return false;
    }
    return true;
}

void VitWebViewHost::set_bounds(int x, int y, int width, int height) {
    if (native_ == nullptr) {
        return;
    }
    width = std::max(width, 1);
    height = std::max(height, 1);
    native_->bounds = RECT{x, y, x + width, y + height};
    if (native_->composition_mode) {
        position_composition_visual(native_->dcomp_device.Get(), native_->dcomp_webview_visual.Get(), zero_origin_bounds(native_->bounds));
    }
    if (!native_->detached_window) {
        move_webview_child_window(native_->child_window, native_->parent, native_->bounds, native_->overlay_window);
    }
    if (native_->controller) {
        RECT controller_bounds = native_->composition_mode
                ? zero_origin_bounds(native_->bounds)
                : controller_bounds_for_child(native_->child_window, native_->bounds, native_->detached_window);
        native_->controller->put_Bounds(controller_bounds);
    }
}

void VitWebViewHost::set_screen_bounds(int x, int y, int width, int height) {
    width = std::max(width, 1);
    height = std::max(height, 1);
    if (native_ != nullptr && native_->detached_window && native_->child_window != nullptr) {
        native_->bounds = RECT{0, 0, width, height};
        SetWindowPos(native_->child_window, HWND_TOP, x, y, width, height, SWP_NOACTIVATE);
        if (native_->controller) {
            RECT controller_bounds = controller_bounds_for_child(native_->child_window, native_->bounds, true);
            native_->controller->put_Bounds(controller_bounds);
        }
        return;
    }
    if (native_ == nullptr || native_->parent == nullptr) {
        set_bounds(x, y, width, height);
        return;
    }
    POINT top_left{x, y};
    ScreenToClient(native_->parent, &top_left);
    const int right = top_left.x + width;
    const int bottom = top_left.y + height;
    native_->bounds = RECT{
            top_left.x,
            top_left.y,
            std::max(static_cast<LONG>(right), static_cast<LONG>(top_left.x + 1)),
            std::max(static_cast<LONG>(bottom), static_cast<LONG>(top_left.y + 1)),
    };
    if (native_->composition_mode) {
        position_composition_visual(native_->dcomp_device.Get(), native_->dcomp_webview_visual.Get(), zero_origin_bounds(native_->bounds));
    }
    if (!native_->detached_window) {
        move_webview_child_window(native_->child_window, native_->parent, native_->bounds, native_->overlay_window);
    }
    if (native_->controller) {
        RECT controller_bounds = native_->composition_mode
                ? zero_origin_bounds(native_->bounds)
                : controller_bounds_for_child(native_->child_window, native_->bounds, native_->detached_window);
        native_->controller->put_Bounds(controller_bounds);
    }
}

bool VitWebViewHost::create_window(int64_t parent_hwnd, String title, int width, int height) {
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        set_visible(true);
        emit_signal("ready");
        return true;
    }
    native_->detached_window = true;
    native_->overlay_window = false;
    native_->window_title = godot_to_wide(title);
    if (native_->window_title.empty()) {
        native_->window_title = L"Vit Browser";
    }
    width = std::max(width, 640);
    height = std::max(height, 420);
    native_->bounds = RECT{0, 0, width, height};
    return create(parent_hwnd);
}

bool VitWebViewHost::create_overlay_window(int64_t parent_hwnd, String title, int width, int height) {
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        set_visible(true);
        emit_signal("ready");
        return true;
    }
    native_->detached_window = false;
    native_->overlay_window = true;
    native_->window_title = godot_to_wide(title);
    if (native_->window_title.empty()) {
        native_->window_title = L"Vit Browser Overlay";
    }
    width = std::max(width, 320);
    height = std::max(height, 240);
    native_->bounds = RECT{0, 0, width, height};
    return create(parent_hwnd);
}

bool VitWebViewHost::create_composition(int64_t parent_hwnd) {
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        emit_signal("ready");
        return true;
    }
    native_->composition_mode = true;
    native_->detached_window = false;
    native_->overlay_window = false;
    native_->bounds = RECT{0, 0, 800, 600};
    return create(parent_hwnd);
}

bool VitWebViewHost::create_probe(int64_t parent_hwnd) {
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        set_visible(true);
        emit_signal("ready");
        return true;
    }
    HWND parent = reinterpret_cast<HWND>(parent_hwnd);
    if (parent == nullptr) {
        DisplayServer *display = DisplayServer::get_singleton();
        if (display != nullptr) {
            parent = reinterpret_cast<HWND>(display->window_get_native_handle(DisplayServer::WINDOW_HANDLE, 0));
        }
    }
    if (parent == nullptr) {
        last_error_ = "Unable to find the Godot native window handle for WebView2.";
        emit_signal("load_error", last_error_);
        return false;
    }
    native_->parent = parent;
    native_->composition_mode = false;
    native_->detached_window = false;
    native_->overlay_window = false;
    native_->bounds = RECT{0, 0, 800, 600};
    apply_child_window_parent_style(parent);
    if (!ensure_webview_child_class()) {
        last_error_ = "Unable to register the WebView2 child window class.";
        emit_signal("load_error", last_error_);
        return false;
    }
    native_->child_window = create_webview_host_window(native_->parent,
            native_->bounds,
            false,
            false,
            visible_,
            L"Vit HWND Probe");
    if (native_->child_window == nullptr) {
        last_error_ = "Unable to create the WebView2 probe window.";
        emit_signal("load_error", last_error_);
        return false;
    }
    g_webview_probe_colors[native_->child_window] = RGB(15, 118, 110);
    move_webview_child_window(native_->child_window, native_->parent, native_->bounds, false);
    ready_ = true;
    last_error_ = "";
    InvalidateRect(native_->child_window, nullptr, TRUE);
    UpdateWindow(native_->child_window);
    emit_signal("ready");
    return true;
}

bool VitWebViewHost::create(int64_t parent_hwnd) {
    webview_debug_log("create called parent_hwnd=0x%llX ready=%d", static_cast<unsigned long long>(parent_hwnd), ready_ ? 1 : 0);
    if (native_ == nullptr) {
        native_ = new NativeState();
    }
    if (ready_) {
        emit_signal("ready");
        return true;
    }

    HRESULT hr = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);
    if (SUCCEEDED(hr)) {
        native_->com_initialized = true;
    } else if (hr != RPC_E_CHANGED_MODE) {
        last_error_ = wide_to_godot(hresult_text("CoInitializeEx", hr));
        emit_signal("load_error", last_error_);
        return false;
    }

    HWND parent = reinterpret_cast<HWND>(parent_hwnd);
    if (parent == nullptr) {
        DisplayServer *display = DisplayServer::get_singleton();
        if (display != nullptr) {
            parent = reinterpret_cast<HWND>(display->window_get_native_handle(DisplayServer::WINDOW_HANDLE, 0));
            webview_debug_log("create fallback DisplayServer WINDOW_HANDLE parent=%p", parent);
        }
    }
    if (parent == nullptr) {
        last_error_ = "Unable to find the Godot native window handle for WebView2.";
        emit_signal("load_error", last_error_);
        return false;
    }
    native_->parent = parent;
    webview_debug_log("create using parent=%p", native_->parent);
    if (!native_->detached_window) {
        apply_child_window_parent_style(parent);
    }
    if (!ensure_webview_child_class()) {
        last_error_ = "Unable to register the WebView2 child window class.";
        emit_signal("load_error", last_error_);
        return false;
    }
    if (native_->composition_mode) {
        if (native_->child_window == nullptr) {
            native_->child_window = create_webview_host_window(native_->parent,
                    native_->bounds,
                    false,
                    false,
                    visible_,
                    native_->window_title);
            if (native_->child_window == nullptr) {
                last_error_ = "Unable to create the WebView2 composition host window.";
                emit_signal("load_error", last_error_);
                return false;
            }
            g_webview_probe_colors[native_->child_window] = RGB(15, 118, 110);
            InvalidateRect(native_->child_window, nullptr, TRUE);
            UpdateWindow(native_->child_window);
        }
        move_webview_child_window(native_->child_window, native_->parent, native_->bounds, native_->overlay_window);
        HRESULT dcomp_hr = setup_direct_composition(native_->child_window,
                native_->dcomp_device,
                native_->dcomp_target,
                native_->dcomp_root_visual,
                native_->dcomp_webview_visual);
        if (FAILED(dcomp_hr)) {
            last_error_ = wide_to_godot(hresult_text("DCompositionCreateDevice/CreateTarget", dcomp_hr));
            emit_signal("load_error", last_error_);
            return false;
        }
        position_composition_visual(native_->dcomp_device.Get(), native_->dcomp_webview_visual.Get(), zero_origin_bounds(native_->bounds));
    } else if (native_->child_window == nullptr) {
        native_->child_window = create_webview_host_window(
                native_->parent,
                native_->bounds,
                native_->detached_window,
                native_->overlay_window,
                visible_,
                native_->window_title);
        if (native_->child_window == nullptr) {
            last_error_ = "Unable to create the WebView2 child window.";
            emit_signal("load_error", last_error_);
            return false;
        }
    }
    if (!native_->detached_window) {
        move_webview_child_window(native_->child_window, native_->parent, native_->bounds, native_->overlay_window);
    }

    std::vector<std::wstring> loader_paths;
    const std::wstring mod_dir = module_directory();
    if (!mod_dir.empty()) {
        loader_paths.push_back(mod_dir + L"\\WebView2Loader.dll");
        loader_paths.push_back(directory_of(mod_dir) + L"\\third_party\\webview2\\WebView2Loader.dll");
    }
    loader_paths.push_back(L"WebView2Loader.dll");

    for (const std::wstring &path : loader_paths) {
        native_->loader = LoadLibraryW(path.c_str());
        if (native_->loader != nullptr) {
            webview_debug_log("loaded WebView2Loader from %s", wide_to_utf8(path).c_str());
            break;
        }
    }
    if (native_->loader == nullptr) {
        last_error_ = "WebView2Loader.dll was not found. Copy it to extension/bin or install the WebView2 SDK.";
        emit_signal("load_error", last_error_);
        return false;
    }

    native_->create_environment = reinterpret_cast<NativeState::CreateEnvironmentFn>(
            GetProcAddress(native_->loader, "CreateCoreWebView2EnvironmentWithOptions"));
    if (native_->create_environment == nullptr) {
        last_error_ = "WebView2Loader.dll does not export CreateCoreWebView2EnvironmentWithOptions.";
        emit_signal("load_error", last_error_);
        return false;
    }

    bool completed = false;
    std::wstring async_error;
    const std::wstring user_data = scoped_user_data_dir(native_->user_data_subdir);
    webview_debug_log("creating environment user_data=%s", wide_to_utf8(user_data).c_str());
    hr = native_->create_environment(nullptr, user_data.c_str(), nullptr,
            Callback<ICoreWebView2CreateCoreWebView2EnvironmentCompletedHandler>(
                    [this, &completed, &async_error](HRESULT result, ICoreWebView2Environment *environment) -> HRESULT {
                        webview_debug_log("environment callback result=0x%08X environment=%p", static_cast<unsigned int>(result), environment);
                        if (FAILED(result) || environment == nullptr) {
                            async_error = hresult_text("CreateCoreWebView2EnvironmentWithOptions", result);
                            completed = true;
                            return S_OK;
                        }
                        native_->environment = environment;
                        if (native_->composition_mode) {
                            ComPtr<ICoreWebView2Environment3> environment3;
                            HRESULT qi_environment = native_->environment.As(&environment3);
                            if (FAILED(qi_environment) || !environment3) {
                                async_error = hresult_text("ICoreWebView2Environment3", qi_environment);
                                completed = true;
                                return S_OK;
                            }
                            HWND composition_parent = native_->child_window != nullptr ? native_->child_window : native_->parent;
                            HRESULT hr_composition = environment3->CreateCoreWebView2CompositionController(composition_parent,
                                    Callback<ICoreWebView2CreateCoreWebView2CompositionControllerCompletedHandler>(
                                            [this, &completed, &async_error](HRESULT controller_result, ICoreWebView2CompositionController *composition_controller) -> HRESULT {
                                                if (FAILED(controller_result) || composition_controller == nullptr) {
                                                    async_error = hresult_text("CreateCoreWebView2CompositionController", controller_result);
                                                    completed = true;
                                                    return S_OK;
                                                }
                                                native_->composition_controller = composition_controller;
                                                if (native_->dcomp_webview_visual) {
                                                    HRESULT root_hr = native_->composition_controller->put_RootVisualTarget(native_->dcomp_webview_visual.Get());
                                                    if (FAILED(root_hr)) {
                                                        async_error = hresult_text("CompositionController::put_RootVisualTarget", root_hr);
                                                        completed = true;
                                                        return S_OK;
                                                    }
                                                }
                                                HRESULT qi_controller = native_->composition_controller.As(&native_->controller);
                                                if (FAILED(qi_controller) || !native_->controller) {
                                                    async_error = hresult_text("CompositionController::ICoreWebView2Controller", qi_controller);
                                                    completed = true;
                                                    return S_OK;
                                                }
                                                native_->controller->get_CoreWebView2(&native_->webview);
                                                RECT controller_bounds = zero_origin_bounds(native_->bounds);
                                                native_->controller->put_Bounds(controller_bounds);
                                                native_->controller->put_IsVisible(visible_ ? TRUE : FALSE);
                                                position_composition_visual(native_->dcomp_device.Get(), native_->dcomp_webview_visual.Get(), zero_origin_bounds(native_->bounds));
                                                ready_ = native_->webview != nullptr;
                                                last_error_ = "";
                                                completed = true;
                                                emit_signal("ready");
                                                return S_OK;
                                            })
                                            .Get());
                            if (FAILED(hr_composition)) {
                                async_error = hresult_text("CreateCoreWebView2CompositionController", hr_composition);
                                completed = true;
                            }
                            return S_OK;
                        }
                        HWND controller_parent = native_->child_window != nullptr ? native_->child_window : native_->parent;
                        webview_debug_log("creating controller controller_parent=%p child=%p parent=%p",
                                controller_parent,
                                native_->child_window,
                                native_->parent);
                        HRESULT hr_controller = native_->environment->CreateCoreWebView2Controller(controller_parent,
                                Callback<ICoreWebView2CreateCoreWebView2ControllerCompletedHandler>(
                                        [this, &completed, &async_error](HRESULT controller_result, ICoreWebView2Controller *controller) -> HRESULT {
                                            webview_debug_log("controller callback result=0x%08X controller=%p", static_cast<unsigned int>(controller_result), controller);
                                            if (FAILED(controller_result) || controller == nullptr) {
                                                async_error = hresult_text("CreateCoreWebView2Controller", controller_result);
                                                completed = true;
                                                return S_OK;
                                            }
                                             native_->controller = controller;
                                             native_->controller->get_CoreWebView2(&native_->webview);
                                             RECT controller_bounds = controller_bounds_for_child(native_->child_window, native_->bounds, native_->detached_window);
                                             native_->controller->put_Bounds(controller_bounds);
                                             native_->controller->put_IsVisible(visible_ ? TRUE : FALSE);
                                             g_webview_controllers[native_->child_window] = native_->controller.Get();
                                             if (!native_->detached_window) {
                                                 move_webview_child_window(native_->child_window, native_->parent, native_->bounds, native_->overlay_window);
                                             }
                                             HWND parent_hwnd = native_->child_window != nullptr ? native_->child_window : native_->parent;
                                            if (parent_hwnd != nullptr) {
                                                RedrawWindow(parent_hwnd, nullptr, nullptr, RDW_INVALIDATE | RDW_ALLCHILDREN | RDW_UPDATENOW);
                                            }
                                            if (native_->webview) {
                                                EventRegistrationToken token;
                                                native_->webview->add_NavigationStarting(
                                                        Callback<ICoreWebView2NavigationStartingEventHandler>(
                                                                [this](ICoreWebView2 *, ICoreWebView2NavigationStartingEventArgs *args) -> HRESULT {
                                                                    LPWSTR uri = nullptr;
                                                                    if (args != nullptr && SUCCEEDED(args->get_Uri(&uri)) && uri != nullptr) {
                                                                        current_url_ = wide_to_godot(take_string(uri));
                                                                        CoTaskMemFree(uri);
                                                                    }
                                                                    Dictionary state;
                                                                    state["url"] = current_url_;
                                                                    state["loading"] = true;
                                                                    append_navigation_capabilities(native_->webview.Get(), state);
                                                                    emit_signal("navigation_started", current_url_);
                                                                    emit_signal("page_state_changed", state);
                                                                    return S_OK;
                                                                })
                                                                .Get(),
                                                        &token);
                                                native_->webview->add_NavigationCompleted(
                                                        Callback<ICoreWebView2NavigationCompletedEventHandler>(
                                                                [this](ICoreWebView2 *, ICoreWebView2NavigationCompletedEventArgs *) -> HRESULT {
                                                                    Dictionary state;
                                                                    LPWSTR source = nullptr;
                                                                    LPWSTR title = nullptr;
                                                                    if (native_->webview && SUCCEEDED(native_->webview->get_Source(&source)) && source != nullptr) {
                                                                        current_url_ = wide_to_godot(take_string(source));
                                                                        CoTaskMemFree(source);
                                                                    }
                                                                    if (native_->webview && SUCCEEDED(native_->webview->get_DocumentTitle(&title)) && title != nullptr) {
                                                                        state["title"] = wide_to_godot(take_string(title));
                                                                        CoTaskMemFree(title);
                                                                    }
                                                                    state["url"] = current_url_;
                                                                    state["loading"] = false;
                                                                    append_navigation_capabilities(native_->webview.Get(), state);
                                                                    emit_signal("page_state_changed", state);
                                                                    return S_OK;
                                                                })
                                                                .Get(),
                                                        &token);
                                                native_->webview->add_DocumentTitleChanged(
                                                        Callback<ICoreWebView2DocumentTitleChangedEventHandler>(
                                                                [this](ICoreWebView2 *, IUnknown *) -> HRESULT {
                                                                    Dictionary state;
                                                                    LPWSTR title = nullptr;
                                                                    if (native_->webview && SUCCEEDED(native_->webview->get_DocumentTitle(&title)) && title != nullptr) {
                                                                        state["title"] = wide_to_godot(take_string(title));
                                                                        CoTaskMemFree(title);
                                                                    }
                                                                    state["url"] = current_url_;
                                                                    state["loading"] = false;
                                                                    append_navigation_capabilities(native_->webview.Get(), state);
                                                                    emit_signal("page_state_changed", state);
                                                                    return S_OK;
                                                                })
                                                                .Get(),
                                                        &token);
                                                native_->webview->add_WebMessageReceived(
                                                        Callback<ICoreWebView2WebMessageReceivedEventHandler>(
                                                                [this](ICoreWebView2 *, ICoreWebView2WebMessageReceivedEventArgs *args) -> HRESULT {
                                                                    LPWSTR json = nullptr;
                                                                    if (args != nullptr && SUCCEEDED(args->get_WebMessageAsJson(&json)) && json != nullptr) {
                                                                        String message = wide_to_godot(take_string(json));
                                                                        CoTaskMemFree(json);
                                                                        emit_signal("web_message_received", message);
                                                                    }
                                                                    return S_OK;
                                                                })
                                                                .Get(),
                                                        &token);
                                            }
                                            ready_ = native_->webview != nullptr;
                                            last_error_ = "";
                                            completed = true;
                                            webview_debug_log("ready=%d parent=%p child=%p webview=%p",
                                                    ready_ ? 1 : 0,
                                                    native_->parent,
                                                    native_->child_window,
                                                    native_->webview.Get());
                                            emit_signal("ready");
                                            return S_OK;
                                        })
                                        .Get());
                        if (FAILED(hr_controller)) {
                            async_error = hresult_text("CreateCoreWebView2Controller", hr_controller);
                            completed = true;
                        }
                        return S_OK;
                    })
                    .Get());
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("CreateCoreWebView2EnvironmentWithOptions", hr));
        emit_signal("load_error", last_error_);
        return false;
    }

    pump_until([&completed]() { return completed; }, 10000);
    if (!async_error.empty()) {
        last_error_ = wide_to_godot(async_error);
        emit_signal("load_error", last_error_);
        return false;
    }
    if (!ready_) {
        last_error_ = "Timed out while initializing WebView2.";
        emit_signal("load_error", last_error_);
        return false;
    }
    return true;
}

void VitWebViewHost::navigate(String url) {
    current_url_ = url;
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    HRESULT hr = native_->webview->Navigate(godot_to_wide(url).c_str());
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("Navigate", hr));
        emit_signal("load_error", last_error_);
        return;
    }
    emit_signal("navigation_started", current_url_);
}

void VitWebViewHost::navigate_html(String html) {
    current_url_ = "vit://embedded-webview-test";
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    ComPtr<ICoreWebView2_2> webview2;
    HRESULT qi = native_->webview.As(&webview2);
    if (FAILED(qi) || !webview2) {
        last_error_ = wide_to_godot(hresult_text("ICoreWebView2_2::NavigateToString", qi));
        emit_signal("load_error", last_error_);
        return;
    }
    HRESULT hr = webview2->NavigateToString(godot_to_wide(html).c_str());
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("NavigateToString", hr));
        emit_signal("load_error", last_error_);
        return;
    }
    emit_signal("navigation_started", current_url_);
}

void VitWebViewHost::go_back() {
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    BOOL can_go_back = FALSE;
    if (SUCCEEDED(native_->webview->get_CanGoBack(&can_go_back)) && can_go_back) {
        native_->webview->GoBack();
    }
}

void VitWebViewHost::go_forward() {
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    BOOL can_go_forward = FALSE;
    if (SUCCEEDED(native_->webview->get_CanGoForward(&can_go_forward)) && can_go_forward) {
        native_->webview->GoForward();
    }
}

void VitWebViewHost::reload() {
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    native_->webview->Reload();
}

void VitWebViewHost::stop() {
    if (ready_ && native_ != nullptr && native_->webview) {
        native_->webview->Stop();
    }
    Dictionary state;
    state["url"] = current_url_;
    state["loading"] = false;
    append_navigation_capabilities(native_ != nullptr ? native_->webview.Get() : nullptr, state);
    emit_signal("page_state_changed", state);
}

Dictionary VitWebViewHost::debug_state() const {
    Dictionary state;
    state["ready"] = ready_;
    state["visible"] = visible_;
    state["last_error"] = last_error_;
    state["current_url"] = current_url_;
    if (native_ == nullptr) {
        return state;
    }
    state["parent_hwnd"] = static_cast<int64_t>(reinterpret_cast<intptr_t>(native_->parent));
    state["child_hwnd"] = static_cast<int64_t>(reinterpret_cast<intptr_t>(native_->child_window));
    state["composition_mode"] = native_->composition_mode;
    state["detached_window"] = native_->detached_window;
    state["overlay_window"] = native_->overlay_window;
    state["user_data_subdir"] = wide_to_godot(native_->user_data_subdir);
    state["controller"] = native_->controller ? true : false;
    state["composition_controller"] = native_->composition_controller ? true : false;
    state["dcomp_device"] = native_->dcomp_device ? true : false;
    state["dcomp_target"] = native_->dcomp_target ? true : false;
    state["dcomp_root_visual"] = native_->dcomp_root_visual ? true : false;
    state["dcomp_webview_visual"] = native_->dcomp_webview_visual ? true : false;
    state["webview"] = native_->webview ? true : false;
    Dictionary bounds;
    bounds["left"] = native_->bounds.left;
    bounds["top"] = native_->bounds.top;
    bounds["right"] = native_->bounds.right;
    bounds["bottom"] = native_->bounds.bottom;
    state["bounds"] = bounds;
    RECT controller_rect = native_->composition_mode
            ? zero_origin_bounds(native_->bounds)
            : controller_bounds_for_child(native_->child_window, native_->bounds, native_->detached_window);
    Dictionary controller_bounds;
    controller_bounds["left"] = controller_rect.left;
    controller_bounds["top"] = controller_rect.top;
    controller_bounds["right"] = controller_rect.right;
    controller_bounds["bottom"] = controller_rect.bottom;
    state["controller_bounds"] = controller_bounds;
    if (native_->child_window != nullptr) {
        RECT child_rect = {};
        GetWindowRect(native_->child_window, &child_rect);
        Dictionary child;
        child["left"] = child_rect.left;
        child["top"] = child_rect.top;
        child["right"] = child_rect.right;
        child["bottom"] = child_rect.bottom;
        child["visible"] = IsWindowVisible(native_->child_window) ? true : false;
        child["parent"] = static_cast<int64_t>(reinterpret_cast<intptr_t>(GetParent(native_->child_window)));
        child["probe_color"] = g_webview_probe_colors.find(native_->child_window) != g_webview_probe_colors.end();
        state["child_rect"] = child;
        RECT child_client = {};
        if (GetClientRect(native_->child_window, &child_client)) {
            Dictionary client;
            client["left"] = child_client.left;
            client["top"] = child_client.top;
            client["right"] = child_client.right;
            client["bottom"] = child_client.bottom;
            state["child_client_rect"] = client;
        }
        state["child_dpi"] = static_cast<int>(GetDpiForWindow(native_->child_window));
    }
    return state;
}

Variant VitWebViewHost::eval_js(String script) {
    if (!ready_ || native_ == nullptr || !native_->webview) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return Variant();
    }

    bool completed = false;
    HRESULT script_result = S_OK;
    std::wstring raw_result;
    HRESULT hr = native_->webview->ExecuteScript(godot_to_wide(script).c_str(),
            Callback<ICoreWebView2ExecuteScriptCompletedHandler>(
                    [&completed, &script_result, &raw_result](HRESULT error_code, LPCWSTR result_object_as_json) -> HRESULT {
                        script_result = error_code;
                        if (result_object_as_json != nullptr) {
                            raw_result = result_object_as_json;
                        }
                        completed = true;
                        return S_OK;
                    })
                    .Get());
    if (FAILED(hr)) {
        last_error_ = wide_to_godot(hresult_text("ExecuteScript", hr));
        emit_signal("load_error", last_error_);
        return Variant();
    }
    if (!pump_until([&completed]() { return completed; }, 5000)) {
        last_error_ = "Timed out while executing JavaScript in WebView2.";
        emit_signal("load_error", last_error_);
        return Variant();
    }
    if (FAILED(script_result)) {
        last_error_ = wide_to_godot(hresult_text("ExecuteScript callback", script_result));
        emit_signal("load_error", last_error_);
        return Variant();
    }
    std::string utf8 = wide_to_utf8(raw_result);
    utf8 = json_string_unescape(utf8);
    return String::utf8(utf8.c_str());
}

} // namespace godot

#else

namespace godot {

VitWebViewHost::VitWebViewHost() = default;

VitWebViewHost::~VitWebViewHost() = default;

bool VitWebViewHost::create(int64_t parent_hwnd) {
    (void)parent_hwnd;
    ready_ = false;
    last_error_ = "VitWebViewHost native WebView2 backend is not linked in this build.";
    emit_signal("load_error", last_error_);
    return false;
}

bool VitWebViewHost::create_window(int64_t parent_hwnd, String title, int width, int height) {
    (void)parent_hwnd;
    (void)title;
    (void)width;
    (void)height;
    return create(parent_hwnd);
}

bool VitWebViewHost::create_overlay_window(int64_t parent_hwnd, String title, int width, int height) {
    (void)title;
    (void)width;
    (void)height;
    return create(parent_hwnd);
}

bool VitWebViewHost::create_composition(int64_t parent_hwnd) {
    return create(parent_hwnd);
}

bool VitWebViewHost::create_probe(int64_t parent_hwnd) {
    return create(parent_hwnd);
}

void VitWebViewHost::close() {
    ready_ = false;
    visible_ = false;
    current_url_ = "";
    last_error_ = "";
}

void VitWebViewHost::set_bounds(int x, int y, int width, int height) {
    (void)x;
    (void)y;
    (void)width;
    (void)height;
}

void VitWebViewHost::set_screen_bounds(int x, int y, int width, int height) {
    (void)x;
    (void)y;
    (void)width;
    (void)height;
}

void VitWebViewHost::set_visible(bool visible) {
    visible_ = visible;
    (void)visible_;
}

void VitWebViewHost::focus() {
}

bool VitWebViewHost::send_mouse_input(String event_kind, int virtual_keys, int mouse_data, int x, int y) {
    (void)event_kind;
    (void)virtual_keys;
    (void)mouse_data;
    (void)x;
    (void)y;
    return false;
}

void VitWebViewHost::navigate(String url) {
    current_url_ = url;
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
        return;
    }
    emit_signal("navigation_started", current_url_);
    Dictionary state;
    state["url"] = current_url_;
    state["title"] = current_url_;
    state["loading"] = false;
    emit_signal("page_state_changed", state);
}

void VitWebViewHost::navigate_html(String html) {
    (void)html;
    navigate("vit://embedded-webview-test");
}

void VitWebViewHost::go_back() {
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
    }
}

void VitWebViewHost::go_forward() {
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
    }
}

void VitWebViewHost::reload() {
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
    }
}

void VitWebViewHost::stop() {
    Dictionary state;
    state["url"] = current_url_;
    state["loading"] = false;
    emit_signal("page_state_changed", state);
}

Dictionary VitWebViewHost::debug_state() const {
    Dictionary state;
    state["ready"] = ready_;
    state["visible"] = visible_;
    state["last_error"] = last_error_;
    state["current_url"] = current_url_;
    state["linked"] = false;
    return state;
}

Variant VitWebViewHost::eval_js(String script) {
    (void)script;
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
    }
    return Variant();
}

bool VitWebViewHost::post_web_message(String message) {
    (void)message;
    if (!ready_) {
        emit_signal("load_error", last_error_.is_empty() ? String("WebView2 host is not ready.") : last_error_);
    }
    return false;
}

void VitWebViewHost::set_user_data_subdir(String subdir) {
    (void)subdir;
}

} // namespace godot

#endif
