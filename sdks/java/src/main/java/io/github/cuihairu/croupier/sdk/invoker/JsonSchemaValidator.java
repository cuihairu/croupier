package io.github.cuihairu.croupier.sdk.invoker;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;
import java.util.regex.PatternSyntaxException;

/**
 * Local JSON Schema (Draft-07 subset) validator used by the invoker for
 * pre-flight payload validation, mirroring the Go SDK's Draft7 behaviour for
 * the commonly used keywords:
 *
 * <ul>
 *   <li>{@code type} (object, array, string, number, integer, boolean, null)</li>
 *   <li>{@code enum}, {@code const}</li>
 *   <li>{@code properties}, {@code required}, {@code additionalProperties}</li>
 *   <li>{@code items} (schema form), {@code minItems}, {@code maxItems}, {@code uniqueItems}</li>
 *   <li>{@code minimum}, {@code maximum}, {@code exclusiveMinimum},
 *       {@code exclusiveMaximum}, {@code multipleOf}</li>
 *   <li>{@code minLength}, {@code maxLength}, {@code pattern}</li>
 *   <li>local {@code $ref} pointers of the form {@code #/...}</li>
 * </ul>
 *
 * Keywords outside this subset are ignored; the Server remains the
 * authoritative validator.
 */
public final class JsonSchemaValidator {

    private JsonSchemaValidator() {
    }

    /**
     * Validates {@code value} against {@code schema}.
     *
     * @param schema parsed JSON schema (result of {@link Json#parse(String)})
     * @param value  parsed JSON value
     * @return human-readable error messages; empty when the value is valid
     */
    public static List<String> validate(Object schema, Object value) {
        List<String> errors = new ArrayList<>();
        validate(schema, schema, value, "$", errors, new HashSet<>());
        return errors;
    }

    /**
     * Validates a JSON payload string against a schema JSON string.
     *
     * @return error messages; empty when the payload is valid
     */
    public static List<String> validate(String payloadJson, String schemaJson) {
        Object schema = Json.parse(schemaJson);
        Object value = Json.parse(payloadJson);
        return validate(schema, value);
    }

    public static boolean isValid(Object schema, Object value) {
        return validate(schema, value).isEmpty();
    }

    // ------------------------------------------------------------------

    @SuppressWarnings("unchecked")
    private static void validate(Object root, Object schema, Object value, String path, List<String> errors, Set<Object> activeRefs) {
        if (!(schema instanceof Map<?, ?> schemaMap)) {
            return; // boolean schemas: true passes, false is unusual for payloads — treat as pass
        }
        Map<String, Object> map = (Map<String, Object>) schemaMap;

        Object ref = map.get("$ref");
        if (ref instanceof String refText && refText.startsWith("#/") && activeRefs.add(refText)) {
            Object resolved = resolvePointer(root, refText);
            if (resolved != null) {
                validate(root, resolved, value, path, errors, activeRefs);
            } else {
                errors.add(path + ": unresolved $ref '" + refText + "'");
            }
            return;
        }

        checkType(map.get("type"), value, path, errors);
        checkEnum(map.get("enum"), map.get("const"), value, path, errors);
        checkNumeric(map, value, path, errors);
        checkString(map, value, path, errors);
        checkArray(map, value, path, errors, root);
        checkObject(map, value, path, errors, root);
    }

    private static void checkType(Object typeSpec, Object value, String path, List<String> errors) {
        if (typeSpec == null) {
            return;
        }
        List<String> allowed = new ArrayList<>();
        if (typeSpec instanceof String single) {
            allowed.add(single);
        } else if (typeSpec instanceof List<?> list) {
            for (Object item : list) {
                if (item instanceof String name) {
                    allowed.add(name);
                }
            }
        }
        if (allowed.isEmpty()) {
            return;
        }
        for (String candidate : allowed) {
            if (matchesType(candidate, value)) {
                return;
            }
        }
        errors.add(path + ": expected type " + allowed + " but found " + jsonTypeName(value));
    }

    private static boolean matchesType(String type, Object value) {
        return switch (type) {
            case "object" -> value instanceof Map<?, ?>;
            case "array" -> value instanceof List<?>;
            case "string" -> value instanceof String;
            case "boolean" -> value instanceof Boolean;
            case "null" -> value == null;
            case "number" -> value instanceof Number;
            case "integer" -> isInteger(value);
            default -> true; // unknown type names are ignored (Server validates)
        };
    }

    private static boolean isInteger(Object value) {
        if (value instanceof Long) {
            return true;
        }
        if (value instanceof Double number) {
            return !number.isInfinite() && !number.isNaN() && number == Math.rint(number);
        }
        return false;
    }

    private static String jsonTypeName(Object value) {
        if (value == null) return "null";
        if (value instanceof Map<?, ?>) return "object";
        if (value instanceof List<?>) return "array";
        if (value instanceof String) return "string";
        if (value instanceof Boolean) return "boolean";
        if (value instanceof Number) return "number";
        return value.getClass().getSimpleName();
    }

    private static void checkEnum(Object enumSpec, Object constSpec, Object value, String path, List<String> errors) {
        boolean checked = false;
        if (enumSpec instanceof List<?> options) {
            checked = true;
            boolean matched = options.stream().anyMatch(option -> jsonEquals(option, value));
            if (!matched) {
                errors.add(path + ": value is not one of the allowed enum values");
            }
        }
        if (constSpec != null) {
            checked = true;
            if (!jsonEquals(constSpec, value)) {
                errors.add(path + ": value does not match the const value");
            }
        }
    }

    private static boolean jsonEquals(Object left, Object right) {
        if (left instanceof Number ln && right instanceof Number rn) {
            return ln.doubleValue() == rn.doubleValue();
        }
        return java.util.Objects.equals(left, right);
    }

    private static void checkNumeric(Map<String, Object> map, Object value, String path, List<String> errors) {
        if (!(value instanceof Number number)) {
            return;
        }
        double actual = number.doubleValue();
        Object minimum = map.get("minimum");
        if (minimum instanceof Number min && actual < min.doubleValue()) {
            errors.add(path + ": value " + actual + " is less than minimum " + min.doubleValue());
        }
        Object maximum = map.get("maximum");
        if (maximum instanceof Number max && actual > max.doubleValue()) {
            errors.add(path + ": value " + actual + " is greater than maximum " + max.doubleValue());
        }
        Object exclusiveMinimum = map.get("exclusiveMinimum");
        if (exclusiveMinimum instanceof Number exclMin && actual <= exclMin.doubleValue()) {
            errors.add(path + ": value " + actual + " must be greater than " + exclMin.doubleValue());
        }
        Object exclusiveMaximum = map.get("exclusiveMaximum");
        if (exclusiveMaximum instanceof Number exclMax && actual >= exclMax.doubleValue()) {
            errors.add(path + ": value " + actual + " must be less than " + exclMax.doubleValue());
        }
        Object multipleOf = map.get("multipleOf");
        if (multipleOf instanceof Number divisor && divisor.doubleValue() != 0) {
            double quotient = actual / divisor.doubleValue();
            if (Math.abs(quotient - Math.rint(quotient)) > 1e-9) {
                errors.add(path + ": value " + actual + " is not a multiple of " + divisor.doubleValue());
            }
        }
    }

    private static void checkString(Map<String, Object> map, Object value, String path, List<String> errors) {
        if (!(value instanceof String text)) {
            return;
        }
        int length = text.codePointCount(0, text.length());
        Object minLength = map.get("minLength");
        if (minLength instanceof Number min && length < min.intValue()) {
            errors.add(path + ": string length " + length + " is less than minLength " + min.intValue());
        }
        Object maxLength = map.get("maxLength");
        if (maxLength instanceof Number max && length > max.intValue()) {
            errors.add(path + ": string length " + length + " is greater than maxLength " + max.intValue());
        }
        Object pattern = map.get("pattern");
        if (pattern instanceof String patternText) {
            try {
                if (!Pattern.compile(patternText).matcher(text).find()) {
                    errors.add(path + ": string does not match pattern '" + patternText + "'");
                }
            } catch (PatternSyntaxException ignored) {
                // Invalid patterns are ignored; the Server validates authoritatively.
            }
        }
    }

    @SuppressWarnings("unchecked")
    private static void checkArray(Map<String, Object> map, Object value, String path, List<String> errors, Object root) {
        if (!(value instanceof List<?> list)) {
            return;
        }
        Object minItems = map.get("minItems");
        if (minItems instanceof Number min && list.size() < min.intValue()) {
            errors.add(path + ": array has fewer than minItems " + min.intValue());
        }
        Object maxItems = map.get("maxItems");
        if (maxItems instanceof Number max && list.size() > max.intValue()) {
            errors.add(path + ": array has more than maxItems " + max.intValue());
        }
        if (Boolean.TRUE.equals(map.get("uniqueItems"))) {
            Set<String> seen = new HashSet<>();
            for (Object item : list) {
                if (!seen.add(Json.stringify(item))) {
                    errors.add(path + ": array items are not unique");
                    break;
                }
            }
        }
        Object items = map.get("items");
        if (items instanceof Map<?, ?> itemSchema) {
            for (int i = 0; i < list.size(); i++) {
                validate(root, (Object) itemSchema, list.get(i), path + "/" + i, errors, new HashSet<>());
            }
        }
    }

    @SuppressWarnings("unchecked")
    private static void checkObject(Map<String, Object> map, Object value, String path, List<String> errors, Object root) {
        if (!(value instanceof Map<?, ?>)) {
            return;
        }
        Map<String, Object> objectValue = (Map<String, Object>) value;

        Object required = map.get("required");
        if (required instanceof List<?> requiredNames) {
            for (Object name : requiredNames) {
                if (name instanceof String fieldName && !objectValue.containsKey(fieldName)) {
                    errors.add(path + ": missing required property '" + fieldName + "'");
                }
            }
        }

        Object properties = map.get("properties");
        if (properties instanceof Map<?, ?> propertyMap) {
            for (Map.Entry<?, ?> entry : propertyMap.entrySet()) {
                if (entry.getKey() instanceof String fieldName && objectValue.containsKey(fieldName)) {
                    validate(root, entry.getValue(), objectValue.get(fieldName), path + "/" + fieldName, errors, new HashSet<>());
                }
            }
        }

        Object additional = map.get("additionalProperties");
        if (additional != null && properties instanceof Map<?, ?> propertyMap) {
            Set<String> declared = new HashSet<>();
            for (Object key : propertyMap.keySet()) {
                if (key instanceof String name) {
                    declared.add(name);
                }
            }
            for (String fieldName : objectValue.keySet()) {
                if (declared.contains(fieldName)) {
                    continue;
                }
                if (additional instanceof Boolean allowed && !allowed) {
                    errors.add(path + ": additional property '" + fieldName + "' is not allowed");
                } else if (additional instanceof Map<?, ?> additionalSchema) {
                    validate(root, (Object) additionalSchema, objectValue.get(fieldName), path + "/" + fieldName, errors, new HashSet<>());
                }
            }
        }
    }

    private static Object resolvePointer(Object rootSchema, String pointer) {
        String[] segments = pointer.substring(2).split("/");
        Object current = rootSchema;
        for (String rawSegment : segments) {
            if (rawSegment.isEmpty()) {
                continue;
            }
            if (!(current instanceof Map<?, ?> map)) {
                return null;
            }
            String segment = rawSegment.replace("~1", "/").replace("~0", "~");
            current = map.get(segment);
            if (current == null) {
                return null;
            }
        }
        return current;
    }
}
