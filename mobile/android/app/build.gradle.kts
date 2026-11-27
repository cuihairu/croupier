import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// Managed release signing (AUD-085): production artifacts must be signed with
// the managed keystore supplied through the environment (or -P gradle
// properties); the debug-signed artifact is only produced when the build
// explicitly declares itself a rehearsal (CROUPIER_REHEARSAL_BUILD=1). A
// release build with neither fails loudly instead of silently shipping
// debug-signed.
val releaseKeystorePath = System.getenv("CROUPIER_RELEASE_KEYSTORE")
    ?: providers.gradleProperty("croupierReleaseKeystore").orNull
val releaseKeystorePassword = System.getenv("CROUPIER_RELEASE_KEYSTORE_PASSWORD")
    ?: providers.gradleProperty("croupierReleaseKeystorePassword").orNull
val releaseKeyAlias = System.getenv("CROUPIER_RELEASE_KEY_ALIAS")
    ?: providers.gradleProperty("croupierReleaseKeyAlias").orNull
val releaseKeyPassword = System.getenv("CROUPIER_RELEASE_KEY_PASSWORD")
    ?: providers.gradleProperty("croupierReleaseKeyPassword").orNull
val rehearsalBuild = System.getenv("CROUPIER_REHEARSAL_BUILD") == "1" ||
    providers.gradleProperty("croupierRehearsalBuild").orNull == "1"

android {
    namespace = "com.cuihairu.croupier_mobile"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }

    defaultConfig {
        // TODO: Specify your own unique Application ID (https://developer.android.com/studio/build/application-id.html).
        applicationId = "com.cuihairu.croupier_mobile"
        // You can update the following values to match your application needs.
        // For more information, see: https://flutter.dev/to/review-gradle-config.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    signingConfigs {
        releaseKeystorePath?.let { path ->
            create("managedRelease") {
                storeFile = file(path)
                storePassword = releaseKeystorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        release {
            // AUD-085: managed signing for production; debug signing only as
            // the explicitly flagged rehearsal artifact (CI names it
            // croupier-mobile-rehearsal-debugsigned-apk). The task-graph
            // guard below rejects unflagged release builds with neither.
            signingConfig = signingConfigs.findByName("managedRelease")
                ?: signingConfigs.getByName("debug")
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget.set(JvmTarget.JVM_11)
    }
}

// AUD-085 fail-fast: enforced in doFirst of the release entry tasks, so
// debug builds and IDE sync stay unaffected; an unflagged release build
// without the managed keystore aborts before packaging.
afterEvaluate {
    // mirrors the signingConfigs creation condition above: the managed
    // config exists exactly when a keystore path was supplied.
    val managedSigningPresent = releaseKeystorePath != null
    listOf("assembleRelease", "bundleRelease", "packageRelease").forEach { taskName ->
        tasks.matching { it.name == taskName }.configureEach {
            doFirst {
                if (!managedSigningPresent && !rehearsalBuild) {
                    throw GradleException(
                        "release build has no managed signing config: set CROUPIER_RELEASE_KEYSTORE (+ " +
                            "CROUPIER_RELEASE_KEYSTORE_PASSWORD / CROUPIER_RELEASE_KEY_ALIAS / CROUPIER_RELEASE_KEY_PASSWORD), " +
                            "or set CROUPIER_REHEARSAL_BUILD=1 to produce the debug-signed rehearsal artifact"
                    )
                }
                if (!managedSigningPresent && rehearsalBuild) {
                    logger.warn("croupier: rehearsal build — app-release.apk is DEBUG-signed, not for distribution")
                }
            }
        }
    }
}

flutter {
    source = "../.."
}
