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
            "Open with Finicky Plus"
        )
        let iconName = try XCTUnwrap(
            extensionBundle.object(forInfoDictionaryKey: "CFBundleIconFile") as? String
        )
        let iconURL = try XCTUnwrap(extensionBundle.url(forResource: iconName, withExtension: "icns"))
        let icon = try XCTUnwrap(NSImage(contentsOf: iconURL))
        XCTAssertGreaterThan(icon.size.width, 0)
        XCTAssertGreaterThan(icon.size.height, 0)
    }

    func testSharedLinksCanOfferUnsupportedAlternateRepresentations() throws {
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
        let rule = try XCTUnwrap(attributes["NSExtensionActivationRule"] as? [String: Any])

        // Apple's version 1 default rejects a provider offering both supported
        // text/URLs and unsupported image assets. Version 2 accepts a supported
        // representation without claiming that this extension handles images.
        // https://developer.apple.com/library/archive/documentation/General/Reference/InfoPlistKeyReference/Articles/AppExtensionKeys.html
        XCTAssertEqual(rule["NSExtensionActivationDictionaryVersion"] as? Int, 2)
        XCTAssertEqual(rule["NSExtensionActivationSupportsText"] as? Bool, true)
        let maxURLs = try XCTUnwrap(rule["NSExtensionActivationSupportsWebURLWithMaxCount"] as? Int)
        XCTAssertGreaterThan(maxURLs, 0)
        XCTAssertLessThanOrEqual(maxURLs, 256)
        for unsupportedType in ["Image", "Movie", "File"] {
            XCTAssertNil(rule["NSExtensionActivationSupports\(unsupportedType)WithMaxCount"])
        }
    }
}
