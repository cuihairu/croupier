---
title: JavaScript SDK
---

# JavaScript SDK

A TypeScript-first Node.js SDK for connecting to the Croupier Agent, registering functions, and handling invocations.

## Code Location

- `sdks/js`

## Features

- TypeScript-first with complete type definitions
- Supports modern Node.js runtimes
- Function descriptor and handler registration
- Aligned with the monorepo's unified protocol evolution

```ts
import { setFieldWidget, setFieldHint } from "croupier-sdk";

let desc = setFieldWidget(
  { id: "player.ban", version: "1.0.0" },
  "id",
  "Select",
);
desc = setFieldHint(desc, "id", "x-options-source", {
  functionId: "player.list",
  labelPath: "/items/*/name",
  valuePath: "/items/*/id",
});
```

## Installation

```bash
npm install croupier-js-sdk
```

## Quick Start

```ts
import { CroupierClient } from "croupier-js-sdk";

const client = new CroupierClient({
  agentAddr: "127.0.0.1:19091",
  gameId: "my-game",
  env: "development",
  insecure: true,
});

await client.connect();
await client.serve();
```

## Keep Reading

- [Guide](/sdks/js/guide/)
- [API Reference](/sdks/js/api/)
- [Repository README](https://github.com/cuihairu/croupier/tree/main/sdks/js)
