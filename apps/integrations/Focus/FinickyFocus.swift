import AppIntents
import ExtensionFoundation
import Foundation

@main
struct FinickyFocusExtension: AppIntentsExtension {}

struct FinickyBrowserFocusFilter: SetFocusFilterIntent {
    static var title: LocalizedStringResource = "Browser Profile"
    static var description = IntentDescription("Use a browser profile while this Focus is active, then restore the previous Waystile default.")
    static var openAppWhenRun: Bool = false

    // Optional values revert to nil when the system clears the Focus filter.
    @Parameter(title: "Browser", description: "Browser app name or bundle identifier, such as Google Chrome.") var browser: String?
    @Parameter(title: "Profile") var profile: String?
    @Parameter(title: "Profile Directory") var profileDirectory: String?
    @Parameter(title: "Private Browsing", default: false) var privateMode: Bool

    var displayRepresentation: DisplayRepresentation {
        if let browser, !browser.isEmpty {
            return DisplayRepresentation(title: "\(browser)", subtitle: "\(profile ?? profileDirectory ?? "Default profile")")
        }
        return DisplayRepresentation(title: "Browser Profile", subtitle: "Use the normal Waystile default")
    }

    func perform() async throws -> some IntentResult {
        if let browser, !browser.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            let parameters = FinickyBridge.destinationParameters(browser: browser, profile: profile, profileDirectory: profileDirectory)
            try await FinickyBridge.send("set-default", parameters: [URLQueryItem(name: "focus", value: "active"), URLQueryItem(name: "private", value: privateMode ? "true" : "false")] + parameters)
        } else {
            try await FinickyBridge.send("set-default", parameters: [URLQueryItem(name: "focus", value: "clear")])
        }
        return .result()
    }
}
