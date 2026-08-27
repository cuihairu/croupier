/**
 * Tests for the Descriptor v2 OpenAPI import helper.
 */

import {
  FunctionDescriptor,
  FunctionHandler,
  registerFromOpenAPI,
  ImportOptions,
} from "./index";
import type { RegistrationTarget } from "./openapi";

class RecordingClient implements RegistrationTarget {
  readonly descriptors = new Map<string, FunctionDescriptor>();
  readonly handlers = new Map<string, FunctionHandler>();

  registerFunction(
    descriptor: FunctionDescriptor,
    handler: FunctionHandler,
  ): void {
    this.descriptors.set(descriptor.id, descriptor);
    this.handlers.set(descriptor.id, handler);
  }
}

const SPEC = {
  openapi: "3.0.3",
  info: { title: "GM API", version: "1.0.0" },
  paths: {
    "/players/{id}/ban": {
      put: {
        operationId: "player_ban",
        summary: "Ban player",
        description: "Bans a player account",
        tags: ["gm"],
        "x-resource": "player",
        "x-operation": "ban",
        "x-capability": "action",
        "x-execution": "sync",
        "x-permission": "player.ban",
        "x-risk": "high",
        "x-approval": { required: true, policyKey: "player-ban-v2" },
        requestBody: {
          content: {
            "application/json": {
              schema: {
                type: "object",
                required: ["playerId"],
                properties: {
                  playerId: { type: "string", description: "Player ID" },
                },
              },
            },
          },
        },
        responses: {
          200: {
            content: {
              "application/json": {
                schema: { type: "object", properties: { ok: { type: "boolean" } } },
              },
            },
          },
        },
      },
    },
    "/players/export": {
      post: {
        operationId: "player_export",
        description: "Exports players",
        "x-resource": "player",
        "x-capability": "report",
        "x-execution": "task",
        "x-risk": "low",
        "x-approval": { required: false },
        responses: {
          200: { content: { "application/json": { schema: { type: "object" } } } },
        },
      },
    },
    "/players": {
      get: {
        tags: ["query"],
        "x-capability": "collection_query",
        "x-risk": "warning",
        responses: {
          200: { content: { "application/json": { schema: { type: "array" } } } },
        },
      },
    },
  },
};

function makeHandlers(...ids: string[]): Map<string, FunctionHandler> {
  return new Map(ids.map((id) => [id, ((): FunctionHandler => async () => "{}")()]));
}

describe("registerFromOpenAPI (Descriptor v2)", () => {
  it("maps v2 extensions onto lowerCamelCase descriptor fields", () => {
    const client = new RecordingClient();
    registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    const descriptor = client.descriptors.get("player_ban")!;
    expect(Object.keys(descriptor)).toEqual(
      expect.arrayContaining([
        "inputSchema",
        "outputSchema",
        "approvalRequired",
        "approvalPolicyKey",
        "capability",
        "execution",
      ]),
    );
    expect(descriptor.resource).toBe("player");
    expect(descriptor.operation).toBe("ban");
    expect(descriptor.capability).toBe("action");
    expect(descriptor.execution).toBe("sync");
    expect(descriptor.permission).toBe("player.ban");
    expect(descriptor.risk).toBe("high");
    expect(descriptor.approvalRequired).toBe(true);
    expect(descriptor.approvalPolicyKey).toBe("player-ban-v2");
    expect(descriptor.inputSchema).toEqual({
      type: "object",
      required: ["playerId"],
      properties: { playerId: { type: "string", description: "Player ID" } },
    });
    expect(descriptor.outputSchema).toEqual({
      type: "object",
      properties: { ok: { type: "boolean" } },
    });
  });

  it("normalizes legacy risk aliases to the v2 vocabulary", () => {
    const client = new RecordingClient();
    registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    expect(client.descriptors.get("player_export")!.risk).toBe("safe");
    expect(client.descriptors.get("player_export")!.approvalRequired).toBe(false);
    expect(client.descriptors.get("player_export")!.approvalPolicyKey).toBeUndefined();
    expect(client.descriptors.get("players")!.risk).toBe("warning");
  });

  it("derives id from the path and title-cases the name when operationId is missing", () => {
    const client = new RecordingClient();
    registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    const descriptor = client.descriptors.get("players")!;
    expect(descriptor.id).toBe("players");
    expect(descriptor.name).toBe("Players");
    expect(descriptor.summary).toBe("Players");
    expect(descriptor.capability).toBe("collection_query");
  });

  it("uses summary as name and operationId title-case as fallback", () => {
    const client = new RecordingClient();
    registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    expect(client.descriptors.get("player_ban")!.name).toBe("Ban player");
    expect(client.descriptors.get("player_export")!.name).toBe("Player Export");
  });

  it("applies import options including defaultTimeoutMs", () => {
    const client = new RecordingClient();
    const options: ImportOptions = {
      resourcePrefix: "game",
      tagPrefix: "svc-",
      defaultTimeoutMs: 60000,
    };
    registerFromOpenAPI(
      client,
      SPEC,
      options,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    const descriptor = client.descriptors.get("player_ban")!;
    expect(descriptor.resource).toBe("game.player");
    expect(descriptor.tags).toEqual(["svc-gm"]);
    expect(descriptor.timeoutMs).toBe(60000);
  });

  it("throws on invalid capability values", () => {
    const client = new RecordingClient();
    expect(() =>
      registerFromOpenAPI(
        client,
        {
          paths: {
            "/players": {
              get: {
                operationId: "player_list",
                "x-capability": "list",
                responses: { 200: { description: "ok" } },
              },
            },
          },
        },
        undefined,
        undefined,
        makeHandlers("player_list"),
      ),
    ).toThrow("convert operation player_list failed: invalid x-capability");
  });

  it("skips invalid operations when continueOnError is set", () => {
    const client = new RecordingClient();
    const registered = registerFromOpenAPI(
      client,
      {
        paths: {
          "/bad": {
            get: {
              operationId: "bad_fn",
              "x-execution": "async",
              responses: { 200: { description: "ok" } },
            },
          },
          "/good": {
            get: {
              operationId: "good_fn",
              "x-execution": "sync",
              responses: { 200: { description: "ok" } },
            },
          },
        },
      },
      { continueOnError: true },
      undefined,
      makeHandlers("bad_fn", "good_fn"),
    );

    expect(registered).toEqual(["good_fn"]);
    expect(client.descriptors.has("good_fn")).toBe(true);
  });

  it("throws on malformed x-approval extensions", () => {
    const client = new RecordingClient();
    expect(() =>
      registerFromOpenAPI(
        client,
        {
          paths: {
            "/players": {
              post: {
                operationId: "player_kick",
                "x-approval": "yes",
                responses: { 200: { description: "ok" } },
              },
            },
          },
        },
        undefined,
        undefined,
        makeHandlers("player_kick"),
      ),
    ).toThrow("x-approval for player_kick must be an object");
  });

  it("registers handlers resolved by function id", () => {
    const client = new RecordingClient();
    registerFromOpenAPI(
      client,
      SPEC,
      undefined,
      undefined,
      makeHandlers("player_ban", "player_export", "players"),
    );

    expect(client.handlers.has("player_ban")).toBe(true);
    expect(client.handlers.has("player_export")).toBe(true);
    expect(client.handlers.has("players")).toBe(true);
  });
});
