// Package mongo applies journaled CDC transactions to MongoDB.
package mongo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const ledgerDatabase = "croupier_recovery"
const ledgerCollection = "applied_transactions"

// Applier applies complete source transactions to MongoDB atomically.
type Applier struct {
	client *mongo.Client
	ledger *mongo.Collection
}

// Open resolves the URI from uriEnv and prepares the idempotency ledger.
func Open(ctx context.Context, uriEnv string) (*Applier, error) {
	if strings.TrimSpace(uriEnv) == "" {
		return nil, errors.New("mongo recovery: uriEnv is required")
	}
	uri := os.Getenv(uriEnv)
	if uri == "" {
		return nil, fmt.Errorf("mongo recovery: environment variable %s is not set", uriEnv)
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo recovery: connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo recovery: ping: %w", err)
	}
	return NewApplier(ctx, client)
}

// NewApplier accepts an existing client, normally in tests.
func NewApplier(ctx context.Context, client *mongo.Client) (*Applier, error) {
	if client == nil {
		return nil, errors.New("mongo recovery: client is required")
	}
	if err := EnsureSchema(ctx, client); err != nil {
		return nil, err
	}
	ledger := client.Database(ledgerDatabase).Collection(ledgerCollection)
	return &Applier{client: client, ledger: ledger}, nil
}

// EnsureSchema creates the transaction idempotency ledger.
func EnsureSchema(ctx context.Context, client *mongo.Client) error {
	if client == nil {
		return errors.New("mongo recovery: client is required")
	}
	if _, err := client.Database(ledgerDatabase).Collection(ledgerCollection).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "request_id", Value: 1}, {Key: "transaction_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return fmt.Errorf("mongo recovery: ensure ledger: %w", err)
	}
	return nil
}

var currentTime = time.Now

// Close disconnects the client.
func (a *Applier) Close(ctx context.Context) error {
	if a == nil || a.client == nil {
		return nil
	}
	return a.client.Disconnect(ctx)
}

// Capabilities reports the supported row operations.
func (a *Applier) Capabilities(context.Context) (recovery.TargetCapabilities, error) {
	return recovery.TargetCapabilities{
		AtomicTransactions:     true,
		IdempotentTransactions: true,
		SupportedOperations:    []cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete},
	}, nil
}

// ValidateTransaction checks mapped images, keys and namespaces without writes.
func (a *Applier) ValidateTransaction(ctx context.Context, _ recovery.Plan, events []cdc.ChangeEvent) error {
	if len(events) == 0 {
		return errors.New("mongo recovery: empty transaction")
	}
	for _, event := range events {
		if !event.ColumnsMapped {
			return fmt.Errorf("mongo recovery: event %s columns are not mapped", event.ID)
		}
		var image map[string]json.RawMessage
		switch event.Operation {
		case cdc.OpInsert, cdc.OpUpdate:
			image = event.After
			if image == nil {
				return fmt.Errorf("mongo recovery: %s event %s has no after image", event.Operation, event.ID)
			}
		case cdc.OpDelete:
			image = event.Before
			if image == nil {
				return fmt.Errorf("mongo recovery: delete event %s has no before image", event.ID)
			}
		default:
			return fmt.Errorf("mongo recovery: unsupported operation %q", event.Operation)
		}
		if event.Database == "" || event.Table == "" {
			return fmt.Errorf("mongo recovery: event %s has no target database and collection", event.ID)
		}
		key, err := eventKey(event)
		if err != nil {
			return err
		}
		if _, err := documentID(image, key[0], event.ID); err != nil {
			return err
		}
		if err := collectionExists(ctx, a.client, event.Database, event.Table); err != nil {
			return err
		}
	}
	return nil
}

// ApplyTransaction commits all ledger writes and data changes atomically. The
// ledger reservation is checked before each retried callback body, making the
// WithTransaction retry contract safe.
func (a *Applier) ApplyTransaction(ctx context.Context, requestID, transactionID string, events []cdc.ChangeEvent) error {
	if requestID == "" || transactionID == "" || len(events) == 0 {
		return errors.New("mongo recovery: request, transaction and events are required")
	}
	if err := a.ValidateTransaction(ctx, recovery.Plan{}, events); err != nil {
		return err
	}
	eventHash, err := hashEvents(events)
	if err != nil {
		return err
	}
	session, err := a.client.StartSession()
	if err != nil {
		return fmt.Errorf("mongo recovery: start session: %w", err)
	}
	defer session.EndSession(context.Background())

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		var existing ledgerRecord
		err := a.ledger.FindOne(sessionCtx, bson.M{"request_id": requestID, "transaction_id": transactionID}).Decode(&existing)
		if err == nil {
			if existing.EventCount != int64(len(events)) || !bytes.Equal(existing.EventHash, eventHash) {
				return nil, fmt.Errorf("mongo recovery: transaction %s already applied with different content for request %s", transactionID, requestID)
			}
			return nil, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("mongo recovery: read applied transaction: %w", err)
		}
		if _, err := a.ledger.InsertOne(sessionCtx, ledgerRecord{
			RequestID: requestID, TransactionID: transactionID,
			EventCount: int64(len(events)), EventHash: eventHash,
		}); err != nil {
			return nil, fmt.Errorf("mongo recovery: reserve transaction: %w", err)
		}
		for _, event := range events {
			if err := applyEvent(sessionCtx, a.client, event); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	if err != nil {
		return err
	}
	return nil
}

type ledgerRecord struct {
	RequestID     string `bson:"request_id"`
	TransactionID string `bson:"transaction_id"`
	EventCount    int64  `bson:"event_count"`
	EventHash     []byte `bson:"event_hash"`
}

func hashEvents(events []cdc.ChangeEvent) ([]byte, error) {
	encoded, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("mongo recovery: encode transaction hash: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return sum[:], nil
}

func applyEvent(ctx context.Context, client *mongo.Client, event cdc.ChangeEvent) error {
	collection := client.Database(event.Database).Collection(event.Table)
	key, err := eventKey(event)
	if err != nil {
		return err
	}
	switch event.Operation {
	case cdc.OpInsert:
		document, err := decodeDocument(event.After)
		if err != nil {
			return err
		}
		_, err = collection.InsertOne(ctx, document)
		return err
	case cdc.OpUpdate:
		document, err := decodeDocument(event.After)
		if err != nil {
			return err
		}
		filter, err := decodeDocument(keyFilter(event.After, key))
		if err != nil {
			return err
		}
		documentBSON, err := bson.Marshal(document)
		if err != nil {
			return err
		}
		var existing bson.M
		err = collection.FindOne(ctx, filter).Decode(&existing)
		if err == nil {
			var current bson.D
			for _, name := range sortedDocumentKeys(document) {
				current = append(current, bson.E{Key: name, Value: documentValue(document, name)})
			}
			currentBSON, err := bson.Marshal(current)
			if err != nil {
				return err
			}
			if bytes.Equal(documentBSON, currentBSON) {
				return nil
			}
			return fmt.Errorf("mongo recovery: update event %s conflicts with existing document", event.ID)
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
		result, err := collection.InsertOne(ctx, document)
		if err != nil {
			return err
		}
		_ = result
		return nil
	case cdc.OpDelete:
		filter, err := decodeDocument(keyFilter(event.Before, key))
		if err != nil {
			return err
		}
		result, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return err
		}
		if result.DeletedCount == 0 {
			return fmt.Errorf("mongo recovery: delete event %s matched no document", event.ID)
		}
		return nil
	default:
		return fmt.Errorf("mongo recovery: unsupported operation %q", event.Operation)
	}
}

func sortedDocumentKeys(document bson.D) []string {
	keys := make([]string, 0, len(document))
	for _, field := range document {
		keys = append(keys, field.Key)
	}
	sort.Strings(keys)
	return keys
}

func documentValue(document bson.D, key string) any {
	for _, field := range document {
		if field.Key == key {
			return field.Value
		}
	}
	return nil
}

func collectionExists(ctx context.Context, client *mongo.Client, database, collection string) error {
	names, err := client.Database(database).ListCollectionNames(ctx, bson.D{})
	if err != nil {
		return fmt.Errorf("mongo recovery: inspect %s.%s: %w", database, collection, err)
	}
	for _, name := range names {
		if name == collection {
			return nil
		}
	}
	return fmt.Errorf("mongo recovery: collection %s.%s does not exist", database, collection)
}

func eventKey(event cdc.ChangeEvent) ([]string, error) {
	if len(event.Key) != 1 {
		return nil, fmt.Errorf("mongo recovery: event %s must have exactly one _id key", event.ID)
	}
	if event.Key[0] != "_id" {
		return nil, fmt.Errorf("mongo recovery: event %s key is %q, want _id", event.ID, event.Key[0])
	}
	return event.Key, nil
}

func keyFilter(image map[string]json.RawMessage, key []string) map[string]json.RawMessage {
	filter := make(map[string]json.RawMessage, len(key))
	for _, column := range key {
		filter[column] = image[column]
	}
	return filter
}

func decodeDocument(image map[string]json.RawMessage) (bson.D, error) {
	if len(image) == 0 {
		return nil, errors.New("mongo recovery: empty document image")
	}
	fields := make([]string, 0, len(image))
	for name := range image {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	buffer := &bytes.Buffer{}
	writer := bson.NewDocumentWriter(buffer)
	documentWriter, err := writer.WriteDocument()
	if err != nil {
		return nil, fmt.Errorf("mongo recovery: begin document: %w", err)
	}
	if err := writeDocumentFields(documentWriter, image, fields); err != nil {
		return nil, err
	}
	var document bson.D
	if err := bson.Unmarshal(buffer.Bytes(), &document); err != nil {
		return nil, fmt.Errorf("mongo recovery: decode document: %w", err)
	}
	return document, nil
}

func writeDocumentFields(documentWriter bson.DocumentWriter, image map[string]json.RawMessage, fields []string) error {
	for _, name := range fields {
		valueWriter, err := documentWriter.WriteDocumentElement(name)
		if err != nil {
			return fmt.Errorf("mongo recovery: encode key %q: %w", name, err)
		}
		valueReader, err := extJSONReader(image[name])
		if err != nil {
			return fmt.Errorf("mongo recovery: decode value %q: %w", name, err)
		}
		if err := copyValue(bson.NewEncoder(valueWriter), bson.NewDecoder(valueReader)); err != nil {
			return fmt.Errorf("mongo recovery: copy value %q: %w", name, err)
		}
	}
	return documentWriter.WriteDocumentEnd()
}

func extJSONReader(raw json.RawMessage) (bson.ValueReader, error) {
	return bson.NewExtJSONValueReader(bytes.NewReader(raw), false)
}

func copyValue(encoder *bson.Encoder, decoder *bson.Decoder) error {
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	return encoder.Encode(value)
}

func documentID(image map[string]json.RawMessage, keyColumn, eventID string) (any, error) {
	raw, ok := image[keyColumn]
	if !ok {
		return nil, fmt.Errorf("mongo recovery: event %s key %q is missing", eventID, keyColumn)
	}
	reader, err := extJSONReader(raw)
	if err != nil {
		return nil, fmt.Errorf("mongo recovery: decode key %q: %w", keyColumn, err)
	}
	var value any
	if err := bson.NewDecoder(reader).Decode(&value); err != nil {
		return nil, fmt.Errorf("mongo recovery: decode key %q: %w", keyColumn, err)
	}
	return value, nil
}
