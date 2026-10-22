package mongo

import (
	"context"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/pkg/cdc/recovery"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Auditor durably records recovery metadata without row images or credentials.
type Auditor struct {
	collection *mongo.Collection
}

// NewAuditor accepts an existing client and prepares its ledger and indexes.
func NewAuditor(ctx context.Context, client *mongo.Client) (*Auditor, error) {
	if client == nil {
		return nil, errors.New("mongo recovery auditor: client is required")
	}
	if _, err := client.Database(ledgerDatabase).Collection("audit_records").Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: 1}},
		Options: options.Index(),
	}); err != nil {
		return nil, fmt.Errorf("mongo recovery auditor: ensure schema: %w", err)
	}
	return &Auditor{collection: client.Database(ledgerDatabase).Collection("audit_records")}, nil
}

// Append durably records one audit event.
func (a *Auditor) Append(ctx context.Context, record recovery.AuditRecord) error {
	if a == nil || a.collection == nil {
		return errors.New("mongo recovery auditor: not open")
	}
	if record.RequestID == "" || record.Principal == "" || record.SourceID == "" ||
		record.TargetID == "" || record.Stage == "" || record.Mode == "" {
		return errors.New("mongo recovery auditor: record identity fields are required")
	}
	if record.At.IsZero() {
		record.At = currentTime()
	}
	if _, err := a.collection.InsertOne(ctx, record); err != nil {
		return fmt.Errorf("mongo recovery auditor: append: %w", err)
	}
	return nil
}
