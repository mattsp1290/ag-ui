// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "AGUIConsumerQualification",
    platforms: [.macOS(.v13), .iOS(.v16)],
    dependencies: [
        .package(url: "https://github.com/mattsp1290/ag-ui-swift.git", revision: "9412aab2549e06e165ada85fe6346b9b6e5a0f2b"),
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
