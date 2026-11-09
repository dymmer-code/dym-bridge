// Run from repository root: swift packaging/macos/render-tray.swift
// Render the brand's bridge geometry without the app-icon tile, at 18pt / 2x.
import CoreGraphics
import ImageIO
import Foundation

let size = 36
let ctx = CGContext(data: nil, width: size, height: size, bitsPerComponent: 8,
                    bytesPerRow: size * 4, space: CGColorSpaceCreateDeviceRGB(),
                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
ctx.scaleBy(x: 36.0 / 672, y: 36.0 / 672)
ctx.translateBy(x: -176, y: -176)
ctx.translateBy(x: 512, y: 512)
ctx.rotate(by: .pi / 4)
ctx.translateBy(x: -512, y: -512)
func rounded(_ x: CGFloat, _ y: CGFloat, _ w: CGFloat, _ h: CGFloat, _ radius: CGFloat) {
    ctx.addPath(CGPath(roundedRect: CGRect(x: x, y: y, width: w, height: h),
                      cornerWidth: radius, cornerHeight: radius, transform: nil))
    ctx.fillPath()
}
ctx.setFillColor(CGColor(gray: 1, alpha: 1))
rounded(262, 262, 500, 500, 82)
ctx.setBlendMode(.clear)
rounded(426, 240, 40, 544, 20)
rounded(558, 240, 40, 544, 20)
ctx.setLineWidth(11)
ctx.setLineCap(.round)
for y in [285, 415, 545, 675] {
    ctx.move(to: CGPoint(x: 512, y: y))
    ctx.addLine(to: CGPoint(x: 512, y: min(y + 80, 745)))
    ctx.strokePath()
}
ctx.setBlendMode(.normal)
for x: CGFloat in [412, 544] {
    for y: CGFloat in [375, 625] { rounded(x, y, 68, 26, 7) }
}
let url = URL(fileURLWithPath: "pkg/desktop/tray-macos.png")
let destination = CGImageDestinationCreateWithURL(url as CFURL, "public.png" as CFString, 1, nil)!
CGImageDestinationAddImage(destination, ctx.makeImage()!, [kCGImagePropertyDPIWidth: 144, kCGImagePropertyDPIHeight: 144] as CFDictionary)
precondition(CGImageDestinationFinalize(destination))
