package io.github.cuihairu.croupier.sdk;

/**
 * Local function descriptor for SDK->Agent registration
 * Aligned with agent/local/v1/local.proto
 */
public class LocalFunctionDescriptor {
    private String id;      // function id
    private String version; // function version
    private String capability; // capability contract type
    private String execution;  // execution mode

    public LocalFunctionDescriptor() {}

    public LocalFunctionDescriptor(String id, String version) {
        this.id = id;
        this.version = version;
    }

    public LocalFunctionDescriptor(String id, String version, String capability, String execution) {
        this.id = id;
        this.version = version;
        this.capability = capability;
        this.execution = execution;
    }

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getVersion() { return version; }
    public void setVersion(String version) { this.version = version; }

    public String getCapability() { return capability; }
    public void setCapability(String capability) { this.capability = capability; }

    public String getExecution() { return execution; }
    public void setExecution(String execution) { this.execution = execution; }

    @Override
    public String toString() {
        return String.format(
            "LocalFunctionDescriptor{id='%s', version='%s', capability='%s', execution='%s'}",
            id, version, capability, execution
        );
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        LocalFunctionDescriptor that = (LocalFunctionDescriptor) o;
        return java.util.Objects.equals(id, that.id)
            && java.util.Objects.equals(version, that.version)
            && java.util.Objects.equals(capability, that.capability)
            && java.util.Objects.equals(execution, that.execution);
    }

    @Override
    public int hashCode() {
        return java.util.Objects.hash(id, version, capability, execution);
    }
}
