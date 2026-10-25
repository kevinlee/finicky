import AppKit
import SafariServices
import os

final class SafariExtensionHandler: SFSafariExtensionHandler {
    private let logger = Logger(subsystem: "com.kevinlee.finicky.actions.safari", category: "Links")

    override func toolbarItemClicked(in window: SFSafariWindow) {
        window.getActiveTab { tab in
            tab?.getActivePage { page in
                page?.getPropertiesWithCompletionHandler { properties in
                    guard let url = properties?.url, FinickyBridge.isWebURL(url) else { return }
                    self.open(url)
                }
            }
        }
    }

    override func validateToolbarItem(in window: SFSafariWindow, validationHandler: @escaping (Bool, String) -> Void) {
        window.getActiveTab { tab in
            guard let tab else { validationHandler(false, ""); return }
            tab.getActivePage { page in
                guard let page else { validationHandler(false, ""); return }
                page.getPropertiesWithCompletionHandler { properties in
                    validationHandler(properties?.url.map(FinickyBridge.isWebURL) ?? false, "")
                }
            }
        }
    }

    override func validateContextMenuItem(withCommand command: String, in page: SFSafariPage, userInfo: [String: Any]?, validationHandler: @escaping (Bool, String?) -> Void) {
        guard command == "open-link", let value = userInfo?["linkURL"] as? String,
              let url = URL(string: value), FinickyBridge.isWebURL(url) else {
            validationHandler(true, nil)
            return
        }
        validationHandler(false, nil)
    }

    override func contextMenuItemSelected(withCommand command: String, in page: SFSafariPage, userInfo: [String: Any]? = nil) {
        guard command == "open-link", let value = userInfo?["linkURL"] as? String,
              let url = URL(string: value), FinickyBridge.isWebURL(url) else { return }
        open(url)
    }

    private func open(_ url: URL) {
        Task { @MainActor in
            do { try FinickyBridge.open([url], prompt: true) }
            catch { logger.error("Could not hand a link to Waystile: \(error.localizedDescription)") }
        }
    }
}
