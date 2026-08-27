package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.Json;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.function.Function;

/**
 * OpenAPI 3 import helper mirroring the Go SDK's
 * {@code function.RegisterFromOpenAPI}: parses an OpenAPI 3 spec, converts
 * every operation into a {@link FunctionDescriptor} and registers it on a
 * {@link CroupierClient} with a caller-supplied handler.
 */
public final class OpenAPIImporter {

    private OpenAPIImporter() {
    }

    /** Controls OpenAPI import behaviour (mirrors Go ImportOptions). */
    public static final class ImportOptions {
        /** Prefix prepended to every imported resource (e.g. "game"). */
        private String resourcePrefix = "";
        /** Prefix prepended to every imported tag. */
        private String tagPrefix = "";
        /** Default timeout in milliseconds (Go parity; the Java descriptor carries no timeout field yet). */
        private long defaultTimeoutMs = 0;
        /** Keep importing remaining operations when one fails. */
        private boolean continueOnError = false;

        public ImportOptions resourcePrefix(String resourcePrefix) {
            this.resourcePrefix = resourcePrefix == null ? "" : resourcePrefix;
            return this;
        }

        public ImportOptions tagPrefix(String tagPrefix) {
            this.tagPrefix = tagPrefix == null ? "" : tagPrefix;
            return this;
        }

        public ImportOptions defaultTimeoutMs(long defaultTimeoutMs) {
            this.defaultTimeoutMs = defaultTimeoutMs;
            return this;
        }

        public ImportOptions continueOnError(boolean continueOnError) {
            this.continueOnError = continueOnError;
            return this;
        }
    }

    private static final List<String> OPERATION_METHODS = List.of(
        "get", "put", "post", "delete", "options", "head", "patch", "trace");

    /**
     * Imports an OpenAPI 3 spec, registering every operation on {@code client}.
     *
     * @param client      target SDK client
     * @param spec        OpenAPI 3 JSON document
     * @param options     import options (may be {@code null})
     * @param resolver    supplies a handler for a derived function ID; returning
     *                    {@code null} marks the operation as unhandled
     * @return the list of registered function IDs
     * @throws CroupierException wrapping parse/registration failures
     */
    public static List<String> registerFromOpenAPI(
        CroupierClient client,
        String spec,
        ImportOptions options,
        Function<String, FunctionHandler> resolver) throws CroupierException {

        Object documentRaw;
        try {
            documentRaw = Json.parse(spec);
        } catch (IllegalArgumentException exception) {
            throw new CroupierException("load OpenAPI spec failed: " + exception.getMessage(), exception);
        }
        if (!(documentRaw instanceof Map<?, ?> document) || !(document.get("paths") instanceof Map<?, ?>)) {
            throw new CroupierException("OpenAPI spec must be an object containing 'paths'");
        }

        List<String> registered = new ArrayList<>();
        for (Map.Entry<?, ?> pathEntry : ((Map<?, ?>) document.get("paths")).entrySet()) {
            if (!(pathEntry.getKey() instanceof String path) || !(pathEntry.getValue() instanceof Map<?, ?> pathItem)) {
                continue;
            }
            for (String method : OPERATION_METHODS) {
                if (!(pathItem.get(method) instanceof Map<?, ?> operation)) {
                    continue;
                }
                @SuppressWarnings("unchecked")
                Map<String, Object> operationMap = (Map<String, Object>) operation;
                FunctionDescriptor descriptor = operationToDescriptor(path, operationMap, options);
                FunctionHandler handler = resolver.apply(descriptor.getId());
                if (handler == null) {
                    if (options != null && options.continueOnError) {
                        continue;
                    }
                    throw new CroupierException("no handler provided for function: " + descriptor.getId());
                }
                try {
                    client.registerFunction(descriptor, handler);
                } catch (Exception exception) {
                    if (options != null && options.continueOnError) {
                        continue;
                    }
                    throw new CroupierException("register function " + descriptor.getId() + " failed: " + exception.getMessage(), exception);
                }
                registered.add(descriptor.getId());
            }
        }
        return registered;
    }

    /**
     * Imports an OpenAPI 3 spec using an explicit handler map
     * (mirrors Go RegisterFromOpenAPIWithHandlers).
     */
    public static List<String> registerFromOpenAPIWithHandlers(
        CroupierClient client,
        String spec,
        ImportOptions options,
        Map<String, FunctionHandler> handlers) throws CroupierException {
        return registerFromOpenAPI(client, spec, options, handlers::get);
    }

    // ------------------------------------------------------------------

    @SuppressWarnings("unchecked")
    private static FunctionDescriptor operationToDescriptor(
        String path, Map<String, Object> operation, ImportOptions options) {

        String functionId = deriveOperationId(operation, path);
        FunctionDescriptor descriptor = new FunctionDescriptor();
        descriptor.setId(functionId);
        descriptor.setSummary(deriveSummary(operation, functionId));
        descriptor.setDescription(stringOrNull(operation.get("description")));
        descriptor.setTags(stringTags(operation.get("tags")));
        descriptor.setResource(emptyToNull(extractExtension(operation, "x-resource")));
        descriptor.setOperation(emptyToNull(extractExtension(operation, "x-operation")));
        descriptor.setPermission(emptyToNull(extractExtension(operation, "x-permission")));
        descriptor.setCapability(emptyToNull(extractExtension(operation, "x-capability")));
        descriptor.setExecution(emptyToNull(extractExtension(operation, "x-execution")));
        applyApprovalExtension(descriptor, operation.get("x-approval"));

        String inputSchema = jsonContentSchema(operation.get("requestBody"));
        if (inputSchema != null) {
            descriptor.setInputSchema(inputSchema);
        }
        if (operation.get("responses") instanceof Map<?, ?> responses
            && responses.get("200") instanceof Map<?, ?> ok) {
            String outputSchema = jsonContentSchema(ok);
            if (outputSchema != null) {
                descriptor.setOutputSchema(outputSchema);
            }
        }

        String risk = extractExtension(operation, "x-risk");
        descriptor.setRisk(risk.isEmpty() ? "warning" : parseRiskLevel(risk));

        if (options != null) {
            if (!options.resourcePrefix.isEmpty() && descriptor.getResource() != null) {
                descriptor.setResource(options.resourcePrefix + "." + descriptor.getResource());
            }
            if (!options.tagPrefix.isEmpty()) {
                List<String> prefixed = new ArrayList<>();
                for (String tag : descriptor.getTags()) {
                    prefixed.add(options.tagPrefix + tag);
                }
                descriptor.setTags(prefixed);
            }
        }
        return descriptor;
    }

    static String deriveOperationId(Map<String, Object> operation, String path) {
        String operationId = stringOrNull(operation.get("operationId"));
        if (operationId != null && !operationId.isEmpty()) {
            return operationId;
        }
        if (path != null && !path.isEmpty()) {
            List<String> segments = new ArrayList<>();
            for (String segment : path.split("/")) {
                if (!segment.isEmpty()) {
                    segments.add(segment);
                }
            }
            if (!segments.isEmpty()) {
                return String.join(".", segments);
            }
        }
        return "unknown.function";
    }

    static String deriveSummary(Map<String, Object> operation, String functionId) {
        String summary = stringOrNull(operation.get("summary"));
        if (summary != null && !summary.isEmpty()) {
            return summary;
        }
        if (functionId != null && !"unknown.function".equals(functionId)) {
            return toTitleCase(functionId);
        }
        return "Unnamed Function";
    }

    static String toTitleCase(String value) {
        String[] words = value.split("_");
        StringBuilder output = new StringBuilder();
        for (int i = 0; i < words.length; i++) {
            if (i > 0) {
                output.append(' ');
            }
            String word = words[i];
            if (!word.isEmpty()) {
                output.append(Character.toUpperCase(word.charAt(0)));
                if (word.length() > 1) {
                    output.append(word.substring(1).toLowerCase(Locale.ROOT));
                }
            }
        }
        return output.toString();
    }

    /** Shallow OpenAPI-schema -> JSON-Schema conversion (Go parity). */
    @SuppressWarnings("unchecked")
    private static String jsonContentSchema(Object holder) {
        if (!(holder instanceof Map<?, ?> holderMap) || !(holderMap.get("content") instanceof Map<?, ?> content)) {
            return null;
        }
        if (!(content.get("application/json") instanceof Map<?, ?> media) || !(media.get("schema") instanceof Map<?, ?> schema)) {
            return null;
        }
        Map<String, Object> schemaMap = (Map<String, Object>) schema;
        if (schemaMap.isEmpty()) {
            return null;
        }
        Map<String, Object> result = new java.util.LinkedHashMap<>();
        String type = stringOrNull(schemaMap.get("type"));
        if (type != null && !type.isEmpty()) {
            result.put("type", type);
        }
        String description = stringOrNull(schemaMap.get("description"));
        if (description != null && !description.isEmpty()) {
            result.put("description", description);
        }
        if (schemaMap.get("properties") instanceof Map<?, ?> properties) {
            Map<String, Object> props = new java.util.LinkedHashMap<>();
            for (Map.Entry<?, ?> entry : properties.entrySet()) {
                if (entry.getKey() instanceof String name && entry.getValue() instanceof Map<?, ?> rawProp) {
                    Map<String, Object> entry_ = new java.util.LinkedHashMap<>();
                    String propType = stringOrNull(((Map<String, Object>) rawProp).get("type"));
                    entry_.put("type", propType == null || propType.isEmpty() ? "object" : propType);
                    String propDescription = stringOrNull(((Map<String, Object>) rawProp).get("description"));
                    if (propDescription != null && !propDescription.isEmpty()) {
                        entry_.put("description", propDescription);
                    }
                    props.put(name, entry_);
                }
            }
            result.put("properties", props);
        }
        if (schemaMap.get("required") instanceof List<?> required && !required.isEmpty()) {
            result.put("required", required);
        }
        if (result.isEmpty()) {
            return null;
        }
        return Json.stringify(result);
    }

    static String extractExtension(Map<String, Object> operation, String key) {
        Object value = operation.get(key);
        if (value == null) {
            return "";
        }
        if (value instanceof String text) {
            return text;
        }
        if (value instanceof Boolean flag) {
            return flag ? "true" : "false";
        }
        return Json.stringify(value);
    }

    static String parseRiskLevel(String level) {
        String normalized = level.toLowerCase(Locale.ROOT);
        switch (normalized) {
            case "low":
            case "safe":
                return "safe";
            case "medium":
            case "moderate":
            case "warning":
                return "warning";
            case "high":
                return "high";
            case "danger":
            case "critical":
                return "danger";
            default:
                return "warning";
        }
    }

    /** Maps {@code x-approval: {required, policyKey}} onto descriptor v2 fields. */
    private static void applyApprovalExtension(FunctionDescriptor descriptor, Object approval) {
        if (!(approval instanceof Map<?, ?> approvalMap)) {
            return;
        }
        Object required = approvalMap.get("required");
        if (required instanceof Boolean flag) {
            descriptor.setApprovalRequired(flag);
        } else if (required instanceof String text && !text.isEmpty()) {
            descriptor.setApprovalRequired(Boolean.parseBoolean(text));
        }
        if (approvalMap.get("policyKey") instanceof String policyKey && !policyKey.isEmpty()) {
            descriptor.setApprovalPolicyKey(policyKey);
        }
    }

    private static String stringOrNull(Object value) {
        return value instanceof String text ? text : null;
    }

    private static String emptyToNull(String value) {
        return value == null || value.isEmpty() ? null : value;
    }

    private static List<String> stringTags(Object value) {
        List<String> tags = new ArrayList<>();
        if (value instanceof List<?> list) {
            for (Object item : list) {
                if (item instanceof String tag) {
                    tags.add(tag);
                }
            }
        }
        return tags;
    }
}
