#!/usr/bin/env swift
// Original Waystile route mark. Native vector geometry; no external assets.
import Foundation
import CoreGraphics
import ImageIO
import UniformTypeIdentifiers

let root = URL(fileURLWithPath: CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : FileManager.default.currentDirectoryPath)
let fm = FileManager.default
let staging = fm.temporaryDirectory.appendingPathComponent("waystile-icons-" + UUID().uuidString)
try fm.createDirectory(at: staging, withIntermediateDirectories: true)
defer { try? fm.removeItem(at: staging) }
let space = CGColorSpaceCreateDeviceRGB()
func color(_ r: CGFloat, _ g: CGFloat, _ b: CGFloat, _ a: CGFloat = 1) -> CGColor {
    CGColor(colorSpace: space, components: [r / 255, g / 255, b / 255, a])!
}
func draw(_ size: Int, to url: URL, template: Bool = false) throws {
    let context = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8,
                            bytesPerRow: size * 4, space: space,
                            bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
    context.scaleBy(x: CGFloat(size) / 1024, y: CGFloat(size) / 1024)
    context.translateBy(x: 0, y: 1024)
    context.scaleBy(x: 1, y: -1)
    if !template {
        context.saveGState()
        context.addPath(CGPath(roundedRect: CGRect(x: 64, y: 64, width: 896, height: 896),
                              cornerWidth: 208, cornerHeight: 208, transform: nil))
        context.clip()
        let gradient = CGGradient(colorsSpace: space, colors: [color(32, 40, 63), color(61, 61, 130)] as CFArray, locations: [0, 1])!
        context.drawLinearGradient(gradient, start: CGPoint(x: 64, y: 64), end: CGPoint(x: 960, y: 960), options: [])
        context.restoreGState()
        context.addPath(CGPath(roundedRect: CGRect(x: 66, y: 66, width: 892, height: 892),
                              cornerWidth: 206, cornerHeight: 206, transform: nil))
        context.setStrokeColor(color(255, 255, 255, 0.12))
        context.setLineWidth(4)
        context.strokePath()
    } else {
        // Fill more of an 18px menu-bar cell than the inset app-icon mark.
        context.translateBy(x: -240, y: -240)
        context.scaleBy(x: 1.47, y: 1.47)
    }
    context.setLineWidth(76)
    context.setLineCap(.round)
    context.setLineJoin(.round)
    context.setStrokeColor(template ? color(0, 0, 0) : color(247, 248, 255))
    context.move(to: CGPoint(x: 270, y: 338))
    for point in [CGPoint(x: 382, y: 684), CGPoint(x: 512, y: 470), CGPoint(x: 642, y: 684), CGPoint(x: 754, y: 338)] { context.addLine(to: point) }
    context.strokePath()
    context.setLineWidth(64)
    let accent = template ? color(0, 0, 0) : color(178, 173, 255)
    context.setStrokeColor(accent)
    context.move(to: CGPoint(x: 512, y: 470))
    context.addLine(to: CGPoint(x: 512, y: 286))
    context.strokePath()
    context.setFillColor(accent)
    for point in [CGPoint(x: 270, y: 338), CGPoint(x: 512, y: 286), CGPoint(x: 754, y: 338)] {
        context.fillEllipse(in: CGRect(x: point.x - 48, y: point.y - 48, width: 96, height: 96))
    }
    let destination = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil)!
    CGImageDestinationAddImage(destination, context.makeImage()!, nil)
    guard CGImageDestinationFinalize(destination) else { throw NSError(domain: "WaystileIcon", code: 1) }
}
func iconSet(_ name: String, to destination: URL, template: Bool = false) throws {
    let set = staging.appendingPathComponent(name + ".iconset")
    try fm.createDirectory(at: set, withIntermediateDirectories: true)
    for size in [16, 32, 128, 256, 512] {
        try draw(size, to: set.appendingPathComponent("icon_\(size)x\(size).png"), template: template)
        try draw(size * 2, to: set.appendingPathComponent("icon_\(size)x\(size)@2x.png"), template: template)
    }
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/iconutil")
    process.arguments = ["-c", "icns", set.path, "-o", destination.path]
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0 else { throw NSError(domain: "WaystileIcon", code: 2) }
}
try iconSet("Waystile", to: root.appendingPathComponent("apps/finicky/assets/Resources/finicky.icns"))
try iconSet("WaystileMenu", to: root.appendingPathComponent("apps/finicky/assets/Resources/menu-bar.icns"), template: true)
try draw(1024, to: root.appendingPathComponent("design/waystile-icon.png"))
for size in [16, 48, 128, 256] {
    try draw(size, to: root.appendingPathComponent("apps/browser-addon/icon\(size).png"))
}
print("Generated Waystile app, menu-bar, and browser-extension icons.")
