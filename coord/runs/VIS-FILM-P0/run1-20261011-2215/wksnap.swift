// wksnap.swift — VIS-FILM-P0 read-image loop: real-screenshot producer.
// Loads a URL in an offscreen WKWebView (window placed far outside any display,
// activation policy .prohibited, no input events, no desktop screen capture)
// and snapshots the web content to PNG. macOS-native equivalent of a headless
// browser screenshot; does not touch the user's visible desktop (AGENTS §5).
import AppKit
import WebKit

let args = CommandLine.arguments
guard args.count >= 5 else {
    print("usage: wksnap <url> <out.png> <width> <height> [settleMs]")
    exit(2)
}
let urlStr = args[1], outPath = args[2]
let w = Int(args[3]) ?? 1280
let h = Int(args[4]) ?? 720
let settleMs = args.count >= 6 ? Int(args[5]) ?? 8000 : 8000

let app = NSApplication.shared
app.setActivationPolicy(.prohibited)

let rect = NSRect(x: 0, y: 0, width: w, height: h)
let window = NSWindow(contentRect: rect, styleMask: [.borderless], backing: .buffered, defer: false)
window.isOpaque = true
window.backgroundColor = NSColor.white
window.setFrameOrigin(NSPoint(x: -10000, y: -10000))

let webview = WKWebView(frame: rect, configuration: WKWebViewConfiguration())
window.contentView = webview
// Keep the offscreen window in the WindowServer compositing tree from the start
// so WebKit actually renders (window sits far outside any display; nothing shows).
window.orderFrontRegardless()

final class NavDelegate: NSObject, WKNavigationDelegate {
    var finished = false
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) { finished = true }
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        print("NAV_FAIL: \(error.localizedDescription)")
        finished = true
    }
}
let nav = NavDelegate()
webview.navigationDelegate = nav
webview.load(URLRequest(url: URL(string: urlStr)!))

let start = Date()
var loadedAt: Date? = nil
while true {
    RunLoop.current.run(until: Date().addingTimeInterval(0.05))
    if nav.finished && loadedAt == nil { loadedAt = Date() }
    if let la = loadedAt, Date().timeIntervalSince(la) * 1000.0 >= Double(settleMs) { break }
    if Date().timeIntervalSince(start) > 90 {
        print("TIMEOUT waiting for page load+settle")
        exit(3)
    }
}
// (window was ordered front at creation; offscreen origin keeps it invisible)

let sem = DispatchSemaphore(value: 0)
let snapCfg = WKSnapshotConfiguration()
snapCfg.rect = CGRect(x: 0, y: 0, width: w, height: h)
var snapStarted = false
webview.takeSnapshot(with: snapCfg) { image, error in
    defer { sem.signal() }
    guard let image = image else {
        print("SNAPSHOT_ERR: \(String(describing: error))")
        exit(4)
    }
    guard let tiff = image.tiffRepresentation,
          let rep = NSBitmapImageRep(data: tiff),
          let png = rep.representation(using: .png, properties: [:]) else {
        print("PNG_ENCODE_ERR")
        exit(5)
    }
    do {
        try png.write(to: URL(fileURLWithPath: outPath))
        print("WROTE \(outPath) size=\(w)x\(h) bytes=\(png.count)")
    } catch {
        print("WRITE_ERR: \(error)")
        exit(6)
    }
}
snapStarted = true
// Pump the run loop (main queue) while waiting; a bare sem.wait() would deadlock
// the takeSnapshot completion handler.
let snapDeadline = Date().addingTimeInterval(90)
while Date() < snapDeadline {
    RunLoop.current.run(until: Date().addingTimeInterval(0.05))
    if sem.wait(timeout: .now()) == .success { snapStarted = false; break }
}
if snapStarted {
    print("SNAPSHOT_TIMEOUT")
    exit(7)
}
