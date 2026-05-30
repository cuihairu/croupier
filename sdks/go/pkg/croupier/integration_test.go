// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

//go:build integration
// +build integration

package croupier

// This file exists only to add the build tag "integration" to all integration tests.
// Tests with this build tag can be skipped during normal CI/CD builds by running:
//   go test -tags=!integration
//
// Integration tests require a running Croupier agent or external dependencies.
