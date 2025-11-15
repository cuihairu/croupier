package com.croupier.sdk;

/**
 * Function descriptor aligned with control.proto definition
 */
public class FunctionDescriptor {
    private String id;        // function id, e.g. "player.ban"
    private String version;   // semver, e.g. "1.2.0"
    private String category;  // grouping category
    private String risk;      // "low"|"medium"|"high"
    private String entity;    // entity type, e.g. "item", "player"
    private String operation; // operation type, e.g. "create", "read", "update", "delete"
    private boolean enabled = true; // whether this function is currently enabled

    public FunctionDescriptor() {}

    public FunctionDescriptor(String id, String version) {
        this.id = id;
        this.version = version;
        this.enabled = true;
    }

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getVersion() { return version; }
    public void setVersion(String version) { this.version = version; }

    public String getCategory() { return category; }
    public void setCategory(String category) { this.category = category; }

    public String getRisk() { return risk; }
    public void setRisk(String risk) { this.risk = risk; }

    public String getEntity() { return entity; }
    public void setEntity(String entity) { this.entity = entity; }

    public String getOperation() { return operation; }
    public void setOperation(String operation) { this.operation = operation; }

    public boolean isEnabled() { return enabled; }
    public void setEnabled(boolean enabled) { this.enabled = enabled; }

    @Override
    public String toString() {
        return String.format("FunctionDescriptor{id='%s', version='%s', category='%s', risk='%s', entity='%s', operation='%s', enabled=%s}",
                id, version, category, risk, entity, operation, enabled);
    }
}