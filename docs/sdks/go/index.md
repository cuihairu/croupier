---
title: Go SDK
---

# Go SDK

A high-performance Go SDK for connecting to the Croupier Agent, registering functions, and handling invocations.

## Code Location

- `sdks/go`

## Features

- Evolves in lockstep with `proto/**` in the monorepo
- Function descriptors, session management, and error handling supported
- Well suited to high-concurrency server-side integration

```go
metadata, _ := function.NewMetadataBuilder().
    SetID("player.ban").
    SetInputSchema(`{"type":"object","properties":{"id":{"type":"string"}}}`).
    SetFieldWidget("id", "Select").
    SetFieldHint("id", "x-options-source", map[string]interface{}{
        "functionId": "player.list",
        "labelPath":  "/items/*/name",
        "valuePath":  "/items/*/id",
    }).
    Build()
```

## Installation

```bash
go get github.com/cuihairu/croupier/sdks/go
```

## Quick Start

```go
package main

import "github.com/cuihairu/croupier/sdks/go/pkg/croupier"

func main() {
	cfg := &croupier.ClientConfig{
        AgentAddr: "127.0.0.1:19091",
		GameID:    "my-game",
		Env:       "development",
	}
	_ = croupier.NewClient(cfg)
}
```

## Keep Reading

- [Guide](/sdks/go/guide/)
- [API Reference](/sdks/go/api/)
- [Examples](/sdks/go/examples/)
