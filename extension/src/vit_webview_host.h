#ifndef VIT_WEBVIEW_HOST_H
#define VIT_WEBVIEW_HOST_H

#include <godot_cpp/classes/ref_counted.hpp>
#include <godot_cpp/core/class_db.hpp>
#include <godot_cpp/variant/dictionary.hpp>
#include <godot_cpp/variant/string.hpp>
#include <godot_cpp/variant/variant.hpp>

#include <cstdint>

namespace godot {

class VitWebViewHost : public RefCounted {
    GDCLASS(VitWebViewHost, RefCounted)

protected:
    static void _bind_methods();

public:
    VitWebViewHost();
    ~VitWebViewHost();

    bool create(int64_t parent_hwnd);
    bool create_window(int64_t parent_hwnd, String title, int width, int height);
    bool create_overlay_window(int64_t parent_hwnd, String title, int width, int height);
    bool create_composition(int64_t parent_hwnd);
    bool create_probe(int64_t parent_hwnd);
    void close();
    void set_bounds(int x, int y, int width, int height);
    void set_screen_bounds(int x, int y, int width, int height);
    void set_visible(bool visible);
    void focus();
    bool send_mouse_input(String event_kind, int virtual_keys, int mouse_data, int x, int y);
    void navigate(String url);
    void navigate_html(String html);
    void go_back();
    void go_forward();
    void reload();
    void stop();
    Variant eval_js(String script);
    bool post_web_message(String message);
    void set_user_data_subdir(String subdir);
    Dictionary debug_state() const;

private:
    struct NativeState;
    NativeState *native_ = nullptr;
    bool ready_ = false;
    bool visible_ = false;
    String current_url_;
    String last_error_;
};

} // namespace godot

#endif // VIT_WEBVIEW_HOST_H
