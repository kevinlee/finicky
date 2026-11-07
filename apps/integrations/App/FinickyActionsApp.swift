import AppIntents
import SafariServices
import SwiftUI

@main
struct FinickyActionsApp: App {
    init() {
        FinickyAppShortcuts.updateAppShortcutParameters()
    }

    var body: some Scene {
        WindowGroup("Waystile Actions") {
            IntegrationView()
        }
        .windowResizability(.contentSize)
    }
}

private struct IntegrationView: View {
    @State private var error: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            Label("Waystile Actions", systemImage: "arrow.triangle.branch")
                .font(.title.bold())
            Text("Use your browser profiles from Shortcuts, Safari, the Share menu, and Focus modes.")
                .foregroundStyle(.secondary)
                .fixedSize(horizontal: false, vertical: true)
            GroupBox("Shortcuts") {
                Text("Search for Waystile Actions in Shortcuts to open links, choose a default profile, open the clipboard, or copy a clean URL.")
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(5)
            }
            GroupBox("Safari and sharing") {
                Text("Enable Waystile Safari in Safari’s Extensions settings. Use its toolbar button or right-click a link to choose a profile. In other apps, choose Open with Waystile from the Share menu.")
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(5)
            }
            GroupBox("Focus modes") {
                Text("Add the Browser Profile filter to a Focus in System Settings. Waystile restores your previous default when the filter is cleared.")
                    .fixedSize(horizontal: false, vertical: true)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .padding(5)
            }
            HStack {
                Button("Safari Extensions…") {
                    SFSafariApplication.showPreferencesForExtension(withIdentifier: "com.kevinlee.finicky.actions.safari") { failure in
                        if let failure { Task { @MainActor in error = failure.localizedDescription } }
                    }
                }
                Spacer()
                Button("Open Waystile") {
                    do { try FinickyBridge.send("settings") }
                    catch { self.error = error.localizedDescription }
                }
                .keyboardShortcut(.defaultAction)
            }
            if let error { Text(error).foregroundStyle(.red).font(.callout).textSelection(.enabled) }
        }
        .padding(26)
        .frame(width: 490)
    }
}
