# Croupier Node.js SDK

TypeScript-first SDK for hosting Croupier game functions inside Node.js services.

## Features

- 🛰️ **Real gRPC pipeline** – spins up a local FunctionService gRPC server and registers with your nearest agent.
- 🔁 **Heartbeat + reconnection** – keeps the session alive and automatically retries after transient failures.
- 📦 **Handler registry** – strongly-typed descriptors with optional JSON schema metadata.
- 🧪 **Examples included** – `examples/main.ts` demonstrates multiple handlers and payload validation.

## Requirements

- Node.js ≥ 16
- pnpm ≥ 8 (or npm/yarn, but pnpm lockfile is provided)

## Quick Start

```bash
cd sdks/js
pnpm install
pnpm run build
```

### Minimal Usage (TypeScript)

```ts
import { createClient, FunctionDescriptor, FunctionHandler } from './src';

const config = {
  agentAddr: '127.0.0.1:19090',
  controlAddr: '127.0.0.1:19100', // optional: uploads provider manifest
  serviceId: 'inventory-service',
  serviceVersion: '1.2.3',
};

const client = createClient(config);

const addItem: FunctionHandler = async (_ctx, payload) => {
  const request = JSON.parse(payload);
  // ... mutate state ...
  return JSON.stringify({ status: 'ok', item_id: request.item_id });
};

const descriptor: FunctionDescriptor = {
  id: 'inventory.add_item',
  version: '1.0.0',
  description: 'Adds an item to the player inventory',
  input_schema: {
    type: 'object',
    required: ['player_id', 'item_id'],
    properties: {
      player_id: { type: 'string' },
      item_id: { type: 'string' },
      quantity: { type: 'number', default: 1 },
    },
  },
};

await client.registerFunction(descriptor, addItem);
await client.connect();

console.log('✅ inventory.add_item registered');
```

### Example App

```
# inside sdks/js
pnpm install
pnpm ts-node examples/main.ts
```

The example registers three handlers (`player.ban`, `wallet.transfer`, `shop.buy`) and logs invocations. Point it at a running agent (`127.0.0.1:19090` by default).

## Project Layout

```
sdks/js/
├── src/                # SDK source (TypeScript)
├── generated/          # Protobuf/gRPC bindings (connect-es)
├── examples/           # End-to-end demo
├── dist/               # tsc output
└── package.json
```

## Roadmap

- Provider manifest upload via `ControlService.RegisterCapabilities`
- Rich runtime metrics + health probes
- First-class CommonJS/Esm dual build

Contributions welcome – open an issue or PR if you run into anything! 🧑‍💻
