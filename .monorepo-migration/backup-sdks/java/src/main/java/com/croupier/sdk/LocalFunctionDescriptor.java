package com.croupier.sdk;

/**
 * Local function descriptor for SDK->Agent registration
 * Aligned with agent/local/v1/local.proto
 */
public class LocalFunctionDescriptor {
    private String id;      // function id
    private String version; // function version

    public LocalFunctionDescriptor() {}

    public LocalFunctionDescriptor(String id, String version) {
        this.id = id;
        this.version = version;
    }

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getVersion() { return version; }
    public void setVersion(String version) { this.version = version; }

    @Override
    public String toString() {
        return String.format("LocalFunctionDescriptor{id='%s', version='%s'}", id, version);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        LocalFunctionDescriptor that = (LocalFunctionDescriptor) o;
        return id.equals(that.id) && version.equals(that.version);
    }

    @Override
    public int hashCode() {
        return java.util.Objects.hash(id, version);
    }
}