package model

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

const (
	TermDomainResource  = "resource"
	TermDomainOperation = "operation"
)

func NormalizeTermDictionaryDomain(domain string) (string, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return "", nil
	}
	switch domain {
	case TermDomainResource, TermDomainOperation:
		return domain, nil
	default:
		return "", fmt.Errorf("unsupported term dictionary domain %q; expected resource or operation", domain)
	}
}

type TermDictionaryModel struct {
	db *gorm.DB
}

func NewTermDictionaryModel(db *gorm.DB) *TermDictionaryModel {
	return &TermDictionaryModel{db: db}
}

func (m *TermDictionaryModel) List(ctx context.Context, domain string) ([]TermDictionary, error) {
	query := m.db.WithContext(ctx).Model(&TermDictionary{})
	domain, err := NormalizeTermDictionaryDomain(domain)
	if err != nil {
		return nil, err
	}
	if domain != "" {
		query = query.Where("domain = ?", domain)
	}
	var out []TermDictionary
	err = query.Order("sort_order ASC").Order("id ASC").Find(&out).Error
	return out, err
}

func (m *TermDictionaryModel) Upsert(ctx context.Context, item *TermDictionary) error {
	if item == nil {
		return nil
	}
	domain, err := NormalizeTermDictionaryDomain(item.Domain)
	if err != nil {
		return err
	}
	item.Domain = domain
	item.TermKey = strings.TrimSpace(strings.ToLower(item.TermKey))
	item.Alias = strings.TrimSpace(strings.ToLower(item.Alias))
	if item.Domain == "" || item.TermKey == "" || item.Alias == "" {
		return nil
	}
	var existing TermDictionary
	findErr := m.db.WithContext(ctx).
		Where("domain = ? AND alias = ?", item.Domain, item.Alias).
		First(&existing).Error
	if findErr == nil {
		// map 形式的 Updates 不经过 serializer，需手动序列化。
		updates := map[string]any{
			"term_key":   item.TermKey,
			"sort_order": item.SortOrder,
		}
		display, err := marshalTermDisplay(NormalizeTermDisplay(item.Display))
		if err != nil {
			return err
		}
		if display != nil {
			updates["display"] = display
		} else {
			updates["display"] = nil
		}
		return m.db.WithContext(ctx).Model(&existing).Updates(updates).Error
	}
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return findErr
	}
	item.Display = NormalizeTermDisplay(item.Display)
	return m.db.WithContext(ctx).Create(item).Error
}

func (m *TermDictionaryModel) AliasMap(ctx context.Context) (map[string]map[string]string, error) {
	items, err := m.List(ctx, "")
	if err != nil {
		return nil, err
	}
	out := map[string]map[string]string{
		TermDomainResource:  {},
		TermDomainOperation: {},
	}
	for _, it := range items {
		domain, err := NormalizeTermDictionaryDomain(it.Domain)
		if err != nil {
			return nil, err
		}
		if _, ok := out[domain]; !ok {
			out[domain] = map[string]string{}
		}
		alias := strings.TrimSpace(strings.ToLower(it.Alias))
		key := strings.TrimSpace(strings.ToLower(it.TermKey))
		if alias == "" || key == "" {
			continue
		}
		out[domain][alias] = key
	}
	return out, nil
}

func (m *TermDictionaryModel) DeleteByAlias(ctx context.Context, domain, alias string) error {
	var err error
	domain, err = NormalizeTermDictionaryDomain(domain)
	if err != nil {
		return err
	}
	alias = strings.TrimSpace(strings.ToLower(alias))
	if domain == "" || alias == "" {
		return nil
	}
	return m.db.WithContext(ctx).
		Where("domain = ? AND alias = ?", domain, alias).
		Delete(&TermDictionary{}).Error
}
