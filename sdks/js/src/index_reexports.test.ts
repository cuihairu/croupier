/**
 * index.ts 顶层 re-export 绑定的函数覆盖：
 * ts-jest 的 CommonJS 输出将每个 named re-export 编译为 getter，
 * 只有测试侧真实 import 并访问该绑定，对应 instrumentation 计数才会增加。
 * 本文件从公开入口（src/index）逐一消费 re-export 表面，
 * 并断言 createClient 的基本契约，防止 re-export 表面静默漂移。
 */
import {
  createClient,
  TCPTransport,
  traceParentFromContext,
  traceIdFromContext,
  Invoker,
  createInvoker,
  InvokerError,
  InvokerEventSource,
  registerFromOpenAPI,
} from "./index";
import * as reexports from "./index";
import defaultExport from "./index";

describe("index re-export surface", () => {
  test("transport / trace / invoker / openapi re-exports 均可从入口访问", () => {
    expect(typeof TCPTransport).toBe("function");
    expect(typeof traceParentFromContext).toBe("function");
    expect(typeof traceIdFromContext).toBe("function");
    expect(typeof Invoker).toBe("function");
    expect(typeof createInvoker).toBe("function");
    expect(typeof InvokerError).toBe("function");
    expect(typeof InvokerEventSource).toBe("function");
    expect(typeof registerFromOpenAPI).toBe("function");
  });

  test("createClient 构造默认客户端", () => {
    const client = createClient();
    expect(client).toBeDefined();
    expect(typeof client.registerFunction).toBe("function");
    expect(typeof client.connect).toBe("function");
  });

  test("star re-export（./protocol）符号经入口访问", () => {
    // export * from "./protocol" 的 getter 需要真实访问绑定才计数。
    expect(typeof reexports.newMessage).toBe("function");
    expect(typeof reexports.parseMessage).toBe("function");
    expect(typeof reexports.getResponseMsgId).toBe("function");
    expect(typeof reexports.MSG_INVOKE_REQUEST).toBe("number");
  });

  test("default 导出为 BasicClient 构造器", () => {
    expect(typeof defaultExport).toBe("function");
    const client = new defaultExport({});
    expect(typeof client.registerFunction).toBe("function");
  });

  test("InvokerError 形态与 re-export 一致", () => {
    const err = new InvokerError("boom", 500);
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toBe("boom");
    expect(err.status).toBe(500);
  });

  test("createInvoker 构造 Invoker 实例", () => {
    const invoker = createInvoker({ baseUrl: "http://127.0.0.1:18780" });
    expect(invoker).toBeInstanceOf(Invoker);
    expect(typeof invoker.invoke).toBe("function");
  });
});
