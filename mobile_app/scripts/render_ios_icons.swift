// Render the original vector mark into the sizes requested by the iOS catalog.
// Run from mobile_app: swift scripts/render_ios_icons.swift
import AppKit
import Foundation
let base = URL(fileURLWithPath: "ios/Runner/Assets.xcassets/AppIcon.appiconset")
let data = try Data(contentsOf: base.appendingPathComponent("Contents.json"))
let catalog = try JSONSerialization.jsonObject(with: data) as! [String: Any]
for icon in catalog["images"] as! [[String: String]] {
  guard let file = icon["filename"], let size = icon["size"], let scale = icon["scale"] else { continue }
  let points = Double(size.split(separator: "x")[0])!
  let pixels = Int(points * Double(scale.replacingOccurrences(of: "x", with: ""))!)
  let ctx = CGContext(data: nil, width: pixels, height: pixels, bitsPerComponent: 8, bytesPerRow: pixels * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue)!
  ctx.scaleBy(x: CGFloat(pixels)/108, y: CGFloat(pixels)/108)
  ctx.setFillColor(CGColor(red:24/255,green:63/255,blue:54/255,alpha:1)); ctx.fill(CGRect(x:0,y:0,width:108,height:108))
  ctx.translateBy(x:0,y:108); ctx.scaleBy(x:1,y:-1)
  ctx.setStrokeColor(CGColor(red:246/255,green:247/255,blue:243/255,alpha:1));ctx.setLineWidth(4);ctx.setLineCap(.round);ctx.setLineJoin(.round)
  let paths: [[CGPoint]] = [[CGPoint(x:28,y:41),CGPoint(x:54,y:28),CGPoint(x:80,y:41),CGPoint(x:54,y:54),CGPoint(x:28,y:41)],[CGPoint(x:28,y:54),CGPoint(x:54,y:67),CGPoint(x:80,y:54)],[CGPoint(x:28,y:67),CGPoint(x:54,y:80),CGPoint(x:80,y:67)]]
  for points in paths { ctx.beginPath();ctx.move(to:points[0]);for point in points.dropFirst(){ctx.addLine(to:point)};ctx.strokePath() }
  let rep = NSBitmapImageRep(cgImage:ctx.makeImage()!)
  try rep.representation(using:.png,properties:[:])!.write(to:base.appendingPathComponent(file))
}
print("Rendered original SettleGraph app mark into iOS icon catalog")
