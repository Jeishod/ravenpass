import com.github.jk1.license.filter.DependencyFilter
import com.github.jk1.license.filter.SpdxLicenseBundleNormalizer
import com.github.jk1.license.render.ReportRenderer
import com.github.jk1.license.render.TextReportRenderer

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.license.report)
}

val appVersion: String = providers
    .fileContents(rootProject.layout.projectDirectory.file("../../../../version.txt"))
    .asText
    .orNull
    ?.trim()
    ?: throw GradleException("version.txt is missing from the repository root")

// versionCode = MAJOR * 1_000_000 + MINOR * 1_000 + PATCH; Google Play caps it at 2_100_000_000.
val appVersionCode: Int = Regex("""(\d{1,4})\.(\d{1,3})\.(\d{1,3})""")
    .matchEntire(appVersion)
    ?.groupValues
    ?.drop(1)
    ?.map(String::toInt)
    ?.takeIf { (major) -> major < 2100 }
    ?.let { (major, minor, patch) -> major * 1_000_000 + minor * 1_000 + patch }
    ?: throw GradleException(
        "version.txt holds \"$appVersion\"; it must be MAJOR.MINOR.PATCH with MAJOR below 2100 " +
            "and MINOR and PATCH below 1000",
    )

val releaseSigning: Map<String, String?> = listOf(
    "RAVENPASS_ANDROID_KEYSTORE",
    "RAVENPASS_ANDROID_KEYSTORE_PASSWORD",
    "RAVENPASS_ANDROID_KEY_ALIAS",
    "RAVENPASS_ANDROID_KEY_PASSWORD",
).associateWith { providers.environmentVariable(it).orNull?.takeIf(String::isNotEmpty) }

android {
    namespace = "com.wails.app"
    compileSdk = 36
    ndkVersion = "29.0.14206865"

    defaultConfig {
        applicationId = "com.dortanes.ravenpass"
        // Keystore key agreement (KeyProperties.PURPOSE_AGREE_KEY) needs API 31.
        minSdk = 31
        targetSdk = 36
        versionCode = appVersionCode
        versionName = appVersion

        ndk {
            abiFilters += "arm64-v8a"
        }
    }

    signingConfigs {
        create("release") {
            releaseSigning["RAVENPASS_ANDROID_KEYSTORE"]?.let { storeFile = file(it) }
            storePassword = releaseSigning["RAVENPASS_ANDROID_KEYSTORE_PASSWORD"]
            keyAlias = releaseSigning["RAVENPASS_ANDROID_KEY_ALIAS"]
            keyPassword = releaseSigning["RAVENPASS_ANDROID_KEY_PASSWORD"]
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("release")
        }
    }

    buildFeatures {
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    bundle {
        language {
            // The app switches to the language chosen in Ravenpass at run time, so every language ships.
            enableSplit = false
        }
    }

    packaging {
        jniLibs {
            // The Go build strips the release libwails.so itself; Gradle never needs the NDK to strip it.
            keepDebugSymbols += "**/libwails.so"
        }
    }
}

dependencies {
    implementation(libs.androidx.appcompat)
    implementation(libs.androidx.autofill)
    implementation(libs.androidx.biometric)
    implementation(libs.androidx.credentials)
    implementation(libs.androidx.webkit)
}

licenseReport {
    configurations = arrayOf("releaseRuntimeClasspath")
    filters = arrayOf<DependencyFilter>(SpdxLicenseBundleNormalizer())
    renderers = arrayOf<ReportRenderer>(TextReportRenderer())
    // The Android notices reproduce only the Apache-2.0 text.
    allowedLicensesFile = rootProject.layout.projectDirectory.file("../../../../notices/allowed-licenses-android.json").asFile
}

val requireWailsLibrary = tasks.register("requireWailsLibrary") {
    val library = layout.projectDirectory.file("src/main/jniLibs/arm64-v8a/libwails.so").asFile
    doLast {
        if (!library.isFile) {
            throw GradleException(
                "$library is missing: build the Go library first with `make -C apps/mobile lib` " +
                    "or `make -C apps/mobile lib-release`",
            )
        }
    }
}

tasks.matching { it.name == "preBuild" }.configureEach { dependsOn(requireWailsLibrary) }

val requireReleaseSigning = tasks.register("requireReleaseSigning") {
    val missing = releaseSigning.filterValues { it == null }.keys.toList()
    doLast {
        if (missing.isNotEmpty()) {
            throw GradleException("Signing the release needs these environment variables: ${missing.joinToString()}")
        }
    }
}

tasks.matching { it.name == "validateSigningRelease" }.configureEach { dependsOn(requireReleaseSigning) }
