import AppKit
import Foundation

enum FinickyBridgeError: LocalizedError {
    case invalidURL
    case notInstalled
    case noLinks

    var errorDescription: String? {
        switch self {
        case .invalidURL: return "Waystile requires a valid HTTP or HTTPS web link."
        case .notInstalled: return "Open Waystile once to register it, then try again."
        case .noLinks: return "No web links were found in the shared content."
        }
    }
}

enum FinickyBridge {
    static func commandURL(_ command: String, parameters: [URLQueryItem] = []) throws -> URL {
        var components = URLComponents()
        components.scheme = "finicky-plus"
        components.host = command
        components.queryItems = parameters.isEmpty ? nil : parameters
        guard let url = components.url else { throw FinickyBridgeError.invalidURL }
        return url
    }

    static func destinationParameters(browser: String?, profile: String?, profileDirectory: String?) -> [URLQueryItem] {
        [("browser", browser), ("profile", profile), ("profileDirectory", profileDirectory)].compactMap { name, value in
            guard let value, !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
            return URLQueryItem(name: name, value: value)
        }
    }

    @MainActor
    static func send(_ command: String, parameters: [URLQueryItem] = []) throws {
        let url = try commandURL(command, parameters: parameters)
        guard NSWorkspace.shared.open(url) else { throw FinickyBridgeError.notInstalled }
    }

    static func openParameters(_ urls: [URL], prompt: Bool = true, browser: String? = nil, profile: String? = nil, profileDirectory: String? = nil, privateMode: Bool = false, newWindow: Bool = false, background: Bool = false) throws -> [[URLQueryItem]] {
        guard !urls.isEmpty else { throw FinickyBridgeError.noLinks }
        // Validate the entire batch before any dispatch, so a mixed list cannot
        // open some links and only then report an unsupported file/custom URL.
        guard urls.allSatisfy(isWebURL) else { throw FinickyBridgeError.invalidURL }
        let destination = destinationParameters(browser: browser, profile: profile, profileDirectory: profileDirectory)
        return try urls.map { url in
            guard var normalized = URLComponents(url: url, resolvingAgainstBaseURL: false) else { throw FinickyBridgeError.invalidURL }
            normalized.scheme = normalized.scheme?.lowercased()
            guard let webURL = normalized.url else { throw FinickyBridgeError.invalidURL }
            return [
                URLQueryItem(name: "url", value: webURL.absoluteString),
                URLQueryItem(name: "prompt", value: prompt ? "true" : "false"),
                URLQueryItem(name: "private", value: privateMode ? "true" : "false"),
                URLQueryItem(name: "newWindow", value: newWindow ? "true" : "false"),
                URLQueryItem(name: "background", value: background ? "true" : "false")
            ] + destination
        }
    }

    @MainActor
    static func open(_ urls: [URL], prompt: Bool = true, browser: String? = nil, profile: String? = nil, profileDirectory: String? = nil, privateMode: Bool = false, newWindow: Bool = false, background: Bool = false) throws {
        let requests = try openParameters(urls, prompt: prompt, browser: browser, profile: profile, profileDirectory: profileDirectory, privateMode: privateMode, newWindow: newWindow, background: background)
        // The receiving application's protocol accepts one URL per request.
        // Send independent, encoded requests; never concatenate raw URLs.
        for parameters in requests {
            try send("open", parameters: parameters)
        }
    }

    static func isWebURL(_ url: URL) -> Bool {
        ["http", "https"].contains(url.scheme?.lowercased() ?? "") && !(url.host ?? "").isEmpty
    }

    static func webURLs(in text: String) -> [URL] {
        guard let detector = try? NSDataDetector(types: NSTextCheckingResult.CheckingType.link.rawValue) else { return [] }
        let range = NSRange(text.startIndex..<text.endIndex, in: text)
        var seen = Set<String>()
        return detector.matches(in: text, range: range).compactMap { match in
            guard let url = match.url, isWebURL(url), seen.insert(url.absoluteString).inserted else { return nil }
            return url
        }
    }
}
