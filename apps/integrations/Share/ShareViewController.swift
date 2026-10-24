import AppKit
import UniformTypeIdentifiers

final class ShareViewController: NSViewController {
    private let status = NSTextField(wrappingLabelWithString: "Reading shared links…")
    private let progress = NSProgressIndicator()
    private var started = false
    private var finished = false

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 360, height: 130))
        let heading = NSTextField(labelWithString: "Open with Finicky Plus")
        heading.font = .systemFont(ofSize: 17, weight: .semibold)
        status.textColor = .secondaryLabelColor
        progress.style = .spinning
        progress.controlSize = .small
        progress.startAnimation(nil)
        let cancel = NSButton(title: "Cancel", target: self, action: #selector(cancelRequest))
        cancel.bezelStyle = .rounded
        let controls = NSStackView(views: [progress, NSView(), cancel])
        controls.distribution = .fill
        let stack = NSStackView(views: [heading, status, controls])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 20),
            stack.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -20),
            stack.topAnchor.constraint(equalTo: view.topAnchor, constant: 20),
            status.widthAnchor.constraint(equalTo: stack.widthAnchor),
            controls.widthAnchor.constraint(equalTo: stack.widthAnchor)
        ])
    }

    override func viewDidAppear() {
        super.viewDidAppear()
        guard !started else { return }
        started = true
        Task { @MainActor in
            let urls = await collectURLs()
            guard !finished else { return }
            do {
                try FinickyBridge.open(urls, prompt: true)
                finished = true
                extensionContext?.completeRequest(returningItems: [], completionHandler: nil)
            } catch {
                progress.stopAnimation(nil)
                progress.isHidden = true
                status.stringValue = error.localizedDescription
                status.textColor = .labelColor
            }
        }
    }

    private func collectURLs() async -> [URL] {
        var found: [URL] = []
        for case let item as NSExtensionItem in extensionContext?.inputItems ?? [] {
            if let text = item.attributedContentText?.string {
                found.append(contentsOf: FinickyBridge.webURLs(in: text))
            }
            for provider in item.attachments ?? [] {
                guard !finished else { return [] }
                let type: String
                if provider.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
                    type = UTType.url.identifier
                } else if provider.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
                    type = UTType.plainText.identifier
                } else { continue }
                let value = await load(provider, type: type)
                if let url = value as? URL, FinickyBridge.isWebURL(url) {
                    found.append(url)
                } else if let string = value as? String {
                    found.append(contentsOf: FinickyBridge.webURLs(in: string))
                } else if let data = value as? Data, let string = String(data: data, encoding: .utf8) {
                    found.append(contentsOf: FinickyBridge.webURLs(in: string))
                }
            }
        }
        var seen = Set<String>()
        return found.filter { seen.insert($0.absoluteString).inserted }
    }

    private func load(_ provider: NSItemProvider, type: String) async -> NSSecureCoding? {
        await withCheckedContinuation { continuation in
            provider.loadItem(forTypeIdentifier: type, options: nil) { item, _ in
                continuation.resume(returning: item)
            }
        }
    }

    @objc private func cancelRequest() {
        guard !finished else { return }
        finished = true
        extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
    }
}
