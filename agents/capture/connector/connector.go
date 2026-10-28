// Package connector preserves the historical capture-agent import path.
// Generic CDC consumers should depend on pkg/cdc directly.
package connector

import "github.com/cuihairu/croupier/pkg/cdc"

type SourceConfig = cdc.SourceConfig
type ColumnMeta = cdc.ColumnMeta
type TableMeta = cdc.TableMeta
type DiscoveryRequest = cdc.DiscoveryRequest
type DiscoveryResult = cdc.DiscoveryResult
type SnapshotRequest = cdc.SnapshotRequest
type PollRequest = cdc.PollRequest
type SourceConnector = cdc.SourceConnector
type TableSchema = cdc.TableSchema
type SchemaHistorySource = cdc.SchemaHistorySource

var ErrUnsupportedSnapshot = cdc.ErrUnsupportedSnapshot
