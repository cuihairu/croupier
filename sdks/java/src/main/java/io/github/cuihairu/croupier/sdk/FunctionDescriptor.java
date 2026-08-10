package io.github.cuihairu.croupier.sdk;

import java.util.ArrayList;
import java.util.List;

/**
 * Function descriptor aligned with control.proto definition
 */
public class FunctionDescriptor {
    private String id;        // function id, e.g. "player.ban"
    private String version;   // semver, e.g. "1.2.0"
    private List<String> tags = new ArrayList<>();
    private String summary;
    private String description;
    private String operationId;
    private boolean deprecated;
    private String inputSchema;
    private String outputSchema;
    private String resource;  // business resource/capability key
    private String operation; // business action key, e.g. "ban", "send", "list"
    private String capability; // collection_query|item_query|create|update|delete|action|task|report
    private String execution;  // sync|task
    private boolean approvalRequired;
    private String approvalPolicyKey;
    private String risk;      // "safe"|"warning"|"high"|"danger"
    private String permission;
    private boolean enabled = true; // whether this function is currently enabled

    public FunctionDescriptor() {}

    public FunctionDescriptor(String id, String version) {
        this.id = id;
        this.version = version;
        this.enabled = true;
    }

    /**
     * Copy constructor for creating a deep copy of a descriptor
     */
    public FunctionDescriptor(FunctionDescriptor other) {
        this.id = other.id;
        this.version = other.version;
        this.tags = other.tags != null ? new ArrayList<>(other.tags) : null;
        this.summary = other.summary;
        this.description = other.description;
        this.operationId = other.operationId;
        this.deprecated = other.deprecated;
        this.inputSchema = other.inputSchema;
        this.outputSchema = other.outputSchema;
        this.resource = other.resource;
        this.operation = other.operation;
        this.capability = other.capability;
        this.execution = other.execution;
        this.approvalRequired = other.approvalRequired;
        this.approvalPolicyKey = other.approvalPolicyKey;
        this.risk = other.risk;
        this.permission = other.permission;
        this.enabled = other.enabled;
    }

    // Getters and setters
    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getVersion() { return version; }
    public void setVersion(String version) { this.version = version; }

    public List<String> getTags() { return tags; }
    public void setTags(List<String> tags) { this.tags = tags; }

    public String getSummary() { return summary; }
    public void setSummary(String summary) { this.summary = summary; }

    public String getDescription() { return description; }
    public void setDescription(String description) { this.description = description; }

    public String getOperationId() { return operationId; }
    public void setOperationId(String operationId) { this.operationId = operationId; }

    public boolean isDeprecated() { return deprecated; }
    public void setDeprecated(boolean deprecated) { this.deprecated = deprecated; }

    public String getInputSchema() { return inputSchema; }
    public void setInputSchema(String inputSchema) { this.inputSchema = inputSchema; }

    public String getOutputSchema() { return outputSchema; }
    public void setOutputSchema(String outputSchema) { this.outputSchema = outputSchema; }

    public String getResource() { return resource; }
    public void setResource(String resource) { this.resource = resource; }

    public String getRisk() { return risk; }
    public void setRisk(String risk) { this.risk = risk; }

    public String getOperation() { return operation; }
    public void setOperation(String operation) { this.operation = operation; }

    public String getCapability() { return capability; }
    public void setCapability(String capability) { this.capability = capability; }

    public String getExecution() { return execution; }
    public void setExecution(String execution) { this.execution = execution; }

    public boolean isApprovalRequired() { return approvalRequired; }
    public void setApprovalRequired(boolean approvalRequired) { this.approvalRequired = approvalRequired; }

    public String getApprovalPolicyKey() { return approvalPolicyKey; }
    public void setApprovalPolicyKey(String approvalPolicyKey) { this.approvalPolicyKey = approvalPolicyKey; }

    public String getPermission() { return permission; }
    public void setPermission(String permission) { this.permission = permission; }

    public boolean isEnabled() { return enabled; }
    public void setEnabled(boolean enabled) { this.enabled = enabled; }

    @Override
    public String toString() {
        return String.format(
            "FunctionDescriptor{id='%s', version='%s', summary='%s', resource='%s', operation='%s', risk='%s', enabled=%s}",
            id, version, summary, resource, operation, risk, enabled
        );
    }
}
