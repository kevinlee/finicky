import AppKit
import Foundation
import XCTest

final class ShareActivationTests: XCTestCase {
    func testPackagedShareExtensionHasDisplayNameAndReadableIcon() throws {
        // Inspect the embedded product, so a missing resource-copy build phase
        // fails even when the source plist and icon are present. This unhosted
        // test bundle and the app are built beside each other by FinickyActions.
        let products = Bundle(for: Self.self).bundleURL.deletingLastPathComponent()
        let extensionURL = products.appendingPathComponent(
            "Finicky Actions.app/Contents/PlugIns/Open with Finicky Plus.appex"
        )
        let extensionBundle = try XCTUnwrap(Bundle(url: extensionURL))
        XCTAssertEqual(
            extensionBundle.object(forInfoDictionaryKey: "CFBundleDisplayName") as? String,
            "Open with Waystile"
        )
        let iconName = try XCTUnwrap(
            extensionBundle.object(forInfoDictionaryKey: "CFBundleIconFile") as? String
        )
        let iconURL = try XCTUnwrap(extensionBundle.url(forResource: iconName, withExtension: "icns"))
        let icon = try XCTUnwrap(NSImage(contentsOf: iconURL))
        XCTAssertGreaterThan(icon.size.width, 0)
        XCTAssertGreaterThan(icon.size.height, 0)
    }

    func testSelectedAttributedTextDoesNotRequireAttachments() throws {
        let predicate = try activationPredicate()
        // This is the shape macOS creates for NSExtensionItem's attributed
        // content: RTF data under contentText, with no attachments key. Testing
        // an empty attachments array alone would miss the host-menu regression.
        let text = NSAttributedString(string: "https://example.com/one https://example.com/two")
        let data = try text.data(
            from: NSRange(location: 0, length: text.length),
            documentAttributes: [.documentType: NSAttributedString.DocumentType.rtf]
        )
        XCTAssertTrue(predicate.evaluate(with: ["extensionItems": [["contentText": data]]]))
        XCTAssertTrue(predicate.evaluate(with: ["extensionItems": [["contentText": data, "attachments": []]]]))
    }

    func testActivationAcceptsReadableTextAndWebURLRepresentations() throws {
        let predicate = try activationPredicate()
        for type in ["public.url", "public.plain-text", "public.utf8-plain-text", "public.utf16-plain-text", "public.rtf"] {
            XCTAssertTrue(predicate.evaluate(with: matchingInput([[type]])), type)
        }
        // An unsupported alternate representation must not hide a readable
        // URL/text representation of the same item.
        XCTAssertTrue(predicate.evaluate(with: matchingInput([["public.url", "public.html", "public.png"]])))
        XCTAssertTrue(predicate.evaluate(with: matchingInput([["public.url", "com.apple.active-webpage"]])))
        XCTAssertTrue(predicate.evaluate(with: matchingInput([["public.utf8-plain-text", "public.png"]])))
        XCTAssertTrue(predicate.evaluate(with: matchingInput([["public.file-url", "public.utf8-plain-text"]])))
    }

    func testActivationRejectsUnsupportedAndEmptyInputs() throws {
        let predicate = try activationPredicate()
        for type in ["public.png", "public.movie", "public.file-url", "public.html", "com.apple.active-webpage"] {
            XCTAssertFalse(predicate.evaluate(with: matchingInput([[type]])), type)
        }
        XCTAssertFalse(predicate.evaluate(with: matchingInput([["public.file-url", "public.url"]])))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": []]))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": [[:]]]))
        XCTAssertFalse(predicate.evaluate(with: matchingInput([])))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": [["title": Data("title".utf8)]]]))
    }

    func testActivationBoundsItemsAndAttachments() throws {
        let predicate = try activationPredicate()
        let provider = ["registeredTypeIdentifiers": ["public.url"]]
        let item = ["attachments": [provider]]
        XCTAssertTrue(predicate.evaluate(with: ["extensionItems": Array(repeating: item, count: 256)]))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": Array(repeating: item, count: 257)]))
        XCTAssertTrue(predicate.evaluate(with: ["extensionItems": [["attachments": Array(repeating: provider, count: 256)]]]))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": [["attachments": Array(repeating: provider, count: 257)]]]))
        XCTAssertFalse(predicate.evaluate(with: ["extensionItems": [item, ["attachments": [["registeredTypeIdentifiers": ["public.png"]]]]]]))
    }

    private func matchingInput(_ providerTypes: [[String]]) -> [String: Any] {
        ["extensionItems": [["attachments": providerTypes.map { ["registeredTypeIdentifiers": $0] }]]]
    }

    private func activationPredicate() throws -> NSPredicate {
        let source = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .appendingPathComponent("Share/Info.plist")
        let data = try Data(contentsOf: source)
        let plist = try XCTUnwrap(
            PropertyListSerialization.propertyList(from: data, format: nil) as? [String: Any]
        )
        let extensionInfo = try XCTUnwrap(plist["NSExtension"] as? [String: Any])
        let attributes = try XCTUnwrap(extensionInfo["NSExtensionAttributes"] as? [String: Any])
        let rule = try XCTUnwrap(attributes["NSExtensionActivationRule"] as? String)
        return NSPredicate(format: rule)
    }
}
