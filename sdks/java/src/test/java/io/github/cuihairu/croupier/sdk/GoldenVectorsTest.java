package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import org.junit.jupiter.api.Test;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * CR-FIX-05 跨 SDK 行为级契约 golden 向量测试（Java SDK 面）。
 * 读取 proto/contracts/golden_vectors.json（语言中立字节向量），断言本 SDK 的
 * Protocol.newMessage 产出与向量 frame 逐字节一致，且 parseMessage 能还原。
 */
class GoldenVectorsTest {

    private static final Path REPO = Path.of("..", "..");
    private static final Path GOLDEN = REPO.resolve(Path.of("proto", "contracts", "golden_vectors.json"));

    // 最小 JSON 解析（golden 文件结构固定，避免引入额外测试依赖）。
    @SuppressWarnings("unchecked")
    private static Map<String, Object> parseJson(String json) {
        JsonParser p = new JsonParser(json);
        return (Map<String, Object>) p.parseValue();
    }

    private static String str(Object o) {
        return (String) o;
    }

    private static int msgIdOf(Map<String, Object> vec) {
        return Integer.parseUnsignedInt(str(vec.get("msgId")), 16);
    }

    private static int reqIdOf(Map<String, Object> vec) {
        return Integer.parseUnsignedInt(str(vec.get("reqId")), 16);
    }

    private static byte[] bodyOf(Map<String, Object> vec) {
        String s = str(vec.get("body"));
        return s.isEmpty() ? new byte[0] : hexToBytes(s);
    }

    private static byte[] frameOf(Map<String, Object> vec) {
        return hexToBytes(str(vec.get("frame")));
    }

    private static byte[] hexToBytes(String s) {
        int len = s.length();
        byte[] out = new byte[len / 2];
        for (int i = 0; i < out.length; i++) {
            out[i] = (byte) Integer.parseInt(s.substring(i * 2, i * 2 + 2), 16);
        }
        return out;
    }

    @Test
    void wireFramesEncodeAndDecode() throws Exception {
        String json = Files.readString(GOLDEN);
        @SuppressWarnings("unchecked")
        Map<String, Object> vectors = (Map<String, Object>) parseJson(json).get("vectors");
        for (String name : vectors.keySet()) {
            @SuppressWarnings("unchecked")
            Map<String, Object> vec = (Map<String, Object>) vectors.get(name);
            int msgId = msgIdOf(vec);
            int reqId = reqIdOf(vec);
            byte[] body = bodyOf(vec);
            byte[] want = frameOf(vec);

            byte[] got = Protocol.newMessage(msgId, reqId, body);
            assertArrayEquals(want, got, name + ": frame mismatch");

            Protocol.ParsedMessage parsed = Protocol.parseMessage(want);
            assertEquals(Protocol.VERSION_1, parsed.version, name + ": version");
            assertEquals(msgId, parsed.msgId, name + ": msgId");
            assertEquals(reqId, parsed.reqId, name + ": reqId");
            assertArrayEquals(body, parsed.body, name + ": body");
        }
    }

    // 极简递归下降 JSON 解析器（仅支持对象/字符串/数字，golden 文件够用）。
    private static final class JsonParser {
        private final String s;
        private int i;

        JsonParser(String s) { this.s = s; this.i = 0; }

        Object parseValue() {
            skipWs();
            char c = s.charAt(i);
            if (c == '{') return parseObject();
            if (c == '"') return parseString();
            if (c == '[') return parseArray();
            if (c == '-' || (c >= '0' && c <= '9')) return parseNumber();
            if (s.startsWith("true", i)) { i += 4; return Boolean.TRUE; }
            if (s.startsWith("false", i)) { i += 5; return Boolean.FALSE; }
            if (s.startsWith("null", i)) { i += 4; return null; }
            throw new IllegalStateException("unexpected char " + c);
        }

        private Number parseNumber() {
            int start = i;
            while (i < s.length() && "-+.eE0123456789".indexOf(s.charAt(i)) >= 0) i++;
            String raw = s.substring(start, i);
            if (raw.contains(".") || raw.contains("e") || raw.contains("E")) {
                return Double.parseDouble(raw);
            }
            return Long.parseLong(raw);
        }

        private Map<String, Object> parseObject() {
            Map<String, Object> m = new LinkedHashMap<>();
            i++; // '{'
            skipWs();
            if (s.charAt(i) == '}') { i++; return m; }
            while (true) {
                skipWs();
                String k = parseString();
                skipWs();
                i++; // ':'
                m.put(k, parseValue());
                skipWs();
                char c = s.charAt(i++);
                if (c == '}') break;
                if (c != ',') throw new IllegalStateException("expected , or }");
            }
            return m;
        }

        private Object parseArray() {
            java.util.List<Object> a = new java.util.ArrayList<>();
            i++; // '['
            skipWs();
            if (s.charAt(i) == ']') { i++; return a; }
            while (true) {
                a.add(parseValue());
                skipWs();
                char c = s.charAt(i++);
                if (c == ']') break;
                if (c != ',') throw new IllegalStateException("expected , or ]");
            }
            return a;
        }

        private String parseString() {
            i++; // '"'
            StringBuilder sb = new StringBuilder();
            while (s.charAt(i) != '"') {
                char c = s.charAt(i++);
                if (c == '\\') c = s.charAt(i++);
                sb.append(c);
            }
            i++; // '"'
            return sb.toString();
        }

        private void skipWs() {
            while (i < s.length() && Character.isWhitespace(s.charAt(i))) i++;
        }
    }
}
