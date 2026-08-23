package io.github.cuihairu.croupier.sdk.invoker;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Minimal JSON codec shared by the L3 HTTP invoker, schema validation and
 * OpenAPI import. Extracted from ServerHttpInvoker so other SDK components
 * can reuse it without widening SDK dependencies.
 */
public final class Json {
    public static Object parse(String source) { Parser parser = new Parser(source); Object value = parser.value(); parser.space(); if (!parser.done()) throw new IllegalArgumentException("unexpected trailing JSON content"); return value; }
    public static String stringify(Object value) { StringBuilder output = new StringBuilder(); write(value, output); return output.toString(); }
    private static void write(Object value, StringBuilder output) {
        if (value == null) { output.append("null"); return; } if (value instanceof String string) { writeString(string, output); return; } if (value instanceof Number || value instanceof Boolean) { output.append(value); return; }
        if (value instanceof Map<?, ?> map) { output.append('{'); boolean first = true; for (Map.Entry<?, ?> entry : map.entrySet()) { if (!(entry.getKey() instanceof String key)) throw new IllegalArgumentException("JSON object keys must be strings"); if (!first) output.append(','); writeString(key, output); output.append(':'); write(entry.getValue(), output); first = false; } output.append('}'); return; }
        if (value instanceof List<?> list) { output.append('['); boolean first = true; for (Object item : list) { if (!first) output.append(','); write(item, output); first = false; } output.append(']'); return; }
        throw new IllegalArgumentException("unsupported JSON value: " + value.getClass().getName());
    }
    private static void writeString(String value, StringBuilder output) { output.append('"'); for (int i = 0; i < value.length(); i++) { char c = value.charAt(i); switch (c) { case '"' -> output.append("\\\""); case '\\' -> output.append("\\\\"); case '\b' -> output.append("\\b"); case '\f' -> output.append("\\f"); case '\n' -> output.append("\\n"); case '\r' -> output.append("\\r"); case '\t' -> output.append("\\t"); default -> { if (c < 0x20) output.append(String.format(Locale.ROOT, "\\u%04x", (int) c)); else output.append(c); } } } output.append('"'); }
    private static final class Parser {
        private final String source; private int index; Parser(String source) { this.source = source == null ? "" : source; } boolean done() { return index >= source.length(); } void space() { while (!done() && Character.isWhitespace(source.charAt(index))) index++; }
        Object value() { space(); if (done()) throw new IllegalArgumentException("empty JSON"); return switch (source.charAt(index)) { case '{' -> object(); case '[' -> array(); case '"' -> string(); case 't' -> literal("true", Boolean.TRUE); case 'f' -> literal("false", Boolean.FALSE); case 'n' -> literal("null", null); default -> number(); }; }
        Object literal(String text, Object value) { if (!source.startsWith(text, index)) throw new IllegalArgumentException("invalid JSON literal"); index += text.length(); return value; }
        Map<String, Object> object() { Map<String, Object> map = new LinkedHashMap<>(); index++; space(); if (take('}')) return map; while (true) { space(); if (done() || source.charAt(index) != '"') throw new IllegalArgumentException("JSON object key expected"); String key = string(); need(':'); map.put(key, value()); space(); if (take('}')) return map; need(','); } }
        List<Object> array() { List<Object> list = new ArrayList<>(); index++; space(); if (take(']')) return list; while (true) { list.add(value()); space(); if (take(']')) return list; need(','); } }
        String string() { need('"'); StringBuilder output = new StringBuilder(); while (!done()) { char c = source.charAt(index++); if (c == '"') return output.toString(); if (c != '\\') { output.append(c); continue; } if (done()) throw new IllegalArgumentException("unterminated JSON escape"); switch (source.charAt(index++)) { case '"', '\\', '/' -> output.append(source.charAt(index - 1)); case 'b' -> output.append('\b'); case 'f' -> output.append('\f'); case 'n' -> output.append('\n'); case 'r' -> output.append('\r'); case 't' -> output.append('\t'); case 'u' -> { if (index + 4 > source.length()) throw new IllegalArgumentException("invalid unicode escape"); try { output.append((char) Integer.parseInt(source.substring(index, index + 4), 16)); } catch (NumberFormatException exception) { throw new IllegalArgumentException("invalid unicode escape", exception); } index += 4; } default -> throw new IllegalArgumentException("invalid JSON escape"); } } throw new IllegalArgumentException("unterminated JSON string"); }
        Number number() { int start = index; take('-'); if (done() || !Character.isDigit(source.charAt(index))) throw new IllegalArgumentException("invalid JSON number"); if (source.charAt(index) == '0') index++; else while (!done() && Character.isDigit(source.charAt(index))) index++; if (take('.')) { if (done() || !Character.isDigit(source.charAt(index))) throw new IllegalArgumentException("invalid JSON number"); while (!done() && Character.isDigit(source.charAt(index))) index++; } if (!done() && (source.charAt(index) == 'e' || source.charAt(index) == 'E')) { index++; if (!done() && (source.charAt(index) == '+' || source.charAt(index) == '-')) index++; if (done() || !Character.isDigit(source.charAt(index))) throw new IllegalArgumentException("invalid JSON number"); while (!done() && Character.isDigit(source.charAt(index))) index++; } String text = source.substring(start, index); try { return text.contains(".") || text.contains("e") || text.contains("E") ? Double.valueOf(text) : Long.valueOf(text); } catch (NumberFormatException exception) { throw new IllegalArgumentException("invalid JSON number", exception); } }
        boolean take(char expected) { if (!done() && source.charAt(index) == expected) { index++; return true; } return false; } void need(char expected) { space(); if (!take(expected)) throw new IllegalArgumentException("expected '" + expected + "'"); }
    }
}

