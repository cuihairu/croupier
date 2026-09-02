/**
 * F14 验收测试：setFieldHint/setFieldWidget 便捷层
 */
import { setFieldHint, setFieldWidget, type FunctionDescriptor } from "./index";

function base(): FunctionDescriptor {
  return { id: "player.ban", version: "1.0.0" };
}

describe("F14: setFieldHint", () => {
  test("空 schema 自动创建 object 骨架并合并 hint", () => {
    const descriptor = setFieldWidget(base(), "id", "Select");
    const schema = descriptor.inputSchema as Record<string, unknown>;
    expect(schema.type).toBe("object");
    const props = schema.properties as Record<string, Record<string, unknown>>;
    expect(props.id["x-widget"]).toBe("Select");
  });

  test("保留既有字段属性，重复设置覆盖", () => {
    const descriptor = setFieldWidget(
      {
        ...base(),
        inputSchema: {
          type: "object",
          properties: { id: { type: "string", title: "玩家 ID", "x-widget": "Input" } },
        },
      },
      "id",
      "TreeSelect",
    );
    const props = (descriptor.inputSchema as Record<string, unknown>).properties as Record<
      string,
      Record<string, unknown>
    >;
    expect(props.id.title).toBe("玩家 ID");
    expect(props.id["x-widget"]).toBe("TreeSelect");
  });

  test("x-options-source 完整对象", () => {
    const descriptor = setFieldHint(base(), "id", "x-options-source", {
      functionId: "player.list",
      labelPath: "/items/*/name",
      valuePath: "/items/*/id",
    });
    const props = (descriptor.inputSchema as Record<string, unknown>).properties as Record<
      string,
      Record<string, unknown>
    >;
    expect(props.id["x-options-source"]).toEqual({
      functionId: "player.list",
      labelPath: "/items/*/name",
      valuePath: "/items/*/id",
    });
  });

  test("非 x- hint 拒绝", () => {
    expect(() => setFieldHint(base(), "a", "widget", "Input")).toThrow(/x- extension key/);
  });

  test("空 field 拒绝", () => {
    expect(() => setFieldHint(base(), "  ", "x-widget", "Input")).toThrow(/field key is required/);
  });
});
