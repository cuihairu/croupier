package function

import (
	"github.com/cuihairu/croupier/internal/logic/utils"
)

// Convert from utils.Function to function.Function
func convertFromUtilsFunction(u utils.Function) Function {
	return Function{
		ID:          u.Id,
		Name:        u.Name,
		Description: u.Description,
		Category:    u.Category,
		GameId:      u.GameId,
		Status:      u.Status,
		Version:     u.Version,
		Instances:   u.Instances,
		SpecFormat:  u.SpecFormat,
		OpenAPISpec: u.OpenAPISpec,
	}
}

// Convert slice from utils.Function to function.Function
func convertFromUtilsFunctionSlice(funcs []utils.Function) []Function {
	result := make([]Function, len(funcs))
	for i, f := range funcs {
		result[i] = convertFromUtilsFunction(f)
	}
	return result
}

// Convert from utils.FunctionPermission to function.FunctionPermission
func convertFromUtilsPermission(u utils.FunctionPermission) FunctionPermission {
	return FunctionPermission{
		Resource: u.Resource,
		Actions:  u.Actions,
		Roles:    u.Roles,
	}
}

// Convert from function.FunctionPermission to utils.FunctionPermission
func convertToUtilsPermission(f FunctionPermission) utils.FunctionPermission {
	return utils.FunctionPermission{
		Resource: f.Resource,
		Actions:  f.Actions,
		Roles:    f.Roles,
	}
}

// Convert slice from utils.FunctionPermission to function.FunctionPermission
func convertFromUtilsPermissions(perms []utils.FunctionPermission) []FunctionPermission {
	result := make([]FunctionPermission, len(perms))
	for i, p := range perms {
		result[i] = convertFromUtilsPermission(p)
	}
	return result
}

// Convert slice from function.FunctionPermission to utils.FunctionPermission
func convertToUtilsPermissions(perms []FunctionPermission) []utils.FunctionPermission {
	result := make([]utils.FunctionPermission, len(perms))
	for i, p := range perms {
		result[i] = convertToUtilsPermission(p)
	}
	return result
}
