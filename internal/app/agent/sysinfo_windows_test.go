//go:build windows

package agent

import (
	"strings"
	"testing"
)

// checkServiceManagerAccess tests if we can access the service manager.
// Returns true if access is granted, false if access is denied.
func checkServiceManagerAccess() bool {
	_, err := listServicesPlatform("", "", 1)
	return err == nil || !strings.Contains(err.Error(), "Access is denied")
}

// TestListServicesPlatform_Basic tests basic service listing.
func TestListServicesPlatform_Basic(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	services, err := listServicesPlatform("", "", 100)
	if err != nil {
		t.Fatalf("listServicesPlatform failed: %v", err)
	}

	if len(services) == 0 {
		t.Fatal("expected at least some services to be returned")
	}

	// Verify first service has required fields
	first := services[0]
	if first.Name == "" {
		t.Error("service name should not be empty")
	}
	if first.Status == "" {
		t.Error("service status should not be empty")
	}
	if first.StartType == "" {
		t.Error("service start type should not be empty")
	}

	t.Logf("Found %d services, first: %s (%s)", len(services), first.Name, first.Status)
}

// TestListServicesPlatform_StateFilter tests filtering by service state.
func TestListServicesPlatform_StateFilter(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	tests := []struct {
		name        string
		state       string
		expectEmpty bool
	}{
		{"Running services", "running", false},
		{"Stopped services", "stopped", false},
		{"Invalid state", "invalid_state", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := listServicesPlatform(tt.state, "", 100)
			if err != nil {
				t.Fatalf("listServicesPlatform failed: %v", err)
			}

			if tt.expectEmpty && len(services) > 0 {
				t.Errorf("expected no services for state %q, got %d", tt.state, len(services))
			}

			if !tt.expectEmpty && len(services) == 0 {
				t.Errorf("expected some services for state %q", tt.state)
			}

			// Verify all returned services match the filter (if valid state)
			if !tt.expectEmpty && tt.state != "" {
				for _, s := range services {
					if !strings.EqualFold(s.Status, tt.state) {
						t.Errorf("service %s has status %q, expected %q", s.Name, s.Status, tt.state)
					}
				}
			}

			t.Logf("Found %d services for state %q", len(services), tt.state)
		})
	}
}

// TestListServicesPlatform_NamePattern tests filtering by service name pattern.
func TestListServicesPlatform_NamePattern(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	tests := []struct {
		name        string
		pattern     string
		expectMatch bool
	}{
		{"EventLog pattern", "event", true},
		{"Schedule pattern", "schedule", true},
		{"Pattern uppercase", "EVENTLOG", true},
		{"Pattern lowercase", "eventlog", true},
		{"Non-existent pattern", "xyznonexistent123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := listServicesPlatform("", tt.pattern, 100)
			if err != nil {
				t.Fatalf("listServicesPlatform failed: %v", err)
			}

			if !tt.expectMatch && len(services) > 0 {
				t.Errorf("expected no services for pattern %q, got %d", tt.pattern, len(services))
			}

			if tt.expectMatch && len(services) == 0 {
				t.Errorf("expected some services for pattern %q", tt.pattern)
			}

			// Verify all returned services match the pattern
			for _, s := range services {
				if !strings.Contains(strings.ToLower(s.Name), strings.ToLower(tt.pattern)) {
					t.Errorf("service %s does not match pattern %q", s.Name, tt.pattern)
				}
			}

			t.Logf("Found %d services for pattern %q", len(services), tt.pattern)
		})
	}
}

// TestListServicesPlatform_Limit tests result limiting.
func TestListServicesPlatform_Limit(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	limits := []int{1, 5, 10}

	for _, limit := range limits {
		t.Run(t.Name(), func(t *testing.T) {
			services, err := listServicesPlatform("", "", limit)
			if err != nil {
				t.Fatalf("listServicesPlatform failed: %v", err)
			}

			if len(services) > limit {
				t.Errorf("expected at most %d services, got %d", limit, len(services))
			}

			t.Logf("Limit %d returned %d services", limit, len(services))
		})
	}
}

// TestGetServiceStatusPlatform_ExistingService tests getting status of existing services.
func TestGetServiceStatusPlatform_ExistingService(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	// Use well-known Windows services that should exist on most systems
	testServices := []string{"EventLog", "Schedule"}

	for _, svcName := range testServices {
		t.Run(svcName, func(t *testing.T) {
			status, err := getServiceStatusPlatform(svcName)
			if err != nil {
				t.Logf("warning: service %q not found (may not exist on this system): %v", svcName, err)
				return
			}

			// Verify all fields are populated
			if status.Name == "" {
				t.Error("service name should not be empty")
			}
			if status.DisplayName == "" {
				t.Error("display name should not be empty")
			}
			if status.Status == "" {
				t.Error("status should not be empty")
			}
			if status.StartType == "" {
				t.Error("start type should not be empty")
			}

			t.Logf("Service %s: %s (%s), binary: %s",
				status.Name, status.DisplayName, status.Status, status.BinaryPath)
		})
	}
}

// TestGetServiceStatusPlatform_NonExistingService tests error handling for non-existent service.
func TestGetServiceStatusPlatform_NonExistingService(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	_, err := getServiceStatusPlatform("NonExistentService12345")
	if err == nil {
		t.Error("expected error for non-existent service, got nil")
	}
	t.Logf("Correctly got error for non-existent service: %v", err)
}

// TestListServices tests the public ListServices function.
func TestListServices(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	services, err := ListServices("", "", 10)
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}

	if len(services) == 0 {
		t.Fatal("expected at least some services")
	}

	t.Logf("ListServices returned %d services", len(services))
}

// TestGetServiceStatus tests the public GetServiceStatus function.
func TestGetServiceStatus(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	// Try EventLog service which should exist on most Windows systems
	status, err := GetServiceStatus("EventLog")
	if err != nil {
		t.Logf("warning: EventLog service not found: %v", err)
		return
	}

	if status.Name != "EventLog" {
		t.Errorf("expected service name 'EventLog', got %q", status.Name)
	}

	t.Logf("GetServiceStatus(EventLog): %+v", status)
}

// TestListCronJobs tests that cron jobs return an error on Windows.
func TestListCronJobs(t *testing.T) {
	_, err := ListCronJobs()
	if err == nil {
		t.Error("expected error for ListCronJobs on Windows, got nil")
	}
	if !strings.Contains(err.Error(), "not available") {
		t.Errorf("expected 'not available' error, got: %v", err)
	}
}

// TestServiceInfoFields verifies ServiceInfo struct fields.
func TestServiceInfoFields(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	services, err := listServicesPlatform("", "", 1)
	if err != nil {
		t.Fatalf("listServicesPlatform failed: %v", err)
	}

	if len(services) == 0 {
		t.Skip("no services to test")
	}

	s := services[0]

	// Check JSON tags work correctly
	if s.Name == "" && s.DisplayName == "" {
		t.Error("at least Name or DisplayName should be set")
	}

	// Status should be one of the known values
	validStatuses := map[string]bool{
		"running":       true,
		"stopped":       true,
		"paused":        true,
		"start_pending": true,
		"stop_pending":  true,
		"unknown":       true,
	}
	if !validStatuses[s.Status] {
		t.Errorf("invalid status value: %q", s.Status)
	}

	// StartType should be one of the known values
	validStartTypes := map[string]bool{
		"auto":     true,
		"manual":   true,
		"disabled": true,
		"unknown":  true,
	}
	if !validStartTypes[s.StartType] {
		t.Errorf("invalid start type value: %q", s.StartType)
	}

	t.Logf("Service info validated: %s (%s) - %s / %s",
		s.Name, s.DisplayName, s.Status, s.StartType)
}

// TestStateToString tests state string conversion.
func TestStateToString(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	knownValues := map[string]bool{
		"running": true, "stopped": true, "paused": true,
		"start_pending": true, "stop_pending": true, "unknown": true,
	}

	// Get a real service and check its status is valid
	services, err := listServicesPlatform("", "", 1)
	if err != nil {
		t.Fatalf("listServicesPlatform failed: %v", err)
	}

	if len(services) > 0 {
		status := services[0].Status
		if knownValues[status] {
			t.Logf("Status %q is a known value", status)
		} else {
			t.Errorf("Status %q is not a known value", status)
		}
	}
}

// TestStartTypeToString tests start type string conversion.
func TestStartTypeToString(t *testing.T) {
	if !checkServiceManagerAccess() {
		t.Skip("Skipping test: Service manager access denied (requires admin privileges)")
	}

	// Get a real service and check its start type is valid
	services, err := listServicesPlatform("", "", 1)
	if err != nil {
		t.Fatalf("listServicesPlatform failed: %v", err)
	}

	if len(services) > 0 {
		startType := services[0].StartType
		knownValues := map[string]bool{
			"auto": true, "manual": true, "disabled": true, "unknown": true,
		}
		if knownValues[startType] {
			t.Logf("StartType %q is a known value", startType)
		} else {
			t.Errorf("StartType %q is not a known value", startType)
		}
	}
}
