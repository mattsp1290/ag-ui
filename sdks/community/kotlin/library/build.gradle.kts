// Root build script for AG-UI-4K multiplatform library
// All modules are configured individually - see each module's build.gradle.kts

import kotlinx.kover.gradle.plugin.dsl.KoverProjectExtension
import org.gradle.api.publish.PublishingExtension
import org.gradle.api.publish.maven.MavenPublication
import org.gradle.jvm.tasks.Jar
import org.jetbrains.kotlin.gradle.plugin.mpp.KotlinNativeTarget
import org.jetbrains.kotlin.gradle.targets.jvm.KotlinJvmTarget
import org.jreleaser.gradle.plugin.tasks.JReleaserDeployTask

plugins {
    kotlin("multiplatform") version "2.2.20" apply false
    kotlin("plugin.serialization") version "2.2.20" apply false
    id("com.android.library") version "8.10.1" apply false
    id("org.jetbrains.dokka") version "2.0.0"
    id("org.jetbrains.kotlinx.kover") version "0.8.3"
    id("org.jreleaser") version "1.20.0"
}

// Single source of truth for version - used by both subprojects and JReleaser
version = "0.2.3"
group = "com.contextable"

allprojects {
    repositories {
        google()
        mavenCentral()
    }
}

// Configure all subprojects with common settings
subprojects {

    // Apply the publishing plugin to all subprojects
    apply(plugin = "maven-publish")

    // Configure all subprojects to publish to the root staging directory
    extensions.configure<PublishingExtension> {
        repositories {
            maven {
                name = "jreleaserStaging"
                // Use rootProject.buildDir to ensure all modules publish to one
                // single /build/staging-deploy directory at the project root
                url = uri("${rootProject.buildDir}/staging-deploy")
            }
        }
    }

    group = rootProject.group
    version = rootProject.version

    apply(plugin = "org.jetbrains.kotlinx.kover")
    extensions.configure<KoverProjectExtension>("kover") {
        currentProject {
            instrumentation {
                disabledForTestTasks.addAll(
                    "jvmTest",
                    "testDebugUnitTest",
                    "testReleaseUnitTest"
                )
            }
        }
    }

    tasks.withType<Test> {
        useJUnitPlatform()
    }

    afterEvaluate {
        group = rootProject.group
        version = rootProject.version
    }
    
    // Apply Dokka to all subprojects
    apply(plugin = "org.jetbrains.dokka")
        plugins.withId("org.jetbrains.dokka") {

        val dokkaTask = tasks.findByName("dokkaHtml") ?: tasks.findByName("dokkaGenerate")

        if (dokkaTask == null) {
            logger.warn("Dokka task not found in project ${project.name}; skipping javadocJar attachment.")
            return@afterEvaluate
        }

        val javadocJar = tasks.register("javadocJar", Jar::class.java) {
            dependsOn(dokkaTask)
            archiveClassifier.set("javadoc")
            from(dokkaTask.outputs.files)
        }

        extensions.configure(PublishingExtension::class.java) {
            publications.withType(MavenPublication::class.java) {
                // NEW, SIMPLIFIED LOGIC:
                // ONLY attach javadoc to the 'jvm' publication.
                // All other publications (android, ios) are handled
                // by the artifactOverride rules.
                if (name.equals("jvm", ignoreCase = true)) {
                    artifact(javadocJar)
                }
            }
        }
    }
}

// Simple Dokka V2 configuration - let it use defaults for navigation

tasks.register("koverHtmlReportAll") {
    group = "verification"
    description = "Generates HTML coverage reports for all library modules."
    dependsOn(subprojects.map { "${it.path}:koverHtmlReport" })
}

// Create a task to generate unified documentation
tasks.register("dokkaHtmlMultiModule") {
    dependsOn(subprojects.map { "${it.name}:dokkaGenerate" })
    group = "documentation"
    description = "Generate unified HTML documentation for all modules"
    
    doLast {
        val outputDir = layout.buildDirectory.dir("dokka/htmlMultiModule").get().asFile
        outputDir.mkdirs()
        
        // Create master index.html
        val indexContent = """
<!DOCTYPE html>
<html>
<head>
    <title>AG-UI-4K Documentation</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 2rem; }
        .module { margin: 1rem 0; padding: 1rem; border: 1px solid #ddd; border-radius: 8px; }
        .module h2 { margin-top: 0; color: #2563eb; }
        .module a { text-decoration: none; color: #2563eb; font-weight: 500; }
        .module a:hover { text-decoration: underline; }
        .description { color: #6b7280; margin: 0.5rem 0; }
    </style>
</head>
<body>
    <h1>AG-UI-4K Library Documentation</h1>
    <p>Welcome to the AG-UI-4K Kotlin Multiplatform library for building AI agent user interfaces.</p>
    
    <div class="module">
        <h2><a href="core/index.html">core</a></h2>
        <p class="description">Core types, events, and protocol definitions for the AG-UI protocol</p>
    </div>
    
    <div class="module">
        <h2><a href="client/index.html">client</a></h2>
        <p class="description">Client implementations, HTTP transport, SSE parsing, state management, and high-level agent SDKs</p>
    </div>
    
    <div class="module">
        <h2><a href="tools/index.html">tools</a></h2>
        <p class="description">Tool system, error handling, circuit breakers, and execution management</p>
    </div>
    
    <p style="margin-top: 2rem; color: #6b7280; font-size: 0.9rem;">
        Generated by Dokka from KDoc comments
    </p>
</body>
</html>
        """.trimIndent()
        
        val indexFile = outputDir.resolve("index.html")
        indexFile.writeText(indexContent)
        
        // Copy individual module documentation
        subprojects.forEach { project ->
            val moduleDocsDir = project.layout.buildDirectory.dir("dokka/html").get().asFile
            if (moduleDocsDir.exists()) {
                val targetDir = outputDir.resolve(project.name)
                moduleDocsDir.copyRecursively(targetDir, overwrite = true)
            }
        }
        
        println("Unified documentation generated at: ${outputDir.absolutePath}/index.html")
    }
}

// Configure artifact overrides after all subprojects are evaluated
// Workaround for Kotlin Multiplatform iOS targets that produce .klib files instead of JARs
// In your root build.gradle.kts

afterEvaluate {
    val nonJvmTargetArtifactIds = mutableListOf<String>()

    subprojects {
        plugins.withId("org.jetbrains.kotlin.multiplatform") {
            extensions.configure(org.jetbrains.kotlin.gradle.dsl.KotlinMultiplatformExtension::class.java) {
                targets.forEach { target ->
                    // Find all non-JVM targets (android, ios, etc.)
                    // and exclude the "metadata" target
                    if (target !is KotlinJvmTarget && target.name != "metadata") {
                        // THIS IS THE FIX:
                        // Create artifactId: "kotlin-" + "core" + "-" + "iosx64"
                        nonJvmTargetArtifactIds.add("kotlin-${project.name}-${target.name.lowercase()}")
                    }
                }
            }
        }
    }

    // Configure JReleaser with this corrected list
    jreleaser {
        gitRootSearch = true

        // Project information
        project {
            name.set("ag-ui-kotlin-sdk")
            version.set(rootProject.version.toString())
            description.set("Kotlin Multiplatform SDK for the Agent User Interaction Protocol")
            website.set("https://github.com/ag-ui-protocol/ag-ui")
            authors.set(listOf("Mark Fogle"))
            license.set("MIT")
            inceptionYear.set("2024")
            java {
                groupId.set("com.contextable")
                version.set("21")
                multiProject.set(true)
            }
        }

        // Enable GPG signing
        signing {
            active.set(org.jreleaser.model.Active.ALWAYS)
            armored.set(true)
        }

        // Configure Maven Central deployment
        deploy {
            maven {
                mavenCentral {
                    create("sonatype") {
                        active.set(org.jreleaser.model.Active.ALWAYS)
                        url.set("https://central.sonatype.com/api/v1/publisher")
                        stagingRepository("build/staging-deploy")
                        namespace.set("com.contextable")
                        sign.set(true)
                        checksums.set(true)
                        sourceJar.set(true)
                        javadocJar.set(true)

                        // Disable pomchecker 
                        pomchecker {
                            enabled.set(false)
                        }

                        // Merged-in Artifact Overrides
                        nonJvmTargetArtifactIds.forEach { artifactId ->
                            artifactOverride {
                                this.artifactId.set(artifactId)
                                jar.set(false)
                                verifyPom.set(false)
                                sourceJar.set(false)
                                javadocJar.set(false)
                            }
                        }
                    }
                }
            }
        }
    }
}
