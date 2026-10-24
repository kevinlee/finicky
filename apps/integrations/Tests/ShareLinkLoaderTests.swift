import AppKit
import UniformTypeIdentifiers
import XCTest

final class ShareLinkLoaderTests: XCTestCase {
    func testAttributedLinksRetainDestinationAndReadingOrder() {
        let content = NSMutableAttributedString(string: "https://example.edu/first Read document https://example.edu/last")
        let label = (content.string as NSString).range(of: "Read document")
        content.addAttribute(.link, value: URL(string: "https://example.edu/document")!, range: label)
        XCTAssertEqual(ShareLinkLoader.urls(in: content).map(\.absoluteString), [
            "https://example.edu/first", "https://example.edu/document", "https://example.edu/last"
        ])
        let explicit = NSAttributedString(string: "https://example.edu/label", attributes: [.link: "https://example.edu/destination"])
        XCTAssertEqual(ShareLinkLoader.urls(in: explicit).map(\.absoluteString), ["https://example.edu/destination"])
    }

    func testTextDataDecodesUTF8AndBothUTF16ByteOrders() throws {
        let text = "Selected https://example.edu/unicode?q=caf%C3%A9"
        let cases: [(String.Encoding, UTType)] = [
            (.utf8, .utf8PlainText), (.utf16, .plainText),
            (.utf16LittleEndian, .utf16PlainText), (.utf16BigEndian, .utf16ExternalPlainText)
        ]
        for (encoding, type) in cases {
            let data = try XCTUnwrap(text.data(using: encoding))
            XCTAssertEqual(ShareLinkLoader.urls(in: data as NSData, typeIdentifier: type.identifier).map(\.absoluteString), ["https://example.edu/unicode?q=caf%C3%A9"], type.identifier)
        }
    }

    func testRTFExtractsHiddenLinkAttributesWithoutImportingHTML() throws {
        let rich = NSAttributedString(string: "Open document", attributes: [.link: URL(string: "https://example.edu/rtf")!])
        let data = try rich.data(from: NSRange(location: 0, length: rich.length), documentAttributes: [.documentType: NSAttributedString.DocumentType.rtf])
        XCTAssertEqual(ShareLinkLoader.urls(in: data as NSData, typeIdentifier: UTType.rtf.identifier).map(\.absoluteString), ["https://example.edu/rtf"])
        XCTAssertEqual(ShareLinkLoader.urls(in: rich, typeIdentifier: UTType.text.identifier).map(\.absoluteString), ["https://example.edu/rtf"])
        let html = "<img src='https://example.edu/image'><a href='https://example.edu/page'>Open</a>"
        XCTAssertTrue(ShareLinkLoader.urls(in: html as NSString, typeIdentifier: UTType.html.identifier).isEmpty)
        XCTAssertTrue(ShareLinkLoader.urls(in: html.data(using: .utf8)! as NSData, typeIdentifier: UTType.html.identifier).isEmpty)
        XCTAssertTrue(ShareLinkLoader.supportedTypes([UTType.html.identifier, UTType.fileURL.identifier, UTType.image.identifier]).isEmpty)
    }

    @MainActor
    func testContentOnlySelectionAndAttachmentsDeduplicate() async {
        let item = NSExtensionItem()
        item.attributedContentText = NSAttributedString(string: "Document", attributes: [.link: URL(string: "https://example.edu/document")!])
        item.attachments = [provider(text: "https://example.edu/document https://example.edu/second")]
        let result = await ShareLinkLoader.urls(from: [item])
        XCTAssertEqual(result.map(\.absoluteString), ["https://example.edu/document", "https://example.edu/second"])
        item.attachments = nil
        let contentOnly = await ShareLinkLoader.urls(from: [item])
        XCTAssertEqual(contentOnly.map(\.absoluteString), ["https://example.edu/document"])
    }

    @MainActor
    func testUnusableURLRepresentationFallsBackToPlainText() async {
        for failURL in [false, true] {
            let itemProvider = provider(text: "https://example.edu/fallback")
            itemProvider.registerDataRepresentation(forTypeIdentifier: UTType.url.identifier, visibility: .all) { completion in
                if failURL {
                    completion(nil, NSError(domain: "SyntheticProvider", code: 1))
                } else {
                    completion(Data("file:///synthetic/no-read".utf8), nil)
                }
                return nil
            }
            let item = NSExtensionItem()
            item.attachments = [itemProvider]
            let result = await ShareLinkLoader.urls(from: [item])
            XCTAssertEqual(result.map(\.absoluteString), ["https://example.edu/fallback"])
        }
    }

    @MainActor
    func testCancelledCollectionDoesNotLoadProviders() async {
        var loaded = false
        let itemProvider = NSItemProvider()
        itemProvider.registerDataRepresentation(forTypeIdentifier: UTType.plainText.identifier, visibility: .all) { completion in
            loaded = true
            completion(Data("https://example.edu/not-opened".utf8), nil)
            return nil
        }
        let item = NSExtensionItem()
        item.attachments = [itemProvider]
        let result = await ShareLinkLoader.urls(from: [item], isCancelled: { true })
        XCTAssertTrue(result.isEmpty)
        XCTAssertFalse(loaded)
    }

    private func provider(text: String) -> NSItemProvider {
        let provider = NSItemProvider()
        provider.registerDataRepresentation(forTypeIdentifier: UTType.utf8PlainText.identifier, visibility: .all) { completion in
            completion(Data(text.utf8), nil)
            return nil
        }
        return provider
    }
}
