package connector

import "github.com/cuihairu/croupier/pkg/cdc"

type Operation = cdc.Operation

const (
	OpInsert = cdc.OpInsert
	OpUpdate = cdc.OpUpdate
	OpDelete = cdc.OpDelete
)

type Position = cdc.Position
type TxInfo = cdc.TxInfo
type ChangeEvent = cdc.ChangeEvent

var ParsePosition = cdc.ParsePosition
