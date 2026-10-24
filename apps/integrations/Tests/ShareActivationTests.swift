import Foundation
import XCTest

final class ShareActivationTests: XCTestCase {
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
