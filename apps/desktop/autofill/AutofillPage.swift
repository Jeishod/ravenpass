import AppKit
import UniformTypeIdentifiers
import WebKit

/// A request of the page: the operation it names, with the JSON object that holds its fields.
struct PageRequest {
    let op: String
    fileprivate let id: Int
    fileprivate let body: Data

    /// The request's fields as T, or nil when they are not.
    func fields<T: Decodable>(_ type: T.Type) -> T? {
        try? JSONDecoder().decode(type, from: body)
    }
}

/// What the page shows while a request waits on the running app.
enum PageWait {
    case opening
    case unlocking
    case verifying
    case failed(PageFailure)
}

/// Why Ravenpass stopped a request the page shows no list for, as the page names it.
enum PageFailure: String {
    case unreachable
    case outdated
    case fillFailed = "fill-failed"
    case codeFailed = "code-failed"
    case signInFailed = "sign-in-failed"
    case saveFailed = "save-failed"
    case unverifiable
    case unsupported
    case failed
}

/// Receives the page's requests on the main thread while the page lives.
@MainActor
protocol AutofillPageDelegate: AnyObject {
    func page(_ page: AutofillPage, received request: PageRequest)

    /// The page did not load, or its web content process ended and took it along.
    func pageLost(_ page: AutofillPage)
}

/// The shared interface's autofill page, confined to the extension bundle and hidden until it reports ready.
@MainActor
final class AutofillPage: NSObject, WKNavigationDelegate, WKUIDelegate {
    private static let handler = "ravenpassAutofill"
    private static let address = URL(string: "\(PageOrigin.scheme)://\(PageOrigin.host)/autofill.html")!

    let view: WKWebView
    weak var delegate: AutofillPageDelegate?
    private var ready = false
    private var navigated = false
    /// The latest wait, as JSON text, to deliver once the page is ready.
    private var pending: String?

    init(files: URL) {
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .nonPersistent()
        configuration.setURLSchemeHandler(PageFiles(root: files), forURLScheme: PageOrigin.scheme)
        configuration.preferences.javaScriptCanOpenWindowsAutomatically = false
        configuration.preferences.isElementFullscreenEnabled = false
        configuration.mediaTypesRequiringUserActionForPlayback = .all
        view = WKWebView(frame: .zero, configuration: configuration)
        super.init()
        configuration.userContentController.add(MessageRelay(self), name: Self.handler)
        view.navigationDelegate = self
        view.uiDelegate = self
        view.allowsBackForwardNavigationGestures = false
        view.allowsMagnification = false
        view.allowsLinkPreview = false
        view.isInspectable = false
        view.isHidden = true
    }

    func load() {
        view.load(URLRequest(url: Self.address))
    }

    /// Answers a request; a request sent as a notice, with id 0, takes no answer.
    func answer(_ request: PageRequest, with answer: some Encodable) {
        guard request.id > 0, let text = Self.json(answer) else { return }
        call("window.ravenpassAutofillAnswer?.(id, answer)", ["id": request.id, "answer": text])
    }

    func answer(_ request: PageRequest, status: String) {
        answer(request, with: Status(status: status))
    }

    /// Shows wait in place of the page's screen, or the screen again for nil.
    func show(_ wait: PageWait?) {
        guard let text = Self.json(WaitNotice(wait)) else { return }
        if ready {
            call("window.ravenpassAutofillNotice?.(notice)", ["notice": text])
        } else {
            pending = text
        }
    }

    private func call(_ body: String, _ arguments: [String: Any]) {
        view.callAsyncJavaScript(body, arguments: arguments, in: nil, in: .page) { _ in }
    }

    fileprivate func receive(_ message: WKScriptMessage) {
        guard message.frameInfo.isMainFrame,
              message.frameInfo.securityOrigin.protocol == PageOrigin.scheme,
              let body = message.body as? [String: Any],
              let id = body["id"] as? Int,
              let text = body["request"] as? String,
              let data = text.data(using: .utf8),
              let fields = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let op = fields["op"] as? String
        else {
            return
        }
        if op == "ready" {
            becomeReady()
        }
        delegate?.page(self, received: PageRequest(op: op, id: id, body: data))
    }

    private func becomeReady() {
        guard !ready else { return }
        ready = true
        view.isHidden = false
        view.window?.makeFirstResponder(view)
        if let pending {
            call("window.ravenpassAutofillNotice?.(notice)", ["notice": pending])
        }
        pending = nil
    }

    private static func json(_ value: some Encodable) -> String? {
        guard let data = try? JSONEncoder().encode(value) else { return nil }
        return String(data: data, encoding: .utf8)
    }

    /// Allows the one load of the page, in the main frame, and cancels every other navigation.
    func webView(
        _ webView: WKWebView, decidePolicyFor action: WKNavigationAction,
        decisionHandler: @escaping @MainActor (WKNavigationActionPolicy) -> Void
    ) {
        guard !navigated, action.targetFrame?.isMainFrame == true, action.request.url == Self.address else {
            return decisionHandler(.cancel)
        }
        navigated = true
        decisionHandler(.allow)
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        delegate?.pageLost(self)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        delegate?.pageLost(self)
    }

    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        delegate?.pageLost(self)
    }

    func webView(
        _ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
        for action: WKNavigationAction, windowFeatures: WKWindowFeatures
    ) -> WKWebView? {
        nil
    }
}

/// Where the page's files are served: a scheme WebKit does not handle itself, and one host.
private enum PageOrigin {
    static let scheme = "ravenpass-autofill"
    static let host = "page"
}

/// Holds the page weakly: WKUserContentController retains its message handlers strongly.
@MainActor
private final class MessageRelay: NSObject, WKScriptMessageHandler {
    private weak var page: AutofillPage?

    init(_ page: AutofillPage) {
        self.page = page
    }

    func userContentController(_ controller: WKUserContentController, didReceive message: WKScriptMessage) {
        page?.receive(message)
    }
}

/// An answer that only says how the request ended.
private struct Status: Encodable {
    let status: String
}

/// A wait as the page reads it; an empty wait ends the one shown.
private struct WaitNotice: Encodable {
    let status = "wait"
    let wait: String
    let failure: String?

    init(_ wait: PageWait?) {
        switch wait {
        case nil: self.wait = ""; failure = nil
        case .opening?: self.wait = "opening"; failure = nil
        case .unlocking?: self.wait = "unlocking"; failure = nil
        case .verifying?: self.wait = "verifying"; failure = nil
        case .failed(let reason)?: self.wait = "failed"; failure = reason.rawValue
        }
    }
}

/// Serves the page's files from the extension's bundle, each with a policy that confines the page to them.
@MainActor
private final class PageFiles: NSObject, WKURLSchemeHandler {
    private static let policy = [
        "default-src 'none'",
        "script-src \(PageOrigin.scheme):",
        "style-src \(PageOrigin.scheme): 'unsafe-inline'",
        "img-src \(PageOrigin.scheme): data:",
        "font-src \(PageOrigin.scheme): data:",
        "connect-src 'none'",
        "base-uri 'none'",
        "form-action 'none'",
        "frame-ancestors 'none'",
    ].joined(separator: "; ")

    private let root: URL

    init(root: URL) {
        self.root = root.standardizedFileURL
    }

    func webView(_ webView: WKWebView, start task: any WKURLSchemeTask) {
        guard let url = task.request.url, let file = file(at: url), let data = try? Data(contentsOf: file) else {
            return task.didFailWithError(URLError(.fileDoesNotExist))
        }
        let type = UTType(filenameExtension: file.pathExtension)?.preferredMIMEType ?? "application/octet-stream"
        let headers = [
            "Content-Type": type,
            "Content-Length": String(data.count),
            "Content-Security-Policy": Self.policy,
            "X-Content-Type-Options": "nosniff",
            "Cache-Control": "no-store",
        ]
        guard let response = HTTPURLResponse(url: url, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: headers) else {
            return task.didFailWithError(URLError(.cannotParseResponse))
        }
        task.didReceive(response)
        task.didReceive(data)
        task.didFinish()
    }

    func webView(_ webView: WKWebView, stop task: any WKURLSchemeTask) {}

    /// The file url names inside root, or nil for one outside it or for another scheme or host.
    private func file(at url: URL) -> URL? {
        guard url.scheme == PageOrigin.scheme, url.host() == PageOrigin.host else { return nil }
        let file = root.appending(path: url.path(percentEncoded: false)).standardizedFileURL
        guard file.path.hasPrefix(root.path + "/") else { return nil }
        return file
    }
}
