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
	CreatedAt   string         `json:"createdAt"`
	UpdatedAt   string         `json:"updatedAt"`
	LastLoginAt string         `json:"lastLoginAt"`
	Profile     map[string]any `json:"profile,omitempty"`
}

type orderRecord struct {
	ID         string         `json:"id"`
	PlayerID   string         `json:"playerId"`
	ProductID  string         `json:"productId"`
	Amount     int64          `json:"amount"`
	Currency   string         `json:"currency"`
	Status     string         `json:"status"`
	Channel    string         `json:"channel"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

type leaderboardEntry struct {
	ID        string `json:"id"`
	PlayerID  string `json:"playerId"`
	Player    string `json:"playerName"`
	Score     int64  `json:"score"`
	Rank      int    `json:"rank"`
	UpdatedAt string `json:"updatedAt"`
}

type itemRecord struct {
	ID        string `json:"id"`
	Template  string `json:"templateId"`
	Name      string `json:"name"`
	Quantity  int64  `json:"quantity"`
	Rarity    string `json:"rarity"`
	UpdatedAt string `json:"updatedAt"`
}

type mailRecord struct {
	ID        string         `json:"id"`
	PlayerID  string         `json:"playerId"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Status    string         `json:"status"`
	Reward    map[string]any `json:"reward,omitempty"`
	SentAt    string         `json:"sentAt"`
	UpdatedAt string         `json:"updatedAt"`
	ExpireAt  string         `json:"expireAt,omitempty"`
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
			"player_1002": {ID: "player_1002", PlayerID: "player_1002", Player: "Bob", Score: 98500, Rank: 1, UpdatedAt: now},
			"player_1001": {ID: "player_1001", PlayerID: "player_1001", Player: "Alice", Score: 91200, Rank: 2, UpdatedAt: now},
		},
		inventories: map[string]map[string]*itemRecord{
			"player_1001": {
				"gold_coin": {
					ID:        inventoryItemID("player_1001", "gold_coin"),
					Template:  "gold_coin",
					Name:      "金币",
					Quantity:  128800,
					Rarity:    "common",
					UpdatedAt: now,
				},
				"hero_ticket": {
					ID:        inventoryItemID("player_1001", "hero_ticket"),
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
						"item": "heroTicket",
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

func encodeResponse(data any) ([]byte, error) {
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

func stringSliceValue(body map[string]any, keys ...string) []string {
	for _, key := range keys {
		raw, ok := body[key]
		if !ok {
			continue
		}
		if items, ok := raw.([]any); ok {
			out := make([]string, 0, len(items))
			for _, item := range items {
				if text, ok := item.(string); ok && text != "" {
					out = append(out, text)
				}
			}
			return out
		}
	}
	return nil
}

func mapValue(body map[string]any, key string) map[string]any {
	raw, ok := body[key]
	if !ok {
		return nil
	}
	out, _ := raw.(map[string]any)
	return out
}

func inventoryItemID(playerID, templateID string) string {
	return "item_" + playerID + "_" + templateID
}

func paginate[T any](items []T, body map[string]any) ([]T, int, int) {
	page := intValue(body, 1, "page")
	pageSize := intValue(body, 20, "pageSize")
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}, page, pageSize
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], page, pageSize
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

	id := stringValue(body, "id", "playerId")
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

	return encodeResponse(record)
}

func (s *demoStore) playerGet(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId", "id")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not found", playerID)
	}

	return encodeResponse(record)
}

func (s *demoStore) playerUpdate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId", "id")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not found", playerID)
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

	return encodeResponse(record)
}

func (s *demoStore) playerDelete(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId", "id")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.players, playerID)
	delete(s.inventories, playerID)
	delete(s.mails, playerID)
	delete(s.leaderboard, playerID)

	return encodeResponse(map[string]any{"id": playerID, "deleted": true})
}

// playerBan bans a single player (row action with optional form fields).
func (s *demoStore) playerBan(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId", "id")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}
	reason := stringValue(body, "reason")
	durationHours := 0
	if _, ok := body["durationHours"]; ok {
		durationHours = intValue(body, 0, "durationHours")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not found", playerID)
	}
	record.Status = "banned"
	record.UpdatedAt = s.now()

	response := map[string]any{
		"id":     playerID,
		"banned": true,
		"status": record.Status,
	}
	if reason != "" {
		response["reason"] = reason
	}
	if durationHours > 0 {
		response["durationHours"] = durationHours
	}
	return encodeResponse(response)
}

// playerBatchBan bans multiple players at once (selection/batch action).
func (s *demoStore) playerBatchBan(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	ids := stringSliceValue(body, "ids", "playerIds")
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids is required")
	}
	reason := stringValue(body, "reason")

	s.mu.Lock()
	defer s.mu.Unlock()

	banned := make([]string, 0, len(ids))
	missing := make([]string, 0)
	for _, id := range ids {
		record, ok := s.players[id]
		if !ok {
			missing = append(missing, id)
			continue
		}
		record.Status = "banned"
		record.UpdatedAt = s.now()
		banned = append(banned, id)
	}

	response := map[string]any{
		"banned": banned,
		"count":  len(banned),
	}
	if len(missing) > 0 {
		response["missing"] = missing
	}
	if reason != "" {
		response["reason"] = reason
	}
	return encodeResponse(response)
}

// playerRecharge grants gold to a single player (row action with form).
func (s *demoStore) playerRecharge(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId", "id")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}
	amount := 0
	if _, ok := body["amount"]; ok {
		amount = intValue(body, 0, "amount")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return nil, fmt.Errorf("player %q not found", playerID)
	}
	record.Gold += int64(amount)
	record.UpdatedAt = s.now()

	return encodeResponse(map[string]any{
		"id":    playerID,
		"gold":  record.Gold,
		"added": amount,
	})
}

func (s *demoStore) playerList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*playerRecord, 0, len(s.players))
	for _, item := range s.players {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })

	total := len(items)
	items, page, pageSize := paginate(items, body)
	return encodeResponse(map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *demoStore) orderCreate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := stringValue(body, "orderId", "id")
	if id == "" {
		id = s.nextOrderID()
	}
	now := s.now()
	record := &orderRecord{
		ID:         id,
		PlayerID:   stringValue(body, "playerId"),
		ProductID:  firstNonEmpty(stringValue(body, "productId"), "product.demo"),
		Amount:     int64Value(body, 0, "amount"),
		Currency:   firstNonEmpty(stringValue(body, "currency"), "CNY"),
		Status:     firstNonEmpty(stringValue(body, "status"), "created"),
		Channel:    firstNonEmpty(stringValue(body, "channel"), "gm"),
		CreatedAt:  now,
		UpdatedAt:  now,
		Attributes: mapValue(body, "attributes"),
	}
	s.orders[id] = record

	return encodeResponse(record)
}

func (s *demoStore) orderGet(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "orderId", "id")
	if orderID == "" {
		return nil, fmt.Errorf("orderId is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %q not found", orderID)
	}
	return encodeResponse(record)
}

func (s *demoStore) orderUpdate(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "orderId", "id")
	if orderID == "" {
		return nil, fmt.Errorf("orderId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %q not found", orderID)
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

	return encodeResponse(record)
}

func (s *demoStore) orderDelete(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	orderID := stringValue(body, "orderId", "id")
	if orderID == "" {
		return nil, fmt.Errorf("orderId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.orders, orderID)

	return encodeResponse(map[string]any{"id": orderID, "deleted": true})
}

func (s *demoStore) orderList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")

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

	total := len(items)
	items, page, pageSize := paginate(items, body)
	return encodeResponse(map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *demoStore) leaderboardList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
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

	total := len(items)
	items, page, pageSize := paginate(items, body)
	return encodeResponse(map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *demoStore) leaderboardUpsert(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	playerName := playerID
	if player, ok := s.players[playerID]; ok && player.Name != "" {
		playerName = player.Name
	}
	entry := &leaderboardEntry{
		ID:        playerID,
		PlayerID:  playerID,
		Player:    playerName,
		Score:     int64Value(body, 0, "score"),
		UpdatedAt: s.now(),
	}
	s.leaderboard[playerID] = entry

	return encodeResponse(entry)
}

func (s *demoStore) leaderboardReset(ctx context.Context, payload []byte) ([]byte, error) {
	_, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leaderboard = map[string]*leaderboardEntry{}

	return encodeResponse(map[string]any{"reset": true})
}

func (s *demoStore) inventoryList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	playerInv := s.inventories[playerID]
	items := make([]*itemRecord, 0, len(playerInv))
	for _, item := range playerInv {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Template < items[j].Template })

	total := len(items)
	items, page, pageSize := paginate(items, body)
	return encodeResponse(map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *demoStore) inventoryGrant(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	templateID := stringValue(body, "templateId", "itemId")
	if playerID == "" || templateID == "" {
		return nil, fmt.Errorf("playerId and templateId are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.inventories[playerID]; !ok {
		s.inventories[playerID] = map[string]*itemRecord{}
	}
	record, ok := s.inventories[playerID][templateID]
	if !ok {
		record = &itemRecord{
			ID:       inventoryItemID(playerID, templateID),
			Template: templateID,
			Name:     firstNonEmpty(stringValue(body, "name"), templateID),
			Rarity:   firstNonEmpty(stringValue(body, "rarity"), "common"),
		}
		s.inventories[playerID][templateID] = record
	}
	record.Quantity += int64Value(body, 1, "quantity")
	record.UpdatedAt = s.now()

	return encodeResponse(record)
}

func (s *demoStore) inventoryConsume(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	templateID := stringValue(body, "templateId", "itemId")
	quantity := int64Value(body, 1, "quantity")
	if playerID == "" || templateID == "" {
		return nil, fmt.Errorf("playerId and templateId are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.inventories[playerID][templateID]
	if !ok {
		return nil, fmt.Errorf("inventory item %q for player %q not found", templateID, playerID)
	}
	if record.Quantity < quantity {
		return nil, fmt.Errorf("insufficient quantity for inventory item %q", templateID)
	}
	record.Quantity -= quantity
	record.UpdatedAt = s.now()

	return encodeResponse(record)
}

func (s *demoStore) mailSend(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
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
		ExpireAt:  stringValue(body, "expireAt"),
	}
	s.mails[playerID] = append(s.mails[playerID], record)

	return encodeResponse(record)
}

func (s *demoStore) mailList(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	if playerID == "" {
		return nil, fmt.Errorf("playerId is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	items := append([]*mailRecord(nil), s.mails[playerID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].SentAt > items[j].SentAt })

	total := len(items)
	items, page, pageSize := paginate(items, body)
	return encodeResponse(map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (s *demoStore) mailClaim(ctx context.Context, payload []byte) ([]byte, error) {
	body, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	playerID := stringValue(body, "playerId")
	mailID := stringValue(body, "mailId", "id")
	if playerID == "" || mailID == "" {
		return nil, fmt.Errorf("player_id and mailId are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mail := range s.mails[playerID] {
		if mail.ID == mailID {
			mail.Status = "claimed"
			mail.UpdatedAt = s.now()
			return encodeResponse(mail)
		}
	}

	return nil, fmt.Errorf("mail %q for player %q not found", mailID, playerID)
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
	desc = enrichDescriptor(desc)
	if err := validateDemoDescriptor(desc); err != nil {
		return err
	}
	if err := client.RegisterFunction(desc, handler); err != nil {
		return fmt.Errorf("register %s failed: %w", desc.ID, err)
	}
	log.Printf("registered function: %s", desc.ID)
	return nil
}

func enrichDescriptor(desc croupier.FunctionDescriptor) croupier.FunctionDescriptor {
	if desc.Tags == nil {
		desc.Tags = []string{desc.Resource, desc.Operation}
	}
	if desc.Summary == "" {
		desc.Summary = fmt.Sprintf("%s %s", desc.Resource, desc.Operation)
	}
	if desc.Description == "" {
		desc.Description = fmt.Sprintf("Demo function %s for %s %s operations.", desc.ID, desc.Resource, desc.Operation)
	}
	return desc
}

func validateDemoDescriptor(desc croupier.FunctionDescriptor) error {
	if strings.TrimSpace(desc.Capability) == "" {
		return fmt.Errorf("demo descriptor %s is missing capability", desc.ID)
	}
	if strings.TrimSpace(desc.InputSchema) == "" || !json.Valid([]byte(desc.InputSchema)) {
		return fmt.Errorf("demo descriptor %s has invalid input schema", desc.ID)
	}
	if strings.TrimSpace(desc.OutputSchema) == "" || !json.Valid([]byte(desc.OutputSchema)) {
		return fmt.Errorf("demo descriptor %s has invalid output schema", desc.ID)
	}
	return nil
}

type demoFunctionDefinition struct {
	desc    croupier.FunctionDescriptor
	handler func(context.Context, []byte) ([]byte, error)
}

const (
	playerSchema      = `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"},"gold":{"type":"integer"},"status":{"type":"string"},"server":{"type":"string"},"createdAt":{"type":"string","format":"date-time"},"updatedAt":{"type":"string","format":"date-time"},"lastLoginAt":{"type":"string","format":"date-time"},"profile":{"type":"object"}}}`
	orderSchema       = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"productId":{"type":"string"},"amount":{"type":"integer"},"currency":{"type":"string"},"status":{"type":"string"},"channel":{"type":"string"},"createdAt":{"type":"string","format":"date-time"},"updatedAt":{"type":"string","format":"date-time"},"attributes":{"type":"object"}}}`
	leaderboardSchema = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"playerName":{"type":"string"},"score":{"type":"integer"},"rank":{"type":"integer"},"updatedAt":{"type":"string","format":"date-time"}}}`
	inventorySchema   = `{"type":"object","properties":{"id":{"type":"string"},"templateId":{"type":"string"},"name":{"type":"string"},"quantity":{"type":"integer"},"rarity":{"type":"string"},"updatedAt":{"type":"string","format":"date-time"}}}`
	mailSchema        = `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"status":{"type":"string"},"reward":{"type":"object"},"sentAt":{"type":"string","format":"date-time"},"updatedAt":{"type":"string","format":"date-time"},"expireAt":{"type":"string","format":"date-time"}}}`
	deleteSchema      = `{"type":"object","properties":{"id":{"type":"string"},"deleted":{"type":"boolean"}},"required":["id","deleted"]}`
	resetSchema       = `{"type":"object","properties":{"reset":{"type":"boolean"}},"required":["reset"]}`
)

func demoCollectionSchema(itemSchema string) string {
	return `{"type":"object","properties":{"items":{"type":"array","items":` + itemSchema + `},"total":{"type":"integer"},"page":{"type":"integer"},"pageSize":{"type":"integer"}},"required":["items","total","page","pageSize"]}`
}

func gameDemoFunctionDefinitions(store *demoStore) []demoFunctionDefinition {
	return []demoFunctionDefinition{
		{croupier.FunctionDescriptor{ID: "player.create", Version: "1.0.0", Resource: "player", Operation: "create", Capability: "create", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"},"gold":{"type":"integer"},"status":{"type":"string"},"server":{"type":"string"},"profile":{"type":"object"}}}`, OutputSchema: playerSchema}, store.playerCreate},
		{croupier.FunctionDescriptor{ID: "player.get", Version: "1.0.0", Resource: "player", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: playerSchema}, store.playerGet},
		{croupier.FunctionDescriptor{ID: "player.update", Version: "1.0.0", Resource: "player", Operation: "update", Capability: "update", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"level":{"type":"integer"},"vip":{"type":"integer"},"gold":{"type":"integer"},"status":{"type":"string"},"server":{"type":"string"},"profile":{"type":"object"}},"required":["id"]}`, OutputSchema: playerSchema}, store.playerUpdate},
		{croupier.FunctionDescriptor{ID: "player.delete", Version: "1.0.0", Resource: "player", Operation: "delete", Capability: "delete", Execution: "sync", Risk: "danger", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: deleteSchema}, store.playerDelete},
		{croupier.FunctionDescriptor{ID: "player.list", Version: "1.0.0", Resource: "player", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}}}`, OutputSchema: demoCollectionSchema(playerSchema)}, store.playerList},
		{croupier.FunctionDescriptor{ID: "player.ban", Version: "1.0.0", Resource: "player", Operation: "ban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string","title":"玩家 ID","x-widget":"Select","x-options-source":{"functionId":"player.list","labelPath":"/items/*/name","valuePath":"/items/*/id"},"x-placeholder":"选择玩家"},"reason":{"type":"string","title":"封禁原因"},"durationHours":{"type":"integer","minimum":1,"maximum":8760,"title":"封禁时长（小时）"}},"required":["id"]}`, OutputSchema: `{"type":"object","properties":{"id":{"type":"string"},"banned":{"type":"boolean"},"status":{"type":"string"},"reason":{"type":"string"},"durationHours":{"type":"integer"}}}`}, store.playerBan},
		{croupier.FunctionDescriptor{ID: "player.batch_ban", Version: "1.0.0", Resource: "player", Operation: "batch_ban", Capability: "action", Execution: "sync", Risk: "high", Enabled: true, InputSchema: `{"type":"object","properties":{"ids":{"type":"array","items":{"type":"string"},"title":"玩家 ID 列表"},"reason":{"type":"string","title":"封禁原因"}},"required":["ids"]}`, OutputSchema: `{"type":"object","properties":{"banned":{"type":"array","items":{"type":"string"}},"count":{"type":"integer"},"missing":{"type":"array","items":{"type":"string"}},"reason":{"type":"string"}}}`}, store.playerBatchBan},
		{croupier.FunctionDescriptor{ID: "player.recharge", Version: "1.0.0", Resource: "player", Operation: "recharge", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string","title":"玩家 ID"},"amount":{"type":"integer","minimum":1,"maximum":1000000,"title":"充值金币"}},"required":["id"]}`, OutputSchema: `{"type":"object","properties":{"id":{"type":"string"},"gold":{"type":"integer"},"added":{"type":"integer"}}}`}, store.playerRecharge},

		{croupier.FunctionDescriptor{ID: "order.create", Version: "1.0.0", Resource: "order", Operation: "create", Capability: "create", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"playerId":{"type":"string"},"productId":{"type":"string"},"amount":{"type":"integer"},"currency":{"type":"string"},"status":{"type":"string"},"channel":{"type":"string"},"attributes":{"type":"object"}},"required":["playerId"]}`, OutputSchema: orderSchema}, store.orderCreate},
		{croupier.FunctionDescriptor{ID: "order.get", Version: "1.0.0", Resource: "order", Operation: "get", Capability: "item_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: orderSchema}, store.orderGet},
		{croupier.FunctionDescriptor{ID: "order.update", Version: "1.0.0", Resource: "order", Operation: "update", Capability: "update", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"amount":{"type":"integer"},"status":{"type":"string"},"channel":{"type":"string"},"attributes":{"type":"object"}},"required":["id"]}`, OutputSchema: orderSchema}, store.orderUpdate},
		{croupier.FunctionDescriptor{ID: "order.delete", Version: "1.0.0", Resource: "order", Operation: "delete", Capability: "delete", Execution: "sync", Risk: "danger", Enabled: true, InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, OutputSchema: deleteSchema}, store.orderDelete},
		{croupier.FunctionDescriptor{ID: "order.list", Version: "1.0.0", Resource: "order", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}}}`, OutputSchema: demoCollectionSchema(orderSchema)}, store.orderList},

		{croupier.FunctionDescriptor{ID: "leaderboard.list", Version: "1.0.0", Resource: "leaderboard", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}}}`, OutputSchema: demoCollectionSchema(leaderboardSchema)}, store.leaderboardList},
		{croupier.FunctionDescriptor{ID: "leaderboard.upsert", Version: "1.0.0", Resource: "leaderboard", Operation: "upsert", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"score":{"type":"integer"}},"required":["playerId","score"]}`, OutputSchema: leaderboardSchema}, store.leaderboardUpsert},
		{croupier.FunctionDescriptor{ID: "leaderboard.reset", Version: "1.0.0", Resource: "leaderboard", Operation: "reset", Capability: "action", Execution: "sync", Risk: "danger", Enabled: true, InputSchema: `{"type":"object","properties":{}}`, OutputSchema: resetSchema}, store.leaderboardReset},

		{croupier.FunctionDescriptor{ID: "inventory.list", Version: "1.0.0", Resource: "inventory", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}},"required":["playerId"]}`, OutputSchema: demoCollectionSchema(inventorySchema)}, store.inventoryList},
		{croupier.FunctionDescriptor{ID: "inventory.grant", Version: "1.0.0", Resource: "inventory", Operation: "grant", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"templateId":{"type":"string"},"quantity":{"type":"integer","minimum":1},"name":{"type":"string"},"rarity":{"type":"string"}},"required":["playerId","templateId"]}`, OutputSchema: inventorySchema}, store.inventoryGrant},
		{croupier.FunctionDescriptor{ID: "inventory.consume", Version: "1.0.0", Resource: "inventory", Operation: "consume", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"templateId":{"type":"string"},"quantity":{"type":"integer","minimum":1}},"required":["playerId","templateId"]}`, OutputSchema: inventorySchema}, store.inventoryConsume},

		{croupier.FunctionDescriptor{ID: "mail.send", Version: "1.0.0", Resource: "mail", Operation: "send", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"title":{"type":"string"},"content":{"type":"string"},"reward":{"type":"object"},"expireAt":{"type":"string","format":"date-time"}},"required":["playerId"]}`, OutputSchema: mailSchema}, store.mailSend},
		{croupier.FunctionDescriptor{ID: "mail.list", Version: "1.0.0", Resource: "mail", Operation: "list", Capability: "collection_query", Execution: "sync", Risk: "safe", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"page":{"type":"integer","minimum":1},"pageSize":{"type":"integer","minimum":1,"maximum":100}},"required":["playerId"]}`, OutputSchema: demoCollectionSchema(mailSchema)}, store.mailList},
		{croupier.FunctionDescriptor{ID: "mail.claim", Version: "1.0.0", Resource: "mail", Operation: "claim", Capability: "action", Execution: "sync", Risk: "warning", Enabled: true, InputSchema: `{"type":"object","properties":{"playerId":{"type":"string"},"id":{"type":"string"}},"required":["playerId","id"]}`, OutputSchema: mailSchema}, store.mailClaim},
	}
}

func registerGameDemoFunctions(client croupier.Client, store *demoStore) error {
	definitions := gameDemoFunctionDefinitions(store)

	for _, item := range definitions {
		if err := registerFunction(client, item.desc, item.handler); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	agentAddr := getenv("CROUPIER_AGENT_ADDR", "127.0.0.1:19091")
	gameID := getenv("CROUPIER_GAME_ID", "default")
	serviceID := getenv("CROUPIER_SERVICE_ID", "game-demo-service")
	env := getenv("CROUPIER_ENV", "dev")

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
