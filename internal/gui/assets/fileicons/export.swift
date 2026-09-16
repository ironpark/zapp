// Run on macOS: swift export.swift (outputs PNGs beside this script).
import AppKit
import UniformTypeIdentifiers

let output = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
let core = "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/"
let systemFiles = [
    "app": "GenericApplicationIcon", "folder": "GenericFolderIcon",
    "file": "GenericDocumentIcon", "applications": "ApplicationsFolderIcon",
    "executable": "ExecutableBinaryIcon", "font": "GenericFontIcon",
    "alias": "AliasBadgeIcon",
]
let types: [String: UTType] = [
    "text": .plainText, "pdf": .pdf, "image": .image, "audio": .audio,
    "video": .movie, "archive": .zip, "disk": .diskImage,
    "source": .sourceCode, "script": .shellScript,
]
func export(_ image: NSImage, name: String) throws {
    let side = 256
    let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
    image.draw(in: NSRect(x: 0, y: 0, width: side, height: side), from: .zero,
        operation: .copy, fraction: 1)
    NSGraphicsContext.restoreGraphicsState()
    try rep.representation(using: .png, properties: [:])!.write(to: output.appendingPathComponent(name + ".png"))
}
for (name, file) in systemFiles {
    guard let image = NSImage(contentsOfFile: core + file + ".icns") else {
        fatalError("Missing system icon: \(file)")
    }
    try export(image, name: name)
}
for (name, type) in types { try export(NSWorkspace.shared.icon(for: type), name: name) }
try export(NSWorkspace.shared.icon(for: UTType(filenameExtension: "pkg") ?? .package), name: "package")
