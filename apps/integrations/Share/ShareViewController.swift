import AppKit

final class ShareViewController: NSViewController {
    private let status = NSTextField(wrappingLabelWithString: "Reading shared links…")
    private let progress = NSProgressIndicator()
    private var started = false
    private var finished = false

    override func loadView() {
        view = NSView(frame: NSRect(x: 0, y: 0, width: 360, height: 130))
        let heading = NSTextField(labelWithString: "Open with Waystile")
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
        await ShareLinkLoader.urls(
            from: (extensionContext?.inputItems ?? []).compactMap { $0 as? NSExtensionItem },
            isCancelled: { self.finished }
        )
    }

    @objc private func cancelRequest() {
        guard !finished else { return }
        finished = true
        extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
    }
}
