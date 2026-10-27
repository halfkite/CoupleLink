import java.util.Properties
import java.util.Base64
import org.gradle.api.DefaultTask
import org.gradle.api.file.DirectoryProperty
import org.gradle.api.provider.ListProperty
import org.gradle.api.tasks.Input
import org.gradle.api.tasks.OutputDirectory
import org.gradle.api.tasks.TaskAction

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// A local test build may opt into specific private IPv4 hosts. The platform
// default remains HTTPS-only; no public host or whole subnet is enabled.
val dartDefines = providers.gradleProperty("dart-defines").orElse("").get().split(',')
    .filter { it.isNotEmpty() }.map { String(Base64.getDecoder().decode(it), Charsets.UTF_8) }
val localHttpHosts = dartDefines.firstOrNull { it.startsWith("COUPLELINK_LOCAL_HTTP_HOSTS=") }
    ?.substringAfter('=')?.split(',')?.map { it.trim() }?.filter { it.isNotEmpty() }?.distinct()?.sorted() ?: emptyList()
fun isPrivateIPv4(host: String): Boolean {
    val parts = host.split('.')
    if (parts.size != 4 || parts.any { !it.matches(Regex("(0|[1-9][0-9]{0,2})")) }) return false
    val bytes = parts.map { it.toInt() }
    if (bytes.any { it > 255 }) return false
    return bytes[0] == 10 || (bytes[0] == 172 && bytes[1] in 16..31) ||
        (bytes[0] == 192 && bytes[1] == 168)
}
require(localHttpHosts.all(::isPrivateIPv4)) { "Local HTTP hosts must be explicit private IPv4 addresses" }
abstract class GenerateCoupleLinkNetworkSecurity : DefaultTask() {
    @get:Input abstract val allowedHosts: ListProperty<String>
    @get:OutputDirectory abstract val outputDirectory: DirectoryProperty
    @TaskAction fun generate() {
        val file = outputDirectory.get().file("xml/couplelink_network_security.xml").asFile
        file.parentFile.mkdirs()
        val hosts = (listOf("localhost", "127.0.0.1", "10.0.2.2") + allowedHosts.get()).distinct()
        file.writeText("""<?xml version="1.0" encoding="utf-8"?>
<network-security-config>
    <base-config cleartextTrafficPermitted="false" />
    <domain-config cleartextTrafficPermitted="true">
${hosts.joinToString("\n") { "        <domain includeSubdomains=\"false\">$it</domain>" }}
    </domain-config>
</network-security-config>
""")
    }
}
val generateLocalNetwork = tasks.register<GenerateCoupleLinkNetworkSecurity>("generateCoupleLinkNetworkSecurity") {
    allowedHosts.set(localHttpHosts)
    outputDirectory.set(layout.buildDirectory.dir("generated/couplelinkNetwork/res"))
}

android {
    namespace = "app.couplelink.mobile"
    compileSdk = flutter.compileSdkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        manifestPlaceholders["couplelinkNetworkSecurityConfig"] =
            if (localHttpHosts.isEmpty()) "@xml/network_security_config" else "@xml/couplelink_network_security"
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "app.couplelink.mobile"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        // Uses the version code from pubspec.yaml. When using split APKs, 1000 * ABI_VERSION
        // is added automatically by Flutter. (https://developer.android.com/studio/build/configure-apk-splits#configure-APK-versions)
        // You can force using the value of versionCode by specifying the `-P force-version-code-ignoring-abi=true`
        // flag during build.
        versionCode = flutter.versionCode
        versionName = rootProject.file("../../../VERSION").readText().trim()
    }

    val signingProperties = Properties()
    val signingFile = rootProject.file("../../../.secrets/signing.properties")
    if (signingFile.exists()) signingFile.inputStream().use { signingProperties.load(it) }
    signingConfigs {
        create("couplelink") {
            if (signingFile.exists()) {
                storeFile = rootProject.file(signingProperties.getProperty("storeFile"))
                storePassword = signingProperties.getProperty("storePassword")
                keyAlias = signingProperties.getProperty("keyAlias")
                keyPassword = signingProperties.getProperty("keyPassword")
            }
        }
    }
    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("couplelink")
        }
    }
}

androidComponents.onVariants { variant ->
    variant.sources.res?.addGeneratedSourceDirectory(generateLocalNetwork, GenerateCoupleLinkNetworkSecurity::outputDirectory)
}

if (!rootProject.file("../../../.secrets/signing.properties").exists() &&
    gradle.startParameter.taskNames.any { it.contains("release", ignoreCase = true) }) {
    throw GradleException("Release signing is required. Run scripts/prepare-signing.ps1 first.")
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
