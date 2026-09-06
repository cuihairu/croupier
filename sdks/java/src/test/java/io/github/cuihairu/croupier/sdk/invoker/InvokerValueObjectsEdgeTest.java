package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TaskEventInfo / Json / 各配置类 equals/hashCode 与 JSON 解析器边界补测。
 */
@DisplayName("Value objects and JSON parser edge coverage")
class InvokerValueObjectsEdgeTest {

    @Test
    @DisplayName("TaskEventInfo equals branches and hashCode")
    void taskEventInfoEqualsAndHashCode() {
        TaskEventInfo event = TaskEventInfo.builder()
            .type("progress").taskId("t-1").payload("{}")
            .message("half").progress(50).error(null).done(false)
            .build();
        TaskEventInfo same = TaskEventInfo.builder()
            .type("progress").taskId("t-1").payload("{}")
            .message("half").progress(50).error(null).done(false)
            .build();

        assertEquals(event, event);
        assertNotEquals(null, event);
        assertNotEquals("not-an-event", event);
        assertEquals(event, same);
        assertEquals(event.hashCode(), same.hashCode());
        assertEquals(event.hashCode(), event.hashCode());
        assertNotNull(event.toString());
        assertTrue(event.toString().contains("t-1"));

        assertNotEquals(event, TaskEventInfo.builder().type("completed").taskId("t-1").build());
        assertNotEquals(event, TaskEventInfo.builder().taskId("t-2").type("progress").build());
        assertNotEquals(event, TaskEventInfo.builder().type("progress").taskId("t-1").payload("x").build());
        assertNotEquals(event, TaskEventInfo.builder().type("progress").taskId("t-1").message("m").build());
        assertNotEquals(event, TaskEventInfo.builder().type("progress").taskId("t-1").progress(1).build());
        assertNotEquals(event, TaskEventInfo.builder().type("progress").taskId("t-1").error("e").build());
        assertNotEquals(event, TaskEventInfo.builder().type("progress").taskId("t-1").done(true).build());
    }

    @Test
    @DisplayName("ReconnectConfig / InvokeOptions / RetryConfig equals guard branches")
    void configEqualsGuards() {
        ReconnectConfig reconnect = ReconnectConfig.builder().build();
        assertEquals(reconnect, reconnect);
        assertNotEquals(null, reconnect);
        assertNotEquals("other", reconnect);

        InvokeOptions options = InvokeOptions.builder().build();
        assertEquals(options, options);
        assertNotEquals(null, options);
        assertNotEquals(42, options);

        RetryConfig retry = RetryConfig.builder().build();
        assertEquals(retry, retry);
        assertNotEquals(null, retry);
        assertNotEquals("retry", retry);
        // builder null 归一化
        assertTrue(RetryConfig.builder().retryableStatusCodes(null).build()
            .getRetryableStatusCodes().isEmpty());
    }

    @Test
    @DisplayName("InvokerConfig builder null normalization")
    void invokerConfigBuilderNulls() {
        InvokerConfig config = InvokerConfig.builder()
            .authToken(null)
            .gameId(null)
            .env(null)
            .build();
        assertEquals("", config.getAuthToken());
        assertEquals("", config.getGameId());
        assertEquals("", config.getEnv());
    }

    @Test
    @DisplayName("Json parse: escapes, exponents and malformed inputs")
    void jsonParserEdges() throws Exception {
        // 默认构造器（隐式）覆盖
        assertNotNull(newJsonInstance());

        assertEquals("a\bb\fc\nd\re\tf", Json.parse("\"a\\bb\\fc\\nd\\re\\tf\""));
        assertEquals("A/", Json.parse("\"\\u0041\\/\""));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"bad \\q escape\""));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"trailing \\"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"\\uZZZZ\""));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("\"unterminated"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse(null));

        assertEquals(100000.0, Json.parse("1e5"));
        assertEquals(100000.0, Json.parse("1E+5"));
        assertEquals(0.001, Json.parse("1e-3"));
        assertEquals(0.5, Json.parse("0.5"));
        assertEquals(-3.0, Json.parse("-3"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("1e"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("-"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("nul"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("{} extra"));

        // stringify 边界：非字符串 key / 不支持的值
        assertThrows(IllegalArgumentException.class,
            () -> Json.stringify(Map.of(1, "x")));
        assertThrows(IllegalArgumentException.class,
            () -> Json.stringify(new Object()));
        assertEquals("null", Json.stringify(null));
    }

    private static Object newJsonInstance() throws Exception {
        java.lang.reflect.Constructor<Json> constructor = Json.class.getDeclaredConstructor();
        constructor.setAccessible(true);
        return constructor.newInstance();
    }

    @Test
    @DisplayName("JsonSchemaValidator: type names, const, maximum, maxItems, $ref pointers")
    void jsonSchemaValidatorEdges() {
        // boolean 类型匹配分支
        assertTrue(JsonSchemaValidator.isValid(
            Json.parse("{\"type\":\"boolean\"}"), Boolean.TRUE));

        // jsonTypeName 各分支：expected string 但拿到其他类型
        List<String> objectMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), Json.parse("{\"a\":1}"));
        assertTrue(objectMismatch.get(0).contains("object"));
        List<String> arrayMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), Json.parse("[1]"));
        assertTrue(arrayMismatch.get(0).contains("array"));
        List<String> booleanMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), Json.parse("true"));
        assertTrue(booleanMismatch.get(0).contains("boolean"));
        List<String> numberMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), Json.parse("3"));
        assertTrue(numberMismatch.get(0).contains("number"));
        List<String> nullMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), Json.parse("null"));
        assertTrue(nullMismatch.get(0).contains("null"));
        // 非标准 Java 类型（validate(Object, Object) 直通）
        List<String> alienMismatch = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"string\"}"), new Object());
        assertTrue(alienMismatch.get(0).contains("Object"));

        // const 不匹配
        List<String> constErrors = JsonSchemaValidator.validate(
            Json.parse("{\"const\":\"a\"}"), Json.parse("\"b\""));
        assertEquals(1, constErrors.size());
        assertTrue(constErrors.get(0).contains("const"));

        // maximum 超限
        List<String> maxErrors = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"number\",\"maximum\":10}"), Json.parse("11"));
        assertEquals(1, maxErrors.size());
        assertTrue(maxErrors.get(0).contains("greater than maximum"));

        // maxItems 超限
        List<String> maxItemsErrors = JsonSchemaValidator.validate(
            Json.parse("{\"type\":\"array\",\"maxItems\":1}"), Json.parse("[1,2]"));
        assertEquals(1, maxItemsErrors.size());
        assertTrue(maxItemsErrors.get(0).contains("maxItems"));

        // $ref 指针：根指针（空段 continue）与穿越非 Map 节点
        List<String> rootRefErrors = JsonSchemaValidator.validate(
            Json.parse("{\"$ref\":\"#/\",\"type\":\"string\"}"), Json.parse("\"ok\""));
        assertTrue(rootRefErrors.isEmpty());
        List<String> badRefErrors = JsonSchemaValidator.validate(
            Json.parse("{\"definitions\":\"not-a-map\",\"$ref\":\"#/definitions/x\"}"),
            Json.parse("\"v\""));
        assertEquals(1, badRefErrors.size());
        assertTrue(badRefErrors.get(0).contains("unresolved $ref"));
    }
}
