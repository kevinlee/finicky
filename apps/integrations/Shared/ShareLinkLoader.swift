import AppKit
import UniformTypeIdentifiers

/// Extracts web destinations from shared content without opening files or fetching resources.
enum ShareLinkLoader {
    @MainActor
    static func urls(from items: [NSExtensionItem], isCancelled: () -> Bool = { false }) async -> [URL] {
        var found: [URL] = []
        for item in items {
            guard !isCancelled() else { return [] }
            if let content = item.attributedContentText {
                found.append(contentsOf: urls(in: content))
            }
            for provider in item.attachments ?? [] {
                guard !isCancelled() else { return [] }
                // Prefer a URL, then rich text (which preserves link labels),
                // then plain text. A failed or unusable representation must
                // not hide a usable alternative on the same provider.
                for type in supportedTypes(provider.registeredTypeIdentifiers) {
                    let value = await load(provider, type: type)
                    guard !isCancelled() else { return [] }
                    let links = urls(in: value, typeIdentifier: type)
                    if !links.isEmpty {
                        found.append(contentsOf: links)
                        break
                    }
                }
            }
        }
        return unique(found)
    }

    static func urls(in content: NSAttributedString) -> [URL] {
        let fullRange = NSRange(location: 0, length: content.length)
        var annotated: [(range: NSRange, url: URL)] = []
        content.enumerateAttribute(.link, in: fullRange) { value, range, _ in
            let url = (value as? URL) ?? (value as? String).flatMap(URL.init(string:))
            if let url, FinickyBridge.isWebURL(url) { annotated.append((range, url)) }
        }
        var links = annotated
        if let detector = try? NSDataDetector(types: NSTextCheckingResult.CheckingType.link.rawValue) {
            for match in detector.matches(in: content.string, range: fullRange) {
                guard let url = match.url, FinickyBridge.isWebURL(url),
                      !annotated.contains(where: { NSIntersectionRange($0.range, match.range).length > 0 }) else { continue }
                links.append((match.range, url))
            }
        }
        return unique(links.sorted { $0.range.location < $1.range.location }.map(\.url))
    }

    static func urls(in value: NSSecureCoding?, typeIdentifier: String) -> [URL] {
        guard supportedTypes([typeIdentifier]).count == 1 else { return [] }
        if let url = value as? URL { return FinickyBridge.isWebURL(url) ? [url] : [] }
        if let text = value as? NSAttributedString { return urls(in: text) }
        if let text = value as? String { return FinickyBridge.webURLs(in: text) }
        guard let data = value as? Data else { return [] }
        if UTType(typeIdentifier)?.conforms(to: .rtf) == true {
            guard let text = try? NSAttributedString(data: data, options: [.documentType: NSAttributedString.DocumentType.rtf], documentAttributes: nil) else { return [] }
            return urls(in: text)
        }
        guard let text = plainText(data, typeIdentifier: typeIdentifier) else { return [] }
        return FinickyBridge.webURLs(in: text)
    }

    static func supportedTypes(_ identifiers: [String]) -> [String] {
        func priority(_ identifier: String) -> Int? {
            guard let type = UTType(identifier) else { return nil }
            if type.conforms(to: .fileURL) { return nil }
            if type.conforms(to: .url) { return 0 }
            if type.conforms(to: .rtf) { return 1 }
            if type == .text { return 2 }
            if type.conforms(to: .plainText) { return 3 }
            // In particular, do not use AppKit's HTML importer: it can fetch
            // remote resources. Hosts can offer an accepted text alternative.
            return nil
        }
        return identifiers.enumerated().compactMap { index, identifier -> (Int, Int, String)? in
            priority(identifier).map { ($0, index, identifier) }
        }.sorted { $0.0 == $1.0 ? $0.1 < $1.1 : $0.0 < $1.0 }.map { $0.2 }
    }

    private static func plainText(_ data: Data, typeIdentifier: String) -> String? {
        // UTF-16 external text is big endian without a BOM; native UTF-16 is
        // little endian on supported Macs. A BOM takes precedence for either.
        if data.starts(with: [0xff, 0xfe]) || data.starts(with: [0xfe, 0xff]) {
            return String(data: data, encoding: .utf16)
        }
        if typeIdentifier == UTType.utf16ExternalPlainText.identifier {
            return String(data: data, encoding: .utf16BigEndian)
        }
        if typeIdentifier == UTType.utf16PlainText.identifier {
            return String(data: data, encoding: .utf16LittleEndian)
        }
        return String(data: data, encoding: .utf8)
    }

    private static func unique(_ urls: [URL]) -> [URL] {
        var seen = Set<String>()
        return urls.filter { seen.insert($0.absoluteString).inserted }
    }

    private static func load(_ provider: NSItemProvider, type: String) async -> NSSecureCoding? {
        await withCheckedContinuation { continuation in
            provider.loadItem(forTypeIdentifier: type, options: nil) { item, _ in
                continuation.resume(returning: item)
            }
        }
    }
}
