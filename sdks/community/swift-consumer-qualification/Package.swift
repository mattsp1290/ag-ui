// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AGUIConsumerQualification",
    platforms: [.macOS(.v13), .iOS(.v16)],
    dependencies: [
        .package(url: "https://github.com/mattsp1290/ag-ui-swift.git", revision: "31dceaa535e735bc33d6edad547faed410b049e8"),
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
