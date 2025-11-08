package rbac

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AuthContext provides the context for evaluating authorization rules
type AuthContext struct {
	User        string         // Current user ID
	Permissions []string       // User's permissions
	Roles       []string       // User's roles
	Resource    map[string]any // Resource data being accessed
	Request     map[string]any // Request parameters
	Now         time.Time      // Current time
}

// EnhancedEvaluator provides advanced allow_if evaluation capabilities
type EnhancedEvaluator struct {
	policy *Policy
}

func NewEnhancedEvaluator(policy *Policy) *EnhancedEvaluator {
	return &EnhancedEvaluator{policy: policy}
}

// EvaluateAllowIf evaluates complex allow_if expressions
// Supports: has_permission(), has_role(), is_owner(), numeric comparisons, time windows
func (e *EnhancedEvaluator) EvaluateAllowIf(ctx context.Context, authCtx *AuthContext, expression string) bool {
	if expression == "" {
		return true // No restrictions
	}

	// Handle OR conditions
	orParts := strings.Split(expression, "||")
	for _, orPart := range orParts {
		orPart = strings.TrimSpace(orPart)
		if e.evaluateAndExpression(ctx, authCtx, orPart) {
			return true
		}
	}

	return false
}

func (e *EnhancedEvaluator) evaluateAndExpression(ctx context.Context, authCtx *AuthContext, expression string) bool {
	// Handle AND conditions
	andParts := strings.Split(expression, "&&")
	for _, andPart := range andParts {
		andPart = strings.TrimSpace(andPart)
		if !e.evaluateTerm(ctx, authCtx, andPart) {
			return false
		}
	}
	return true
}

func (e *EnhancedEvaluator) evaluateTerm(ctx context.Context, authCtx *AuthContext, term string) bool {
	term = strings.TrimSpace(term)

	// Handle negation
	if strings.HasPrefix(term, "!") {
		return !e.evaluateTerm(ctx, authCtx, strings.TrimSpace(term[1:]))
	}

	// Handle parentheses
	if strings.HasPrefix(term, "(") && strings.HasSuffix(term, ")") {
		return e.EvaluateAllowIf(ctx, authCtx, term[1:len(term)-1])
	}

	// Handle function calls
	if strings.Contains(term, "(") && strings.HasSuffix(term, ")") {
		return e.evaluateFunction(ctx, authCtx, term)
	}

	// Handle comparison operations
	if e.containsComparisonOperator(term) {
		return e.evaluateComparison(ctx, authCtx, term)
	}

	// Handle time window expressions
	if e.isTimeWindowExpression(term) {
		return e.evaluateTimeWindow(ctx, authCtx, term)
	}

	// Default to simple boolean check
	return e.evaluateSimpleExpression(ctx, authCtx, term)
}

func (e *EnhancedEvaluator) evaluateFunction(ctx context.Context, authCtx *AuthContext, funcCall string) bool {
	// Extract function name and arguments
	openParen := strings.Index(funcCall, "(")
	if openParen == -1 {
		return false
	}

	funcName := strings.TrimSpace(funcCall[:openParen])
	argsStr := strings.TrimSpace(funcCall[openParen+1 : len(funcCall)-1])

	var args []string
	if argsStr != "" {
		// Simple argument parsing (doesn't handle nested quotes/commas)
		args = strings.Split(argsStr, ",")
		for i, arg := range args {
			args[i] = strings.Trim(strings.TrimSpace(arg), "\"'")
		}
	}

	switch funcName {
	case "has_permission":
		return e.evaluateHasPermission(authCtx, args)
	case "has_role":
		return e.evaluateHasRole(authCtx, args)
	case "is_owner":
		return e.evaluateIsOwner(authCtx, args)
	case "time_between":
		return e.evaluateTimeBetween(authCtx, args)
	case "day_of_week":
		return e.evaluateDayOfWeek(authCtx, args)
	case "hour_between":
		return e.evaluateHourBetween(authCtx, args)
	default:
		return false
	}
}

func (e *EnhancedEvaluator) evaluateHasPermission(authCtx *AuthContext, args []string) bool {
	if len(args) != 1 {
		return false
	}
	permission := args[0]

	// Check direct permissions
	for _, perm := range authCtx.Permissions {
		if perm == permission || perm == "*" {
			return true
		}
	}

	// Check policy-based permissions
	return e.policy.Can(authCtx.User, permission)
}

func (e *EnhancedEvaluator) evaluateHasRole(authCtx *AuthContext, args []string) bool {
	if len(args) != 1 {
		return false
	}
	role := args[0]

	for _, userRole := range authCtx.Roles {
		if userRole == role {
			return true
		}
	}
	return false
}

func (e *EnhancedEvaluator) evaluateIsOwner(authCtx *AuthContext, args []string) bool {
	// is_owner() - check if user owns the resource
	// is_owner("field_name") - check if user matches specific field

	if len(args) == 0 {
		// Check common ownership fields
		ownerFields := []string{"owner", "owner_id", "user_id", "created_by"}
		for _, field := range ownerFields {
			if owner, exists := authCtx.Resource[field]; exists {
				if ownerStr, ok := owner.(string); ok && ownerStr == authCtx.User {
					return true
				}
			}
		}
		return false
	}

	if len(args) == 1 {
		fieldName := args[0]
		if owner, exists := authCtx.Resource[fieldName]; exists {
			if ownerStr, ok := owner.(string); ok && ownerStr == authCtx.User {
				return true
			}
		}
		return false
	}

	return false
}

func (e *EnhancedEvaluator) evaluateTimeBetween(authCtx *AuthContext, args []string) bool {
	if len(args) != 2 {
		return false
	}

	startTime, err1 := time.Parse("15:04", args[0])
	endTime, err2 := time.Parse("15:04", args[1])
	if err1 != nil || err2 != nil {
		return false
	}

	now := authCtx.Now
	currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)

	if startTime.Before(endTime) {
		return currentTime.After(startTime) && currentTime.Before(endTime)
	} else {
		// Spans midnight
		return currentTime.After(startTime) || currentTime.Before(endTime)
	}
}

func (e *EnhancedEvaluator) evaluateDayOfWeek(authCtx *AuthContext, args []string) bool {
	if len(args) == 0 {
		return false
	}

	currentDay := authCtx.Now.Weekday().String()
	currentDayShort := currentDay[:3] // First 3 characters

	for _, day := range args {
		if strings.EqualFold(day, currentDay) || strings.EqualFold(day, currentDayShort) {
			return true
		}
	}
	return false
}

func (e *EnhancedEvaluator) evaluateHourBetween(authCtx *AuthContext, args []string) bool {
	if len(args) != 2 {
		return false
	}

	startHour, err1 := strconv.Atoi(args[0])
	endHour, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil {
		return false
	}

	currentHour := authCtx.Now.Hour()
	if startHour <= endHour {
		return currentHour >= startHour && currentHour < endHour
	} else {
		// Spans midnight
		return currentHour >= startHour || currentHour < endHour
	}
}

func (e *EnhancedEvaluator) containsComparisonOperator(term string) bool {
	operators := []string{">=", "<=", "==", "!=", ">", "<"}
	for _, op := range operators {
		if strings.Contains(term, op) {
			return true
		}
	}
	return false
}

func (e *EnhancedEvaluator) evaluateComparison(ctx context.Context, authCtx *AuthContext, term string) bool {
	operators := []string{">=", "<=", "==", "!=", ">", "<"}

	for _, op := range operators {
		if strings.Contains(term, op) {
			parts := strings.SplitN(term, op, 2)
			if len(parts) != 2 {
				continue
			}

			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])

			leftVal := e.resolveValue(authCtx, left)
			rightVal := e.resolveValue(authCtx, right)

			return e.compareValues(leftVal, rightVal, op)
		}
	}
	return false
}

func (e *EnhancedEvaluator) resolveValue(authCtx *AuthContext, expr string) any {
	expr = strings.TrimSpace(expr)

	// Handle string literals
	if (strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"")) ||
		(strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'")) {
		return expr[1 : len(expr)-1]
	}

	// Handle numeric literals
	if val, err := strconv.ParseFloat(expr, 64); err == nil {
		return val
	}

	// Handle boolean literals
	if expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// Handle field references
	if strings.HasPrefix(expr, "resource.") {
		fieldName := expr[9:] // Remove "resource." prefix
		return authCtx.Resource[fieldName]
	}

	if strings.HasPrefix(expr, "request.") {
		fieldName := expr[8:] // Remove "request." prefix
		return authCtx.Request[fieldName]
	}

	// Handle special variables
	switch expr {
	case "user_id":
		return authCtx.User
	case "now":
		return authCtx.Now
	}

	return expr
}

func (e *EnhancedEvaluator) compareValues(left, right any, operator string) bool {
	switch operator {
	case "==":
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	case "!=":
		return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right)
	}

	// For numeric comparisons, try to convert to float64
	leftNum, leftOK := e.toFloat64(left)
	rightNum, rightOK := e.toFloat64(right)

	if leftOK && rightOK {
		switch operator {
		case ">":
			return leftNum > rightNum
		case ">=":
			return leftNum >= rightNum
		case "<":
			return leftNum < rightNum
		case "<=":
			return leftNum <= rightNum
		}
	}

	// For string comparisons
	leftStr := fmt.Sprintf("%v", left)
	rightStr := fmt.Sprintf("%v", right)

	switch operator {
	case ">":
		return leftStr > rightStr
	case ">=":
		return leftStr >= rightStr
	case "<":
		return leftStr < rightStr
	case "<=":
		return leftStr <= rightStr
	}

	return false
}

func (e *EnhancedEvaluator) toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (e *EnhancedEvaluator) isTimeWindowExpression(term string) bool {
	// Simple time window patterns
	timePatterns := []string{
		`\d{1,2}:\d{2}-\d{1,2}:\d{2}`,   // 09:00-17:00
		`(Mon|Tue|Wed|Thu|Fri|Sat|Sun)`, // Day of week
	}

	for _, pattern := range timePatterns {
		if matched, _ := regexp.MatchString(pattern, term); matched {
			return true
		}
	}
	return false
}

func (e *EnhancedEvaluator) evaluateTimeWindow(ctx context.Context, authCtx *AuthContext, term string) bool {
	// Handle time range pattern (e.g., "09:00-17:00")
	if matched, _ := regexp.MatchString(`\d{1,2}:\d{2}-\d{1,2}:\d{2}`, term); matched {
		parts := strings.Split(term, "-")
		if len(parts) == 2 {
			return e.evaluateTimeBetween(authCtx, parts)
		}
	}

	// Handle day of week
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for _, day := range days {
		if strings.EqualFold(term, day) || strings.EqualFold(term, day[:3]) {
			return e.evaluateDayOfWeek(authCtx, []string{day})
		}
	}

	return false
}

func (e *EnhancedEvaluator) evaluateSimpleExpression(ctx context.Context, authCtx *AuthContext, expr string) bool {
	// Handle simple field references
	if val := e.resolveValue(authCtx, expr); val != nil {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return v != "" && v != "false"
		case float64:
			return v != 0
		default:
			return fmt.Sprintf("%v", v) != ""
		}
	}
	return false
}
