package model

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FunctionModel wraps data access for functions and related tables.
type FunctionModel struct {
	db *gorm.DB
}

// NewFunctionModel creates the helper.
func NewFunctionModel(db *gorm.DB) *FunctionModel {
	return &FunctionModel{db: db}
}

// ListFunctionsOptions controls listing.
type ListFunctionsOptions struct {
	PaginationOptions
	GameID   string
	Category string
	Status   *int
	Search   string
}

// Create inserts a new function record.
func (m *FunctionModel) Create(ctx context.Context, fn *Function) error {
	return m.db.WithContext(ctx).Create(fn).Error
}

// Update modifies function fields.
func (m *FunctionModel) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	normalizeJSONMapUpdate(updates, "metadata")
	normalizeJSONMapUpdate(updates, "schema")
	normalizeJSONMapUpdate(updates, "open_api_spec")
	return m.db.WithContext(ctx).Model(&Function{}).Where("id = ?", id).Updates(updates).Error
}

// Delete removes a function.
func (m *FunctionModel) Delete(ctx context.Context, id uint) error {
	return m.db.WithContext(ctx).Delete(&Function{}, id).Error
}

// FindByID fetches a function.
func (m *FunctionModel) FindByID(ctx context.Context, id uint) (*Function, error) {
	var fn Function
	if err := m.db.WithContext(ctx).First(&fn, id).Error; err != nil {
		return nil, err
	}
	return &fn, nil
}

// FindByFunctionID fetches by external function ID.
func (m *FunctionModel) FindByFunctionID(ctx context.Context, functionID string) (*Function, error) {
	var fn Function
	if err := m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		First(&fn).Error; err != nil {
		return nil, err
	}
	return &fn, nil
}

// ListFunctionMenus returns function_id -> metadata.menu map.
func (m *FunctionModel) ListFunctionMenus(ctx context.Context) (map[string]map[string]interface{}, error) {
	type row struct {
		FunctionID string
		Metadata   datatypes.JSONMap
	}
	var rows []row
	if err := m.db.WithContext(ctx).Model(&Function{}).Select("function_id", "metadata").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]map[string]interface{}, len(rows))
	for _, r := range rows {
		if r.FunctionID == "" || r.Metadata == nil {
			continue
		}
		raw, ok := r.Metadata["menu"]
		if !ok {
			continue
		}
		mm, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		out[r.FunctionID] = mm
	}
	return out, nil
}

// List returns paginated functions.
func (m *FunctionModel) List(ctx context.Context, opts ListFunctionsOptions) ([]Function, int64, error) {
	opts.PaginationOptions.Normalize()

	var (
		items []Function
		total int64
	)

	query := m.db.WithContext(ctx).Model(&Function{})
	if opts.GameID != "" {
		query = query.Where("game_id = ?", opts.GameID)
	}
	if opts.Category != "" {
		query = query.Where("category = ?", opts.Category)
	}
	if opts.Status != nil {
		query = query.Where("status = ?", *opts.Status)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("updated_at DESC").
		Offset(opts.Offset()).
		Limit(opts.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// UpsertDescriptor stores descriptor metadata.
func (m *FunctionModel) UpsertDescriptor(ctx context.Context, desc *FunctionDescriptor) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(desc).Error
}

// ListDescriptors fetches descriptors for a function.
func (m *FunctionModel) ListDescriptors(ctx context.Context, functionID string) ([]FunctionDescriptor, error) {
	var descs []FunctionDescriptor
	err := m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Order("version DESC").
		Find(&descs).Error
	return descs, err
}

// ListDescriptorTemplates returns reusable descriptors filtered by category.
func (m *FunctionModel) ListDescriptorTemplates(ctx context.Context, category string) ([]Descriptor, error) {
	query := m.db.WithContext(ctx).Model(&Descriptor{})
	if category != "" {
		query = query.Where("category = ?", category)
	}
	var descs []Descriptor
	if err := query.Order("updated_at DESC").Find(&descs).Error; err != nil {
		return nil, err
	}
	return descs, nil
}

// RegisterInstance upserts instance heartbeat data.
func (m *FunctionModel) RegisterInstance(ctx context.Context, instance *FunctionInstance) error {
	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(instance).Error
}

// ListInstances returns function instances.
func (m *FunctionModel) ListInstances(ctx context.Context, functionID string) ([]FunctionInstance, error) {
	var instances []FunctionInstance
	err := m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Order("updated_at DESC").
		Find(&instances).Error
	return instances, err
}

// ReplacePermissions fully replaces permissions for a function.
func (m *FunctionModel) ReplacePermissions(ctx context.Context, functionID string, perms []FunctionPermission) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("function_id = ?", functionID).Delete(&FunctionPermission{}).Error; err != nil {
			return err
		}
		for i := range perms {
			perms[i].FunctionID = functionID
			if err := tx.Create(&perms[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListPermissions returns function permissions.
func (m *FunctionModel) ListPermissions(ctx context.Context, functionID string) ([]FunctionPermission, error) {
	var perms []FunctionPermission
	err := m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Find(&perms).Error
	return perms, err
}

// SavePendingFunction upserts pending change set.
func (m *FunctionModel) SavePendingFunction(ctx context.Context, pending *PendingFunction) error {
	if pending.FunctionID == "" {
		return errors.New("function id required")
	}

	return m.db.WithContext(ctx).
		Clauses(upsertAllColumns()).
		Create(pending).Error
}

// DeletePending removes a pending function.
func (m *FunctionModel) DeletePending(ctx context.Context, functionID string) error {
	return m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Delete(&PendingFunction{}).Error
}

// ListPending returns pending submissions.
func (m *FunctionModel) ListPending(ctx context.Context) ([]PendingFunction, error) {
	var pending []PendingFunction
	err := m.db.WithContext(ctx).
		Order("updated_at DESC").
		Find(&pending).Error
	return pending, err
}

// DeleteFunction deletes a function by function_id
func (m *FunctionModel) DeleteFunction(ctx context.Context, functionID string) error {
	return m.db.WithContext(ctx).
		Where("function_id = ?", functionID).
		Delete(&Function{}).Error
}

// CopyFunction creates a copy of a function with a new ID
func (m *FunctionModel) CopyFunction(ctx context.Context, functionID string) (string, error) {
	// Find the original function
	var fn Function
	if err := m.db.WithContext(ctx).Where("function_id = ?", functionID).First(&fn).Error; err != nil {
		return "", err
	}

	// Create new function with suffixed ID
	newID := functionID + "_copy_" + time.Now().Format("20060102150405")
	fn.FunctionID = newID
	fn.ID = 0 // Reset ID to let DB generate new one

	if err := m.Create(ctx, &fn); err != nil {
		return "", err
	}

	return newID, nil
}

// BatchUpdateStatus updates status for multiple functions
func (m *FunctionModel) BatchUpdateStatus(ctx context.Context, functionIDs []string, enabled bool) (int, []string, error) {
	if len(functionIDs) == 0 {
		return 0, nil, nil
	}

	result := m.db.WithContext(ctx).
		Model(&Function{}).
		Where("function_id IN ?", functionIDs).
		Updates(map[string]interface{}{"enabled": enabled})

	if result.Error != nil {
		return 0, nil, result.Error
	}

	return int(result.RowsAffected), nil, nil
}

// BatchDeleteFunctions deletes multiple functions
func (m *FunctionModel) BatchDeleteFunctions(ctx context.Context, functionIDs []string) (int, []string, error) {
	if len(functionIDs) == 0 {
		return 0, nil, nil
	}

	result := m.db.WithContext(ctx).
		Where("function_id IN ?", functionIDs).
		Delete(&Function{})

	if result.Error != nil {
		return 0, nil, result.Error
	}

	return int(result.RowsAffected), nil, nil
}

// BatchCopyFunctions copies multiple functions
func (m *FunctionModel) BatchCopyFunctions(ctx context.Context, functionIDs []string) (int, []string, []string, error) {
	if len(functionIDs) == 0 {
		return 0, nil, nil, nil
	}

	var copiedIDs []string
	var failedIDs []string

	for _, id := range functionIDs {
		newID, err := m.CopyFunction(ctx, id)
		if err != nil {
			failedIDs = append(failedIDs, id)
		} else {
			copiedIDs = append(copiedIDs, newID)
		}
	}

	return len(copiedIDs), failedIDs, copiedIDs, nil
}

func normalizeJSONMapUpdate(updates map[string]interface{}, key string) {
	if updates == nil {
		return
	}
	raw, ok := updates[key]
	if !ok || raw == nil {
		return
	}
	switch v := raw.(type) {
	case datatypes.JSONMap:
		return
	case map[string]interface{}:
		updates[key] = datatypes.JSONMap(v)
	case []byte:
		var decoded map[string]interface{}
		if err := json.Unmarshal(v, &decoded); err == nil {
			updates[key] = datatypes.JSONMap(decoded)
		}
	case string:
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			updates[key] = datatypes.JSONMap(decoded)
		}
	}
}
