package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Direct tests for the package-local JSON codec used by ServerHttpInvoker.
 */
@DisplayName("ServerHttpInvoker JSON codec")
class ServerHttpInvokerJsonTest {

    @Test
    @DisplayName("parses literals, numbers and strings")
    void parsePrimitives() {
        assertEquals(Boolean.TRUE, ServerHttpInvoker.Json.parse("true"));
        assertEquals(Boolean.FALSE, ServerHttpInvoker.Json.parse("false"));
        assertNull(ServerHttpInvoker.Json.parse("null"));
        // Recorded bug: Parser.number()'s ternary promotes Long to Double, so
        // every JSON number (including integers) is parsed as Double.
        assertEquals(0.0, (Double) ServerHttpInvoker.Json.parse("0"), 1e-9);
        assertEquals(-7.0, (Double) ServerHttpInvoker.Json.parse("-7"), 1e-9);
        assertEquals(9223372036854775807.0, (Double) ServerHttpInvoker.Json.parse("9223372036854775807"), 1e6);
        assertEquals(3.14, (Double) ServerHttpInvoker.Json.parse("3.14"), 1e-9);
        assertEquals(1000.0, (Double) ServerHttpInvoker.Json.parse("1e3"), 1e-9);
        assertEquals(0.0015, (Double) ServerHttpInvoker.Json.parse("1.5E-3"), 1e-9);
        assertEquals(100.0, (Double) ServerHttpInvoker.Json.parse("1.0e+2"), 1e-9);
        assertEquals("hi", ServerHttpInvoker.Json.parse("\"hi\""));
        assertEquals(" spaced ", ServerHttpInvoker.Json.parse("  \" spaced \"  "));
    }

    @Test
    @DisplayName("parses escapes including unicode code points")
    void parseEscapes() {
        String source = "{\"a\":\"\\n\\t\\r\\b\\f\\\"\\\\\\/\\u00e9\\u4e2d\"}";
        @SuppressWarnings("unchecked")
        Map<String, Object> map = (Map<String, Object>) ServerHttpInvoker.Json.parse(source);
        assertEquals("\n\t\r\b\f\"\\/\u00e9中", map.get("a"));
    }

    @Test
    @DisplayName("parses nested objects and arrays")
    @SuppressWarnings("unchecked")
    void parseNested() {
        Object value = ServerHttpInvoker.Json.parse("{\"a\":[1,{\"b\":[true,null]},\"x\"],\"c\":{}}");
        Map<String, Object> map = (Map<String, Object>) value;
        List<Object> a = (List<Object>) map.get("a");
        assertEquals(1.0, (Double) a.get(0), 1e-9);
        Map<String, Object> nested = (Map<String, Object>) a.get(1);
        assertEquals(java.util.Arrays.asList(Boolean.TRUE, null), nested.get("b"));
        assertEquals("x", a.get(2));
        assertEquals(Map.of(), map.get("c"));
        assertEquals(Map.of(), ServerHttpInvoker.Json.parse("{}"));
        assertEquals(List.of(), ServerHttpInvoker.Json.parse("[]"));
        assertEquals(List.of(1.0, 2.0, 3.0), ServerHttpInvoker.Json.parse("[1,2,3]"));
    }

    @Test
    @DisplayName("stringifies primitives, maps, lists and control characters")
    void stringifyValues() {
        assertEquals("null", ServerHttpInvoker.Json.stringify(null));
        assertEquals("\"s\"", ServerHttpInvoker.Json.stringify("s"));
        assertEquals("1", ServerHttpInvoker.Json.stringify(1L));
        assertEquals("1.5", ServerHttpInvoker.Json.stringify(1.5));
        assertEquals("true", ServerHttpInvoker.Json.stringify(Boolean.TRUE));

        Map<String, Object> ordered = new LinkedHashMap<>();
        ordered.put("a", List.of(1L, 2L));
        ordered.put("b", "v");
        assertEquals("{\"a\":[1,2],\"b\":\"v\"}", ServerHttpInvoker.Json.stringify(ordered));
        // Recorded bug: parsed integers are Doubles, so re-serializing turns 42 into 42.0.
        assertEquals("42.0", ServerHttpInvoker.Json.stringify(ServerHttpInvoker.Json.parse("42")));

        assertEquals("\"\\u0001\"", ServerHttpInvoker.Json.stringify("\u0001"));
        assertEquals("\"\\n\\t\\r\\b\\f\\\"\\\\\"",
            ServerHttpInvoker.Json.stringify("\n\t\r\b\f\"\\"));
    }

    @Test
    @DisplayName("round-trips parsed values")
    @SuppressWarnings("unchecked")
    void roundTrip() {
        String source = "{\"name\":\"croupier\",\"count\":3,\"ratio\":0.5,\"flags\":[true,false],\"nested\":{\"k\":\"\\u00e9\"}}";
        Object parsed = ServerHttpInvoker.Json.parse(source);
        assertEquals(parsed, ServerHttpInvoker.Json.parse(ServerHttpInvoker.Json.stringify(parsed)));
        Map<String, Object> map = (Map<String, Object>) parsed;
        assertEquals("croupier", map.get("name"));
    }

    @Test
    @DisplayName("rejects malformed JSON inputs")
    void parseErrors() {
        assertParseError("");
        assertParseError("   ");
        assertParseError("{");
        assertParseError("{\"a\"");
        assertParseError("{\"a\":}");
        assertParseError("{\"a\":1,}");
        assertParseError("[");
        assertParseError("[1,]");
        assertParseError("tru");
        assertParseError("tRue");
        assertParseError("nul");
        assertParseError("01");
        assertParseError("1.");
        assertParseError("1e");
        assertParseError("-");
        assertParseError("\"unterminated");
        assertParseError("\"bad\\qescape\"");
        assertParseError("\"short\\u00\"");
        assertParseError("{}trailing");
        assertParseError("nullnull");
    }

    @Test
    @DisplayName("rejects non-string keys and unsupported value types")
    void stringifyErrors() {
        Map<Object, Object> badKeys = new HashMap<>();
        badKeys.put(1, "v");
        IllegalArgumentException keys = assertThrows(IllegalArgumentException.class,
            () -> ServerHttpInvoker.Json.stringify(badKeys));
        assertTrue(keys.getMessage().contains("keys must be strings"));

        IllegalArgumentException unsupported = assertThrows(IllegalArgumentException.class,
            () -> ServerHttpInvoker.Json.stringify(new Object()));
        assertTrue(unsupported.getMessage().contains("unsupported JSON value"));
    }

    private static void assertParseError(String source) {
        IllegalArgumentException error = assertThrows(IllegalArgumentException.class,
            () -> ServerHttpInvoker.Json.parse(source), "expected parse failure for: " + source);
        assertNotNull(error.getMessage());
    }
}
