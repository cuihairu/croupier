package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for the Json parser edge branches. */
class JsonParserBranchTest {

    @Test
    void rejectsBrokenUnicodeEscapes() {
        IllegalArgumentException truncated = assertThrows(IllegalArgumentException.class,
            () -> Json.parse("\"\\u12\""));
        assertTrue(truncated.getMessage().contains("unicode"));
        IllegalArgumentException notHex = assertThrows(IllegalArgumentException.class,
            () -> Json.parse("\"\\uzzzz\""));
        assertTrue(notHex.getMessage().contains("unicode"));
    }

    @Test
    void rejectsInvalidEscapeCharacter() {
        IllegalArgumentException bad = assertThrows(IllegalArgumentException.class,
            () -> Json.parse("\"\\q\""));
        assertTrue(bad.getMessage().contains("escape"));
    }

    @Test
    void rejectsTrailingBackslashInString() {
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"abc\\"));
    }

    @Test
    void parsesAllEscapeSequences() {
        Map<?, ?> parsed = (Map<?, ?>) Json.parse(
            "{\"q\":\"\\\"\",\"s\":\"\\\\\",\"slash\":\"\\/\",\"b\":\"\\b\",\"f\":\"\\f\","
                + "\"n\":\"\\n\",\"r\":\"\\r\",\"t\":\"\\t\",\"u\":\"\\u00e9\"}");
        assertEquals("\"", parsed.get("q"));
        assertEquals("\\", parsed.get("s"));
        assertEquals("/", parsed.get("slash"));
        assertEquals("\b", parsed.get("b"));
        assertEquals("\f", parsed.get("f"));
        assertEquals("\n", parsed.get("n"));
        assertEquals("\r", parsed.get("r"));
        assertEquals("\t", parsed.get("t"));
        assertEquals("é", parsed.get("u"));
    }

    @Test
    void parsesNumberGrammarVariants() {
        // NOTE: javac numeric promotion makes the parser's ternary box every
        // number as Double, integers included.
        assertEquals(Double.valueOf(0), Json.parse("0"));
        assertEquals(Double.valueOf(42), Json.parse("42"));
        assertEquals(Double.valueOf(-7), Json.parse("-7"));
        assertEquals(Double.valueOf(1.5), Json.parse("1.5"));
        assertEquals(Double.valueOf(-0.25), Json.parse("-0.25"));
        assertEquals(Double.valueOf(1e3), Json.parse("1e3"));
        assertEquals(Double.valueOf(2.5e-2), Json.parse("2.5E-2"));
        assertEquals(Double.valueOf(3e+2), Json.parse("3e+2"));
        // leading-zero and fraction/exponent combinations
        assertEquals(Double.valueOf(0.5), Json.parse("0.5"));
        assertEquals(Double.valueOf(-0.25), Json.parse("-0.25"));
        assertEquals(Double.valueOf(0e1), Json.parse("0e1"));
        assertEquals(Double.valueOf(10.5), Json.parse("10.5"));
        assertEquals(Double.valueOf(9e2), Json.parse("9e2"));
        assertEquals(Double.valueOf(0.05), Json.parse("0.05"));

        assertThrows(IllegalArgumentException.class, () -> Json.parse("-x"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("1."));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("1e"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("1e+"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("01"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("0."));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("0e"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("0e x"));
    }

    @Test
    void rejectsBrokenStructures() {
        assertThrows(IllegalArgumentException.class, () -> Json.parse(""));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("  "));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{5:1}"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\" 1}"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":1 \"b\":2}"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("[1 2]"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":nope}"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("tru"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("fals"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("nul"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"unterminated"));
    }

    @Test
    void rejectsTruncatedContainersAtEof() {
        // object variants hitting the EOF guards in object()/array()/need()
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\""));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":1"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":1,"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":1 "));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("["));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("[1"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("[1,"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("[1, "));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{\"a\":\"x\""));
    }

    @Test
    void parsesLiteralsArraysAndNestedEmpty() {
        assertEquals(Boolean.TRUE, Json.parse("true"));
        assertEquals(Boolean.FALSE, Json.parse("false"));
        assertEquals(null, Json.parse("null"));
        assertEquals(List.of(), Json.parse("[]"));
        assertEquals(Map.of(), Json.parse("{}"));
        assertEquals(List.of(1.0, List.of("two"), Map.of("three", 3.0)), Json.parse("[1,[\"two\"],{\"three\":3}]"));
        assertEquals("", Json.parse("\"\""));
    }

    @Test
    void stringifyMapsAndNulls() {
        assertEquals("null", Json.stringify(null));
        assertEquals("{}", Json.stringify(Map.of()));
        assertTrue(Json.stringify(Map.of("a", 1)).contains("\"a\""));
    }
}
