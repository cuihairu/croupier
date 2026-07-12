package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

type playerRecord struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Level       int            `json:"level"`
	VIP         int            `json:"vip"`
	Gold        int64          `json:"gold"`
	Status      string         `json:"status"`
	Server      string         `json:"server"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
	LastLoginAt string         `json:"last_login_at"`
	Profile     map[string]any `json:"profile,omitempty"`
}

type orderRecord struct {
	ID         string         `json:"id"`
	PlayerID   string         `json:"player_id"`
	ProductID  string         `json:"product_id"`
	Amount     int64          `json:"amount"`
	Currency   string         `json:"currency"`
	Status     string         `json:"status"`
	Channel    string         `json:"channel"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type leaderboardEntry struct {
	PlayerID  string `json:"player_id"`
	Player    string `json:"player_name"`
	Score     int64  `json:"score"`
	Rank      int    `json:"rank"`
	UpdatedAt string `json:"updated_at"`
}

type itemRecord struct {
	ID        string `json:"id"`
	Template  string `json:"template_id"`
	Name      string `json:"name"`
	Quantity  int64  `json:"quantity"`
	Rarity    string `json:"rarity"`
	UpdatedAt string `json:"updated_at"`
}

type mailRecord struct {
	ID        string         `json:"id"`
	PlayerID  string         `json:"player_id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Status    string         `json:"status"`
	Reward    map[string]any `json:"reward,omitempty"`
	SentAt    string         `json:"sent_at"`
	UpdatedAt string         `json:"updated_at"`
	ExpireAt  string         `json:"expire_at,omitempty"`
}

type demoStore struct {
	mu          sync.RWMutex
	playerSeq   int64
	orderSeq    int64
	mailSeq     int64
	players     map[string]*playerRecord
	orders      map[string]*orderRecord
	leaderboard map[string]*leaderboardEntry
	inventories map[string]map[string]*itemRecord
	mails       map[string][]*mailRecord
}

func newDemoStore() *demoStore {
	now := time.Now().UTC().Format(time.RFC3339)
	players := map[string]*playerRecord{
		"player_1001": {
			ID:          "player_1001",
			Name:        "Alice",
			Level:       35,
			VIP:         3,
			Gold:        128800,
			Status:      "active",
			Server:      "s1",
			CreatedAt:   now,
			UpdatedAt:   now,
			LastLoginAt: now,
			Profile: map[string]any{
				"guild":    "星海旅团",
				"country":  "CN",
				"platform": "ios",
			},
		},
		"player_1002": {
			ID:          "player_1002",
			Name:        "Bob",
			Level:       42,
			VIP:         5,
			Gold:        256000,
			Status:      "active",
			Server:      "s2",
			CreatedAt:   now,
			UpdatedAt:   now,
			LastLoginAt: now,
			Profile: map[string]any{
				"guild":    "苍穹守卫",
				"country":  "US",
				"platform": "android",
			},
		},
	}

	return &demoStore{
		playerSeq: 1002,
		orderSeq:  3002,
		mailSeq:   5002,
		players:   players,
		orders: map[string]*orderRecord{
			"order_3001": {
				ID:        "order_3001",
				PlayerID:  "player_1001",
				ProductID: "com.croupier.gems.648",
				Amount:    6480,
				Currency:  "CNY",
				Status:    "paid",
				Channel:   "appstore",
				CreatedAt: now,
				UpdatedAt: now,
				Attributes: map[string]any{
					"region": "cn",
				},
			},
			"order_3002": {
				ID:        "order_3002",
				PlayerID:  "player_1002",
				ProductID: "battle.pass.s2",
				Amount:    68,
				Currency:  "USD",
				Status:    "pending",
				Channel:   "googleplay",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		leaderboard: map[string]*leaderboardEntry{
			"player_1002": {PlayerID: "player_1002", Player: "Bob", Score: 98500, Rank: 1, UpdatedAt: now},
			"player_1001": {PlayerID: "player_1001", Player: "Alice", Score: 91200, Rank: 2, UpdatedAt: now},
		},
		inventories: map[string]map[string]*itemRecord{
			"player_1001": {
				"gold_coin": {
					ID:        "item_gold_coin",
					Template:  "gold_coin",
					Name:      "金币",
					Quantity:  128800,
					Rarity:    "common",
					UpdatedAt: now,
				},
				"hero_ticket": {
					ID:        "item_hero_ticket",
					Template:  "hero_ticket",
					Name:      "英雄招募券",
					Quantity:  12,
					Rarity:    "rare",
					UpdatedAt: now,
				},
			},
		},
		mails: map[string][]*mailRecord{
			"player_1001": {
				{
					ID:       "mail_5001",
					PlayerID: "player_1001",
					Title:    "开服奖励",
					Content:  "欢迎来到 Croupier Demo World",
					Status:   "unread",
					Reward: map[string]any{
						"gold": 10000,
						"item": "hero_ticket",
					},
					SentAt:    now,
					UpdatedAt: now,
				},
			},
		},
	}
}

func (s *demoStore) now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func decodePayload(payload []byte) (map[string]any, error) {
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("invalid json payload: %w", err)
	}
	return body, nil
}

func encodeResponse(data map[string]any) ([]byte, error) {
	data["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	return json.Marshal(data)
}

func stringValue(body map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := body[key]; ok {
			switch value := raw.(type) {
			case string:
				if strings.TrimSpace(value) != "" {
					return strings.TrimSpace(value)
				}
			case fmt.Stringer:
				return strings.TrimSpace(value.String())
			case float64:
				return strconv.FormatInt(int64(value), 10)
			case int:
				return strconv.Itoa(value)
			case int64:
				return strconv.FormatInt(value, 10)
			}
		}
	}
	return ""
}

func int64Value(body map[string]any, fallback int64, keys ...string) int64 {
	for _, key := range keys {
		if raw, ok := body[key]; ok {
			switch value := raw.(type) {
			case float64:
				return int64(value)
			case int:
				return int64(value)
			case int64:
				return value
			case string:
				parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
				if err == nil {
					return parsed
				}
			}
		}
	}
	return fallback
}

func intValue(body map[string]any, fallback int, keys ...string) int {
	return int(int64Value(body, int64(fallback), keys...))
}

func mapValue(body map[string]any, key string) map[string]any {
	raw, ok := body[key]
	if !ok {
		return nil
	}
	out, _ := raw.(map[string]any)
	return out
}

func (s *demoStore) nextPlayerID() string {
	s.playerSeq++
	return fmt.Sprintf("player_%d", s.playerSeq)
}

func (s *demoStore) nextOrderID() string {
	s.orderSeq++
	return fmt.Sprintf("order_%d", s.orderSeq)
}

func (s *demoStore) nextMailID() string {
	s.mailSeq++
	return fmt.Sprintf("mail_%d", s.mailSeq)
}

func (s *demoStore) playerCreate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := stringValue(body, "id", "player_id")
	if id == "" {
		id = s.nextPlayerID()
	}
	now := s.now()
	record := &playerRecord{
		ID:          id,
		Name:        firstNonEmpty(stringValue(body, "name"), "Player-"+id),
		Level:       intValue(body, 1, "level"),
		VIP:         intValue(body, 0, "vip"),
		Gold:        int64Value(body, 0, "gold"),
		Status:      firstNonEmpty(stringValue(body, "status"), "active"),
		Server:      firstNonEmpty(stringValue(body, "server"), "s1"),
		CreatedAt:   now,
		UpdatedAt:   now,
		LastLoginAt: now,
		Profile:     mapValue(body, "profile"),
	}
	s.players[id] = record

	return encodeResponse(map[string]any{
		"status": "success",
		"action": "player.create",
		"player": record,
	})
}

func (s *demoStore) playerGet(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id", "id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.players[playerID]
	if !ok {
		return encodeResponse(map[string]any{
			"status":  "not_found",
			"message": "player not found",
			"player":  nil,
		})
	}

	return encodeResponse(map[string]any{
		"status": "success",
		"action": "player.get",
		"player": record,
	})
}

func (s *demoStore) playerUpdate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id", "id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return encodeResponse(map[string]any{
			"status":  "not_found",
			"message": "player not found",
		})
	}

	if name := stringValue(body, "name"); name != "" {
		record.Name = name
	}
	if _, ok := body["level"]; ok {
		record.Level = intValue(body, record.Level, "level")
	}
	if _, ok := body["vip"]; ok {
		record.VIP = intValue(body, record.VIP, "vip")
	}
	if _, ok := body["gold"]; ok {
		record.Gold = int64Value(body, record.Gold, "gold")
	}
	if status := stringValue(body, "status"); status != "" {
		record.Status = status
	}
	if server := stringValue(body, "server"); server != "" {
		record.Server = server
	}
	if profile := mapValue(body, "profile"); profile != nil {
		record.Profile = profile
	}
	record.UpdatedAt = s.now()

	return encodeResponse(map[string]any{
		"status": "success",
		"action": "player.update",
		"player": record,
	})
}

func (s *demoStore) playerDelete(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id", "id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.players, playerID)
	delete(s.inventories, playerID)
	delete(s.mails, playerID)
	delete(s.leaderboard, playerID)

	return encodeResponse(map[string]any{
		"status":    "success",
		"action":    "player.delete",
		"player_id": playerID,
	})
}

func (s *demoStore) playerList(ctx context.Context, payload []byte) ([]byte, error) {
	_, _ = decodePayload(payload)

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*playerRecord, 0, len(s.players))
	for _, item := range s.players {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	return encodeResponse(map[string]any{
		"status": "success",
		"action": "player.list",
		"items":  items,
		"total":  len(items),
	})
}

func (s *demoStore) orderCreate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := stringValue(body, "order_id", "id")
	if id == "" {
		id = s.nextOrderID()
	}
	now := s.now()
	record := &orderRecord{
		ID:         id,
		PlayerID:   stringValue(body, "player_id"),
		ProductID:  firstNonEmpty(stringValue(body, "product_id"), "product.demo"),
		Amount:     int64Value(body, 0, "amount"),
		Currency:   firstNonEmpty(stringValue(body, "currency"), "CNY"),
		Status:     firstNonEmpty(stringValue(body, "status"), "created"),
		Channel:    firstNonEmpty(stringValue(body, "channel"), "gm"),
		CreatedAt:  now,
		UpdatedAt:  now,
		Attributes: mapValue(body, "attributes"),
	}
	s.orders[id] = record

	return encodeResponse(map[string]any{
		"status": "success",
		"action": "order.create",
		"order":  record,
	})
}

func (s *demoStore) orderGet(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "order_id", "id")
	if orderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.orders[orderID]
	if !ok {
		return encodeResponse(map[string]any{"status": "not_found", "message": "order not found"})
	}
	return encodeResponse(map[string]any{"status": "success", "action": "order.get", "order": record})
}

func (s *demoStore) orderUpdate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "order_id", "id")
	if orderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.orders[orderID]
	if !ok {
		return encodeResponse(map[string]any{"status": "not_found", "message": "order not found"})
	}
	if status := stringValue(body, "status"); status != "" {
		record.Status = status
	}
	if channel := stringValue(body, "channel"); channel != "" {
		record.Channel = channel
	}
	if _, ok := body["amount"]; ok {
		record.Amount = int64Value(body, record.Amount, "amount")
	}
	if attrs := mapValue(body, "attributes"); attrs != nil {
		record.Attributes = attrs
	}
	record.UpdatedAt = s.now()

	return encodeResponse(map[string]any{"status": "success", "action": "order.update", "order": record})
}

func (s *demoStore) orderDelete(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "order_id", "id")
	if orderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.orders, orderID)

	return encodeResponse(map[string]any{"status": "success", "action": "order.delete", "order_id": orderID})
}

func (s *demoStore) orderList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*orderRecord, 0, len(s.orders))
	for _, item := range s.orders {
		if playerID != "" && item.PlayerID != playerID {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	return encodeResponse(map[string]any{"status": "success", "action": "order.list", "items": items, "total": len(items)})
}

func (s *demoStore) leaderboardList(ctx context.Context, payload []byte) ([]byte, error) {
	_, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]*leaderboardEntry, 0, len(s.leaderboard))
	for _, item := range s.leaderboard {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].PlayerID < items[j].PlayerID
		}
		return items[i].Score > items[j].Score
	})
	for index, item := range items {
		item.Rank = index + 1
	}

	return encodeResponse(map[string]any{"status": "success", "action": "leaderboard.list", "items": items, "total": len(items)})
}

func (s *demoStore) leaderboardUpsert(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	playerName := playerID
	if player, ok := s.players[playerID]; ok && player.Name != "" {
		playerName = player.Name
	}
	entry := &leaderboardEntry{
		PlayerID:  playerID,
		Player:    playerName,
		Score:     int64Value(body, 0, "score"),
		UpdatedAt: s.now(),
	}
	s.leaderboard[playerID] = entry

	return encodeResponse(map[string]any{"status": "success", "action": "leaderboard.upsert", "entry": entry})
}

func (s *demoStore) leaderboardReset(ctx context.Context, payload []byte) ([]byte, error) {
	_, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaderboard = map[string]*leaderboardEntry{}

	return encodeResponse(map[string]any{"status": "success", "action": "leaderboard.reset"})
}

func (s *demoStore) inventoryList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	playerInv := s.inventories[playerID]
	items := make([]*itemRecord, 0, len(playerInv))
	for _, item := range playerInv {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Template < items[j].Template })

	return encodeResponse(map[string]any{"status": "success", "action": "inventory.list", "player_id": playerID, "items": items})
}

func (s *demoStore) inventoryGrant(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	templateID := stringValue(body, "template_id", "item_id")
	if playerID == "" || templateID == "" {
		return nil, fmt.Errorf("player_id and template_id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.inventories[playerID]; !ok {
		s.inventories[playerID] = map[string]*itemRecord{}
	}
	record, ok := s.inventories[playerID][templateID]
	if !ok {
		record = &itemRecord{
			ID:       "item_" + templateID,
			Template: templateID,
			Name:     firstNonEmpty(stringValue(body, "name"), templateID),
			Rarity:   firstNonEmpty(stringValue(body, "rarity"), "common"),
		}
		s.inventories[playerID][templateID] = record
	}
	record.Quantity += int64Value(body, 1, "quantity")
	record.UpdatedAt = s.now()

	return encodeResponse(map[string]any{"status": "success", "action": "inventory.grant", "player_id": playerID, "item": record})
}

func (s *demoStore) inventoryConsume(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	templateID := stringValue(body, "template_id", "item_id")
	quantity := int64Value(body, 1, "quantity")
	if playerID == "" || templateID == "" {
		return nil, fmt.Errorf("player_id and template_id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.inventories[playerID][templateID]
	if !ok {
		return encodeResponse(map[string]any{"status": "not_found", "message": "item not found"})
	}
	if record.Quantity < quantity {
		return encodeResponse(map[string]any{"status": "failed", "message": "insufficient quantity", "item": record})
	}
	record.Quantity -= quantity
	record.UpdatedAt = s.now()

	return encodeResponse(map[string]any{"status": "success", "action": "inventory.consume", "player_id": playerID, "item": record})
}

func (s *demoStore) mailSend(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	record := &mailRecord{
		ID:        s.nextMailID(),
		PlayerID:  playerID,
		Title:     firstNonEmpty(stringValue(body, "title"), "系统邮件"),
		Content:   firstNonEmpty(stringValue(body, "content"), "请查收奖励"),
		Status:    "unread",
		Reward:    mapValue(body, "reward"),
		SentAt:    now,
		UpdatedAt: now,
		ExpireAt:  stringValue(body, "expire_at"),
	}
	s.mails[playerID] = append(s.mails[playerID], record)

	return encodeResponse(map[string]any{"status": "success", "action": "mail.send", "mail": record})
}

func (s *demoStore) mailList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	if playerID == "" {
		return nil, fmt.Errorf("player_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := append([]*mailRecord(nil), s.mails[playerID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].SentAt > items[j].SentAt })

	return encodeResponse(map[string]any{"status": "success", "action": "mail.list", "player_id": playerID, "items": items, "total": len(items)})
}

func (s *demoStore) mailClaim(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "player_id")
	mailID := stringValue(body, "mail_id", "id")
	if playerID == "" || mailID == "" {
		return nil, fmt.Errorf("player_id and mail_id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mail := range s.mails[playerID] {
		if mail.ID == mailID {
			mail.Status = "claimed"
			mail.UpdatedAt = s.now()
			return encodeResponse(map[string]any{"status": "success", "action": "mail.claim", "mail": mail})
		}
	}

	return encodeResponse(map[string]any{"status": "not_found", "message": "mail not found"})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func registerFunction(client croupier.Client, desc croupier.FunctionDescriptor, handler func(context.Context, []byte) ([]byte, error)) error {
	if err := client.RegisterFunction(desc, handler); err != nil {
		return fmt.Errorf("register %s failed: %w", desc.ID, err)
	}
	log.Printf("registered function: %s", desc.ID)
	return nil
}

func registerGameDemoFunctions(client croupier.Client, store *demoStore) error {
	definitions := []struct {
		desc    croupier.FunctionDescriptor
		handler func(context.Context, []byte) ([]byte, error)
	}{
		{croupier.FunctionDescriptor{ID: "player.create", Version: "1.0.0", Category: "player", Risk: "medium", Entity: "player", Operation: "create", Enabled: true}, store.playerCreate},
		{croupier.FunctionDescriptor{ID: "player.get", Version: "1.0.0", Category: "player", Risk: "low", Entity: "player", Operation: "read", Enabled: true}, store.playerGet},
		{croupier.FunctionDescriptor{ID: "player.update", Version: "1.0.0", Category: "player", Risk: "medium", Entity: "player", Operation: "update", Enabled: true}, store.playerUpdate},
		{croupier.FunctionDescriptor{ID: "player.delete", Version: "1.0.0", Category: "player", Risk: "high", Entity: "player", Operation: "delete", Enabled: true}, store.playerDelete},
		{croupier.FunctionDescriptor{ID: "player.list", Version: "1.0.0", Category: "player", Risk: "low", Entity: "player", Operation: "read", Enabled: true}, store.playerList},
		{croupier.FunctionDescriptor{ID: "order.create", Version: "1.0.0", Category: "commerce", Risk: "medium", Entity: "order", Operation: "create", Enabled: true}, store.orderCreate},
		{croupier.FunctionDescriptor{ID: "order.get", Version: "1.0.0", Category: "commerce", Risk: "low", Entity: "order", Operation: "read", Enabled: true}, store.orderGet},
		{croupier.FunctionDescriptor{ID: "order.update", Version: "1.0.0", Category: "commerce", Risk: "medium", Entity: "order", Operation: "update", Enabled: true}, store.orderUpdate},
		{croupier.FunctionDescriptor{ID: "order.delete", Version: "1.0.0", Category: "commerce", Risk: "high", Entity: "order", Operation: "delete", Enabled: true}, store.orderDelete},
		{croupier.FunctionDescriptor{ID: "order.list", Version: "1.0.0", Category: "commerce", Risk: "low", Entity: "order", Operation: "read", Enabled: true}, store.orderList},
		{croupier.FunctionDescriptor{ID: "leaderboard.list", Version: "1.0.0", Category: "leaderboard", Risk: "low", Entity: "leaderboard", Operation: "read", Enabled: true}, store.leaderboardList},
		{croupier.FunctionDescriptor{ID: "leaderboard.upsert", Version: "1.0.0", Category: "leaderboard", Risk: "medium", Entity: "leaderboard", Operation: "update", Enabled: true}, store.leaderboardUpsert},
		{croupier.FunctionDescriptor{ID: "leaderboard.reset", Version: "1.0.0", Category: "leaderboard", Risk: "high", Entity: "leaderboard", Operation: "delete", Enabled: true}, store.leaderboardReset},
		{croupier.FunctionDescriptor{ID: "inventory.list", Version: "1.0.0", Category: "inventory", Risk: "low", Entity: "inventory", Operation: "read", Enabled: true}, store.inventoryList},
		{croupier.FunctionDescriptor{ID: "inventory.grant", Version: "1.0.0", Category: "inventory", Risk: "medium", Entity: "inventory", Operation: "create", Enabled: true}, store.inventoryGrant},
		{croupier.FunctionDescriptor{ID: "inventory.consume", Version: "1.0.0", Category: "inventory", Risk: "medium", Entity: "inventory", Operation: "delete", Enabled: true}, store.inventoryConsume},
		{croupier.FunctionDescriptor{ID: "mail.send", Version: "1.0.0", Category: "mail", Risk: "medium", Entity: "mail", Operation: "create", Enabled: true}, store.mailSend},
		{croupier.FunctionDescriptor{ID: "mail.list", Version: "1.0.0", Category: "mail", Risk: "low", Entity: "mail", Operation: "read", Enabled: true}, store.mailList},
		{croupier.FunctionDescriptor{ID: "mail.claim", Version: "1.0.0", Category: "mail", Risk: "medium", Entity: "mail", Operation: "update", Enabled: true}, store.mailClaim},
	}

	for _, item := range definitions {
		if err := registerFunction(client, item.desc, item.handler); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	agentAddr := getenv("CROUPIER_AGENT_ADDR", "127.0.0.1:19091")
	gameID := getenv("CROUPIER_GAME_ID", "demo-game")
	serviceID := getenv("CROUPIER_SERVICE_ID", "game-demo-service")
	env := getenv("CROUPIER_ENV", "development")

	config := &croupier.ClientConfig{
		AgentAddr:      agentAddr,
		GameID:         gameID,
		Env:            env,
		ServiceID:      serviceID,
		ServiceVersion: "1.0.0",
		TimeoutSeconds: 30,
		Insecure:       true,
	}

	log.Printf("starting game demo provider: agent=%s game=%s env=%s service=%s", agentAddr, gameID, env, serviceID)

	client := croupier.NewClient(config)
	store := newDemoStore()
	if err := registerGameDemoFunctions(client, store); err != nil {
		log.Fatalf("register game demo functions failed: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := client.Serve(ctx); err != nil {
		log.Fatalf("serve failed: %v", err)
	}

	if err := client.Close(); err != nil {
		log.Printf("close failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
