/**
 * AUD-069: pending-approval responses must survive the L3 decoder instead of
 * being reported as no_result / no_task_id.
 */

import { Invoker, InvokerError } from "./invoker";

type FetchImpl = typeof fetch;
let originalFetch: FetchImpl | undefined;

function mockFetch(responder: () => Promise<Response>): void {
  originalFetch = globalThis.fetch;
  globalThis.fetch = (() => responder()) as unknown as FetchImpl;
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  if (originalFetch) globalThis.fetch = originalFetch;
  originalFetch = undefined;
});

describe("Invoker pending approval (AUD-069)", () => {
  it("invoke exposes the approval credential and no business payload", async () => {
    mockFetch(async () =>
      jsonResponse({
        taskId: "",
        approvalId: "apr-1",
        approvalRequired: true,
        approvalWorkflow: "two_person",
        traceId: "tr-1",
      }),
    );
    const result = await new Invoker({ baseUrl: "http://s/api/v1" }).invoke("player.ban", {});
    expect(result.approvalId).toBe("apr-1");
    expect(result.approvalWorkflow).toBe("two_person");
    expect(result.approvalStatus).toBeUndefined();
    expect(result.payload).toBeUndefined();
    expect(result.traceId).toBe("tr-1");
  });

  it("startTask surfaces approval_required instead of no_task_id", async () => {
    mockFetch(async () =>
      jsonResponse({
        taskId: "",
        status: "approval_required",
        approvalId: "apr-2",
        approvalWorkflow: "single_admin",
      }),
    );
    const error = await new Invoker({ baseUrl: "http://s/api/v1" })
      .startTask("player.ban", {})
      .catch((e: unknown) => e);
    expect(error).toBeInstanceOf(InvokerError);
    expect((error as InvokerError).code).toBe("approval_required");
    expect((error as InvokerError).status).toBe(200);
    expect((error as InvokerError).details).toMatchObject({ approvalId: "apr-2" });
    expect((error as InvokerError).resultUnknown).toBeUndefined();
  });

  it("a status-only response without an approval id stays no_task_id", async () => {
    mockFetch(async () => jsonResponse({ taskId: "", status: "approval_required" }));
    const error = await new Invoker({ baseUrl: "http://s/api/v1" })
      .startTask("player.ban", {})
      .catch((e: unknown) => e);
    expect((error as InvokerError).code).toBe("no_task_id");
  });
});
