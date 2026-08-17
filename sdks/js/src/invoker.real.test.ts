import { Invoker, InvokerError } from "./invoker";

const required = ["CROUPIER_SERVER_URL", "CROUPIER_SERVER_TOKEN"] as const;
const enabled = required.every((name) => Boolean(process.env[name]));
const describeReal = enabled ? describe : describe.skip;

function realInvoker(): Invoker {
  return new Invoker({
    baseUrl: process.env.CROUPIER_SERVER_URL!,
    token: process.env.CROUPIER_SERVER_TOKEN!,
    gameId: process.env.CROUPIER_GAME_ID || "e2e-game",
    env: process.env.CROUPIER_ENV || "e2e",
    timeout: 10_000,
  });
}

async function waitForStatus(invoker: Invoker, taskId: string, expected: string) {
  const deadline = Date.now() + 20_000;
  let status = await invoker.getTaskStatus(taskId);
  while (status.status !== expected && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 50));
    status = await invoker.getTaskStatus(taskId);
  }
  expect(status.status).toBe(expected);
  return status;
}

describeReal("Invoker real Server lifecycle", () => {
  it("uses Server auth, sync invoke, task events, task status and cancellation", async () => {
    const unauthenticated = new Invoker({
      baseUrl: process.env.CROUPIER_SERVER_URL!,
      gameId: process.env.CROUPIER_GAME_ID || "e2e-game",
      env: process.env.CROUPIER_ENV || "e2e",
    });
    await expect(
      unauthenticated.invoke("mail.send", { player_id: "p-001", title: "denied" }),
    ).rejects.toMatchObject<Partial<InvokerError>>({ status: expect.any(Number) });

    const invoker = realInvoker();
    const result = await invoker.invoke("mail.send", {
      player_id: "p-001",
      title: "JavaScript",
      content: "body",
    });
    expect(result.payload).toMatchObject({ mail_id: "mail-0001", title: "JavaScript" });

    const completedId = await invoker.startTask("mail.send", {
      player_id: "p-001",
      title: "JavaScript task",
      content: "body",
    });
    const completedEvents = [];
    for await (const event of invoker.streamTask(completedId, { pollIntervalMs: 10 })) {
      completedEvents.push(event.type);
    }
    expect(completedEvents).toEqual(expect.arrayContaining(["started", "completed"]));
    const completed = await waitForStatus(invoker, completedId, "succeeded");
    expect(completed.gameId).toBe(process.env.CROUPIER_GAME_ID || "e2e-game");
    expect(completed.env).toBe(process.env.CROUPIER_ENV || "e2e");

    const cancelledId = await invoker.startTask("mail.wait", { wait_ms: 30_000 });
    await waitForStatus(invoker, cancelledId, "running");
    await invoker.cancelTask(cancelledId);
    const cancelledEvents = [];
    for await (const event of invoker.streamTask(cancelledId, { pollIntervalMs: 10 })) {
      cancelledEvents.push(event.type);
    }
    expect(cancelledEvents).toContain("cancelled");
    await waitForStatus(invoker, cancelledId, "cancelled");
  });
});
