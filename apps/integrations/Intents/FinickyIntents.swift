import AppIntents
import Foundation

struct OpenFinickyURLsIntent: AppIntent {
    static var title: LocalizedStringResource = "Open URLs with Waystile"
    static var description = IntentDescription("Open one or more links using routing rules, the profile picker, or a specific browser profile.")
    static var openAppWhenRun: Bool = false

    @Parameter(title: "URLs") var urls: [URL]
    @Parameter(title: "Show Profile Picker", default: true) var prompt: Bool
    @Parameter(title: "Browser", description: "Optional app name or bundle identifier, such as Google Chrome.") var browser: String?
    @Parameter(title: "Profile", description: "Optional browser profile name.") var profile: String?
    @Parameter(title: "Profile Directory", description: "Optional Chromium directory, such as Default or Profile 2.") var profileDirectory: String?
    @Parameter(title: "Private Browsing", default: false) var privateMode: Bool
    @Parameter(title: "Open in New Window", default: false) var newWindow: Bool
    @Parameter(title: "Open in Background", default: false) var background: Bool

    func perform() async throws -> some IntentResult {
        try await FinickyBridge.open(urls, prompt: prompt, browser: browser, profile: profile, profileDirectory: profileDirectory, privateMode: privateMode, newWindow: newWindow, background: background)
        return .result()
    }
}

struct SetFinickyDefaultIntent: AppIntent {
    static var title: LocalizedStringResource = "Set Default Browser Profile"
    static var description = IntentDescription("Change the default destination inside Waystile. Your macOS default-browser setting is not changed.")
    static var openAppWhenRun: Bool = false

    @Parameter(title: "Browser", default: "Google Chrome") var browser: String
    @Parameter(title: "Profile") var profile: String?
    @Parameter(title: "Profile Directory") var profileDirectory: String?
    @Parameter(title: "Private Browsing", default: false) var privateMode: Bool

    func perform() async throws -> some IntentResult {
        let parameters = FinickyBridge.destinationParameters(browser: browser, profile: profile, profileDirectory: profileDirectory)
        try await FinickyBridge.send("set-default", parameters: parameters + [URLQueryItem(name: "private", value: privateMode ? "true" : "false")])
        return .result()
    }
}

struct OpenFinickyClipboardIntent: AppIntent {
    static var title: LocalizedStringResource = "Open Clipboard with Waystile"
    static var description = IntentDescription("Open web links from the clipboard using Waystile.")
    static var openAppWhenRun: Bool = false
    @Parameter(title: "Show Profile Picker", default: true) var prompt: Bool

    func perform() async throws -> some IntentResult {
        try await FinickyBridge.send("clipboard", parameters: [URLQueryItem(name: "prompt", value: prompt ? "true" : "false")])
        return .result()
    }
}

struct CopyCleanFinickyURLIntent: AppIntent {
    static var title: LocalizedStringResource = "Copy Clean URL"
    static var description = IntentDescription("Ask Waystile to remove tracking parameters and copy the cleaned link.")
    static var openAppWhenRun: Bool = false
    @Parameter(title: "URL") var url: URL

    func perform() async throws -> some IntentResult {
        guard FinickyBridge.isWebURL(url) else { throw FinickyBridgeError.invalidURL }
        try await FinickyBridge.send("copy", parameters: [URLQueryItem(name: "url", value: url.absoluteString)])
        return .result()
    }
}

struct OpenFinickySettingsIntent: AppIntent {
    static var title: LocalizedStringResource = "Open Waystile Settings"
    static var openAppWhenRun: Bool = false

    func perform() async throws -> some IntentResult {
        try await FinickyBridge.send("settings")
        return .result()
    }
}

struct FinickyAppShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(intent: OpenFinickyClipboardIntent(), phrases: ["Open links with \(.applicationName)"], shortTitle: "Open Clipboard", systemImageName: "doc.on.clipboard")
        AppShortcut(intent: OpenFinickySettingsIntent(), phrases: ["Open \(.applicationName) settings"], shortTitle: "Waystile Settings", systemImageName: "gearshape")
    }
}
