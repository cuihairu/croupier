package connector

import "github.com/cuihairu/croupier/pkg/cdc"

// Compatibility aliases. New reusable CDC code should import pkg/cdc.
type Capability = cdc.Capability

const (
	CapBeforeImage         = cdc.CapBeforeImage
	CapAfterImage          = cdc.CapAfterImage
	CapTransactionMetadata = cdc.CapTransactionMetadata
	CapSnapshot            = cdc.CapSnapshot
	CapResumableCheckpoint = cdc.CapResumableCheckpoint
	CapPushdown            = cdc.CapPushdown
	CapSchemaHistory       = cdc.CapSchemaHistory
)

type Capabilities = cdc.Capabilities

var AllCapabilities = cdc.AllCapabilities
var ParseCapabilities = cdc.ParseCapabilities
