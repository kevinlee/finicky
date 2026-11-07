import Foundation
import XCTest

final class BridgeTests: XCTestCase {
    func testURLValuesCannotBecomeAdditionalCommands() throws {
        let original = "https://example.com/a?q=work&prompt=false#日本語"
        let result = try FinickyBridge.commandURL("open", parameters: [
            URLQueryItem(name: "url", value: original),
            URLQueryItem(name: "prompt", value: "true")
        ])
        let parsed = try XCTUnwrap(URLComponents(url: result, resolvingAgainstBaseURL: false))
        XCTAssertEqual(parsed.scheme, "finicky-plus")
        XCTAssertEqual(parsed.host, "open")
        XCTAssertEqual(parsed.queryItems?.filter { $0.name == "url" }.map(\.value), [original])
        XCTAssertEqual(parsed.queryItems?.filter { $0.name == "prompt" }.map(\.value), ["true"])
    }

    func testProfileDirectoryUsesReceivingProtocolKey() {
        let parameters = FinickyBridge.destinationParameters(browser: "Google Chrome", profile: "Work & school", profileDirectory: "Profile 2")
        XCTAssertEqual(parameters.map(\.name), ["browser", "profile", "profileDirectory"])
        XCTAssertEqual(parameters.last?.value, "Profile 2")
        XCTAssertTrue(FinickyBridge.destinationParameters(browser: "  ", profile: nil, profileDirectory: nil).isEmpty)
    }

    func testSharedTextFiltersSchemesAndDeduplicatesInOrder() {
        let links = FinickyBridge.webURLs(in: "First https://example.com/a, then https://example.org/b. Again https://example.com/a and person@example.com.")
        XCTAssertEqual(links.map(\.absoluteString), ["https://example.com/a", "https://example.org/b"])
        XCTAssertFalse(FinickyBridge.isWebURL(URL(string: "file:///tmp/example.html")!))
        XCTAssertFalse(FinickyBridge.isWebURL(URL(string: "finicky-plus://settings")!))
    }

    func testOpenRejectsUnsupportedURLAnywhereInBatch() throws {
        let web = try XCTUnwrap(URL(string: "https://example.com/"))
        XCTAssertThrowsError(try FinickyBridge.openParameters([]))
        for value in ["file:///tmp/example.html", "finicky-plus://settings", "mailto:person@example.com", "https://", "relative/path"] {
            let invalid = try XCTUnwrap(URL(string: value))
            XCTAssertThrowsError(try FinickyBridge.openParameters([web, invalid]))
            XCTAssertThrowsError(try FinickyBridge.openParameters([invalid, web]))
        }
    }

    func testOpenOptionsAreEncodedPerURLAndDefaultOff() throws {
        let first = try XCTUnwrap(URL(string: "HTTPS://example.com/a?private=false&newWindow=false"))
        let second = try XCTUnwrap(URL(string: "https://example.org/b"))
        let requests = try FinickyBridge.openParameters([first, second], prompt: false, browser: "Google Chrome", profileDirectory: "Profile 2", privateMode: true, newWindow: true, background: true)
        XCTAssertEqual(requests.count, 2)
        for parameters in requests {
            let encoded = try FinickyBridge.commandURL("open", parameters: parameters)
            let parsed = try XCTUnwrap(URLComponents(url: encoded, resolvingAgainstBaseURL: false))
            let values = Dictionary(uniqueKeysWithValues: (parsed.queryItems ?? []).map { ($0.name, $0.value ?? "") })
            XCTAssertEqual(values["private"], "true")
            XCTAssertEqual(values["newWindow"], "true")
            XCTAssertEqual(values["background"], "true")
            XCTAssertEqual(values["prompt"], "false")
            XCTAssertEqual(values["profileDirectory"], "Profile 2")
        }
        XCTAssertEqual(requests[0].first?.value, "https://example.com/a?private=false&newWindow=false")
        let defaults = try XCTUnwrap(FinickyBridge.openParameters([second]).first)
        for flag in ["private", "newWindow", "background"] {
            XCTAssertEqual(defaults.first(where: { $0.name == flag })?.value, "false")
        }
    }
}
