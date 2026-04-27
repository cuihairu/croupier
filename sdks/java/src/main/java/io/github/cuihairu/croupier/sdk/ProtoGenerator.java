package io.github.cuihairu.croupier.sdk.scripts;

import java.io.*;
import java.nio.file.*;

/**
 * Proto generator for monorepo builds
 * Validates local proto files are available
 */
public class ProtoGenerator {
    public static void main(String[] args) {
        System.out.println("Croupier Java SDK Proto Generator");
        System.out.println("==================================");

        // Check if we're in CI or local development
        if (!isCI()) {
            System.out.println("Local development build detected, using mock implementation");
            System.out.println("To enable real proto generation, set CROUPIER_CI_BUILD=1");
            return;
        }

        System.out.println("CI build detected, validating proto files...");

        try {
            // Check if proto directory exists in monorepo
            Path protoDir = Paths.get("../../proto");
            if (!Files.exists(protoDir)) {
                // Try relative path from sdks/java
                protoDir = Paths.get("../../../proto");
            }

            if (Files.exists(protoDir) && Files.isDirectory(protoDir)) {
                System.out.println("✅ Found monorepo proto directory at: " + protoDir.toAbsolutePath());

                // List proto files
                long protoCount = Files.walk(protoDir)
                    .filter(p -> p.toString().endsWith(".proto"))
                    .count();

                System.out.println("✅ Found " + protoCount + " proto files");

                // The protobuf-maven-plugin will handle the actual code generation
                System.out.println("CI build setup completed");
                System.out.println("Maven protobuf plugin will generate the protobuf code");
            } else {
                System.err.println("❌ Proto directory not found. Expected at: " + protoDir.toAbsolutePath());
                System.err.println("This should not happen in a monorepo setup!");
                System.exit(1);
            }

        } catch (Exception e) {
            System.err.println("Proto validation failed: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }

    private static boolean isCI() {
        String ci = System.getenv("CI");
        String croupierCI = System.getenv("CROUPIER_CI_BUILD");
        return (ci != null && !ci.isEmpty()) || (croupierCI != null && !croupierCI.isEmpty());
    }
}
