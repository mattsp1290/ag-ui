// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AGUIConsumerQualification",
    platforms: [.macOS(.v13), .iOS(.v16)],
    dependencies: [
        .package(url: "https://github.com/mattsp1290/ag-ui-swift.git", revision: "f709674e35b120a1df7eeabe01be27efccbb335a"),
    ],
    targets: [
        .testTarget(
            name: "ConsumerTests",
            dependencies: [
                .product(name: "AGUICore", package: "ag-ui-swift"),
                .product(name: "AGUIClient", package: "ag-ui-swift"),
            ],
            resources: [.copy("Fixtures")]
        ),
    ]
)
