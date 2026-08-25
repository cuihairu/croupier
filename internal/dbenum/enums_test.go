package dbenum

import (
	"database/sql/driver"
	"encoding/json"
	"testing"
)

// Enum contract tests covering every platform enum: String/Parse round-trips,
// Value/Scan across every driver source shape (including legacy rows), and
// JSON marshal/unmarshal including invalid inputs.

func TestScanInt_Sources(t *testing.T) {
	cases := []struct {
		src    any
		want   int
		wantEr bool
	}{
		{nil, 0, false},
		{int64(3), 3, false},
		{int(4), 4, false},
		{float64(5), 5, false},  // SQLite drivers deliver ints as float
		{[]byte("6"), 6, false}, // legacy string rows
		{[]byte(""), 0, false},  // empty legacy cell
		{"7", 7, false},         // legacy string rows
		{"", 0, false},          // empty legacy cell
		{"abc", 0, true},        // invalid bytes
		{[]byte("x"), 0, true},  // invalid string
		{struct{}{}, 0, true},   // unsupported source
	}
	for _, tc := range cases {
		var got int
		err := ScanInt(&got, tc.src)
		if tc.wantEr {
			if err == nil {
				t.Errorf("ScanInt(%v) expected error", tc.src)
			}
			continue
		}
		if err != nil {
			t.Errorf("ScanInt(%v): %v", tc.src, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ScanInt(%v) = %d, want %d", tc.src, got, tc.want)
		}
	}
}

func TestValueInt(t *testing.T) {
	v, err := ValueInt(9)
	if err != nil || v != int64(9) {
		t.Fatalf("ValueInt(9) = %v, %v", v, err)
	}
}

// roundTrip exercises Value→Scan→String and JSON for one enum value.
// E only needs the value-receiver methods; Scan is called through the typed
// pointer (every enum implements Scan on the pointer receiver).
func roundTrip[E interface {
	String() string
	Value() (driver.Value, error)
	json.Marshaler
}](t *testing.T, ptr *E, wire string) {
	t.Helper()
	e := *ptr
	if got := e.String(); got != wire {
		t.Errorf("String() = %q, want %q", got, wire)
	}
	val, err := e.Value()
	if err != nil {
		t.Fatalf("Value(%s): %v", wire, err)
	}
	scanner, _ := any(ptr).(interface{ Scan(any) error })
	if scanner == nil {
		t.Fatalf("type %T missing Scan", ptr)
	}
	if err := scanner.Scan(val); err != nil {
		t.Fatalf("Scan(%s): %v", wire, err)
	}
	if got := (*ptr).String(); got != wire {
		t.Errorf("after Scan String() = %q, want %q", got, wire)
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("MarshalJSON(%s): %v", wire, err)
	}
	if string(data) != `"`+wire+`"` {
		t.Fatalf("MarshalJSON = %s, want %q", data, wire)
	}
}

func TestCapabilityEnum(t *testing.T) {
	all := map[Capability]string{
		CapabilityUnknown:         "unknown",
		CapabilityCollectionQuery: "collection_query",
		CapabilityItemQuery:       "item_query",
		CapabilityCreate:          "create",
		CapabilityUpdate:          "update",
		CapabilityDelete:          "delete",
		CapabilityAction:          "action",
		CapabilityTask:            "task",
		CapabilityReport:          "report",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseCapability(wire); err != nil || got != e {
			t.Errorf("ParseCapability(%q) = %v, %v; want %v", wire, got, err, e)
		}
	}
	if got := Capability(99).String(); got != "unknown" {
		t.Errorf("Capability(99).String() = %q", got)
	}
	if _, err := ParseCapability("nope"); err == nil {
		t.Error("ParseCapability(invalid) expected error")
	}
	// Empty string is rejected for capability (unlike status enums whose
	// empty value is the zero state).
	if _, err := ParseCapability(""); err == nil {
		t.Error(`ParseCapability("") expected error`)
	}
	// Unmarshal error branches.
	var c Capability
	if err := c.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := c.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
	// Legacy string sources scan into the enum.
	var fromStr Capability
	if err := fromStr.Scan("2"); err != nil || fromStr != CapabilityItemQuery {
		t.Errorf(`Capability.Scan("2") = %v, %v`, fromStr, err)
	}
}

func TestProposalStatusEnum(t *testing.T) {
	all := map[ProposalStatus]string{
		ProposalStatusPending:  "pending",
		ProposalStatusAccepted: "accepted",
		ProposalStatusRejected: "rejected",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseProposalStatus(wire); err != nil || got != e {
			t.Errorf("ParseProposalStatus(%q) = %v, %v", wire, got, err)
		}
	}
	// Legacy alias "expired" maps to pending.
	if got, err := ParseProposalStatus("expired"); err != nil || got != ProposalStatusPending {
		t.Errorf(`ParseProposalStatus("expired") = %v, %v`, got, err)
	}
	if got := ProposalStatus(7).String(); got != "pending" {
		t.Errorf("ProposalStatus(7).String() = %q", got)
	}
	if _, err := ParseProposalStatus("nope"); err == nil {
		t.Error("ParseProposalStatus(invalid) expected error")
	}
	var p ProposalStatus
	if err := p.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := p.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestTicketStatusEnum(t *testing.T) {
	all := map[TicketStatus]string{
		TicketStatusOpen:       "open",
		TicketStatusInProgress: "in_progress",
		TicketStatusResolved:   "resolved",
		TicketStatusClosed:     "closed",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseTicketStatus(wire); err != nil || got != e {
			t.Errorf("ParseTicketStatus(%q) = %v, %v", wire, got, err)
		}
	}
	if got := TicketStatus(9).String(); got != "open" {
		t.Errorf("TicketStatus(9).String() = %q", got)
	}
	if _, err := ParseTicketStatus("nope"); err == nil {
		t.Error("ParseTicketStatus(invalid) expected error")
	}
	var ts TicketStatus
	if err := ts.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := ts.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestMessageStatusEnum(t *testing.T) {
	all := map[MessageStatus]string{
		MessageStatusUnread: "unread",
		MessageStatusRead:   "read",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseMessageStatus(wire); err != nil || got != e {
			t.Errorf("ParseMessageStatus(%q) = %v, %v", wire, got, err)
		}
	}
	if got := MessageStatus(5).String(); got != "unread" {
		t.Errorf("MessageStatus(5).String() = %q", got)
	}
	if _, err := ParseMessageStatus("nope"); err == nil {
		t.Error("ParseMessageStatus(invalid) expected error")
	}
	var m MessageStatus
	if err := m.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := m.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestFeedbackStatusEnum(t *testing.T) {
	all := map[FeedbackStatus]string{
		FeedbackStatusOpen:    "open",
		FeedbackStatusTriaged: "triaged",
		FeedbackStatusClosed:  "closed",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseFeedbackStatus(wire); err != nil || got != e {
			t.Errorf("ParseFeedbackStatus(%q) = %v, %v", wire, got, err)
		}
	}
	// Legacy alias "new" maps to open.
	if got, err := ParseFeedbackStatus("new"); err != nil || got != FeedbackStatusOpen {
		t.Errorf(`ParseFeedbackStatus("new") = %v, %v`, got, err)
	}
	if got := FeedbackStatus(9).String(); got != "open" {
		t.Errorf("FeedbackStatus(9).String() = %q", got)
	}
	if _, err := ParseFeedbackStatus("nope"); err == nil {
		t.Error("ParseFeedbackStatus(invalid) expected error")
	}
	var f FeedbackStatus
	if err := f.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := f.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestExtensionInstallationStatusEnum(t *testing.T) {
	all := map[ExtensionInstallationStatus]string{
		ExtensionInstallationPending:  "pending",
		ExtensionInstallationEnabled:  "enabled",
		ExtensionInstallationDisabled: "disabled",
		ExtensionInstallationError:    "error",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseExtensionInstallationStatus(wire); err != nil || got != e {
			t.Errorf("ParseExtensionInstallationStatus(%q) = %v, %v", wire, got, err)
		}
	}
	if got := ExtensionInstallationStatus(9).String(); got != "pending" {
		t.Errorf("ExtensionInstallationStatus(9).String() = %q", got)
	}
	if _, err := ParseExtensionInstallationStatus("nope"); err == nil {
		t.Error("ParseExtensionInstallationStatus(invalid) expected error")
	}
	var e ExtensionInstallationStatus
	if err := e.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := e.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestCertificateStatusEnum(t *testing.T) {
	all := map[CertificateStatus]string{
		CertificateStatusActive:   "active",
		CertificateStatusExpiring: "expiring",
		CertificateStatusExpired:  "expired",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseCertificateStatus(wire); err != nil || got != e {
			t.Errorf("ParseCertificateStatus(%q) = %v, %v", wire, got, err)
		}
	}
	if got := CertificateStatus(9).String(); got != "active" {
		t.Errorf("CertificateStatus(9).String() = %q", got)
	}
	if _, err := ParseCertificateStatus("nope"); err == nil {
		t.Error("ParseCertificateStatus(invalid) expected error")
	}
	var c CertificateStatus
	if err := c.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := c.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}

func TestRiskEnum(t *testing.T) {
	all := map[Risk]string{
		RiskSafe:    "safe",
		RiskWarning: "warning",
		RiskHigh:    "high",
		RiskDanger:  "danger",
	}
	for e, wire := range all {
		roundTrip(t, &e, wire)
		if got, err := ParseRisk(wire); err != nil || got != e {
			t.Errorf("ParseRisk(%q) = %v, %v", wire, got, err)
		}
	}
	// Legacy aliases.
	aliases := map[string]Risk{
		"":         RiskSafe,
		"low":      RiskSafe,
		"medium":   RiskWarning,
		"critical": RiskDanger,
	}
	for wire, want := range aliases {
		if got, err := ParseRisk(wire); err != nil || got != want {
			t.Errorf("ParseRisk(%q) = %v, %v; want %v", wire, got, err, want)
		}
	}
	if got := Risk(9).String(); got != "safe" {
		t.Errorf("Risk(9).String() = %q", got)
	}
	if _, err := ParseRisk("nope"); err == nil {
		t.Error("ParseRisk(invalid) expected error")
	}
	var r Risk
	if err := r.UnmarshalJSON([]byte(`"nope"`)); err == nil {
		t.Error(`UnmarshalJSON("nope") expected error`)
	}
	if err := r.UnmarshalJSON([]byte(`{`)); err == nil {
		t.Error("UnmarshalJSON(bad json) expected error")
	}
}
