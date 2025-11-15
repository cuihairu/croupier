package com.croupier.sdk.scripts;

import java.io.*;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;

/**
 * Proto generator for CI builds
 * Downloads proto files and generates Java gRPC code
 */
public class ProtoGenerator {
    private static final String[] PROTO_FILES = {
        "croupier/agent/local/v1/local.proto",
        "croupier/control/v1/control.proto",
        "croupier/function/v1/function.proto",
        "croupier/edge/job/v1/job.proto",
        "croupier/tunnel/v1/tunnel.proto",
        "croupier/options/ui.proto",
        "croupier/options/function.proto"
    };

    public static void main(String[] args) {
        System.out.println("Croupier Java SDK Proto Generator");
        System.out.println("==================================");

        // Check if we're in CI or local development
        if (!isCI()) {
            System.out.println("Local development build detected, using mock gRPC implementation");
            System.out.println("To enable real gRPC, set CROUPIER_CI_BUILD=1");
            return;
        }

        System.out.println("CI build detected, enabling proto generation...");

        try {
            String branch = System.getenv("CROUPIER_PROTO_BRANCH");
            if (branch == null || branch.isEmpty()) {
                branch = "main";
            }

            String protoDir = "downloaded_proto";
            downloadProtoFiles(protoDir, branch);

            // The protobuf-maven-plugin will handle the actual code generation
            // This script just downloads the proto files

            System.out.println("CI build setup completed with proto download");
            System.out.println("Maven protobuf plugin will generate the gRPC code");

        } catch (Exception e) {
            System.err.println("Proto generation failed: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }

    private static boolean isCI() {
        String ci = System.getenv("CI");
        String croupierCI = System.getenv("CROUPIER_CI_BUILD");
        return (ci != null && !ci.isEmpty()) || (croupierCI != null && !croupierCI.isEmpty());
    }

    private static void downloadProtoFiles(String protoDir, String branch) throws IOException {
        String baseURL = String.format("https://raw.githubusercontent.com/cuihairu/croupier/%s/proto", branch);

        System.out.printf("Downloading %d proto files to %s...%n", PROTO_FILES.length, protoDir);

        for (String protoFile : PROTO_FILES) {
            String url = String.format("%s/%s", baseURL, protoFile);
            Path destPath = Paths.get(protoDir, protoFile);

            System.out.printf("Downloading: %s%n", url);
            downloadFile(url, destPath);
            System.out.printf("Downloaded: %s%n", protoFile);
        }

        System.out.println("Proto files downloaded successfully");
    }

    private static void downloadFile(String urlString, Path destPath) throws IOException {
        // Create parent directories
        Files.createDirectories(destPath.getParent());

        URL url = new URL(urlString);
        HttpURLConnection connection = (HttpURLConnection) url.openConnection();
        connection.setRequestMethod("GET");

        int responseCode = connection.getResponseCode();
        if (responseCode != HttpURLConnection.HTTP_OK) {
            throw new IOException("Download failed with status " + responseCode + " for " + urlString);
        }

        try (InputStream inputStream = connection.getInputStream();
             FileOutputStream outputStream = new FileOutputStream(destPath.toFile())) {

            byte[] buffer = new byte[8192];
            int bytesRead;
            while ((bytesRead = inputStream.read(buffer)) != -1) {
                outputStream.write(buffer, 0, bytesRead);
            }
        }
    }
}