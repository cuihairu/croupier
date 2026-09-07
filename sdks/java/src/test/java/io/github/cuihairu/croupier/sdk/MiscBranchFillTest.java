package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.invoker.JsonSchemaValidator;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Small branch-fillers: Protocol.isRequest, JsonSchemaValidator additional edges. */
class MiscBranchFillTest {

    @Test
    void isRequestExcludesTaskEventMessageId() {
        // MSG_TASK_EVENT is even, so isRequest is false for it and neighbours
        assertFalse(Protocol.isRequest(Protocol.MSG_TASK_EVENT));
        assertFalse(Protocol.isRequest(Protocol.MSG_TASK_EVENT + 2));
        assertTrue(Protocol.isRequest(1));
        assertFalse(Protocol.isRequest(2));
    }

    @Test
    void minimumAcceptsBoundsAsStringsWhenValueIsNotNumeric() {
        // bounds present as non-numbers: instanceof branches already covered;
        // here: value not a Number at all makes checkNumeric return early
        Map<String, Object> schema = new HashMap<>();
        schema.put("minimum", 1);
        schema.put("maximum", 1);
        assertTrue(JsonSchemaValidator.isValid(schema, "text"));
    }

    @Test
    void additionalPropertiesTrueAllowsEverything() {
        assertTrue(JsonSchemaValidator.validate("{\"x\": 1}",
            "{\"properties\": {\"known\": {\"type\": \"number\"}}, \"additionalProperties\": true}").isEmpty());
    }
}
