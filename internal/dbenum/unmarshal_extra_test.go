package dbenum

import (
	"encoding/json"
	"testing"
)

// 每个 UnmarshalJSON 的「非法枚举名」错误分支（Parse 失败 → return err）。
func TestUnmarshalJSONInvalidNames(t *testing.T) {
	var capEnum Capability
	var proposal ProposalStatus
	var ticket TicketStatus
	var message MessageStatus
	var feedback FeedbackStatus
	var install ExtensionInstallationStatus
	var cert CertificateStatus
	var risk Risk

	cases := []struct {
		name string
		raw  string
		into any
	}{
		{"capability", `"no_such_capability"`, &capEnum},
		{"proposalStatus", `"no_such_status"`, &proposal},
		{"ticketStatus", `"no_such_status"`, &ticket},
		{"messageStatus", `"no_such_status"`, &message},
		{"feedbackStatus", `"no_such_status"`, &feedback},
		{"extensionInstallationStatus", `"no_such_status"`, &install},
		{"certificateStatus", `"no_such_status"`, &cert},
		{"risk", `"no_such_risk"`, &risk},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tc.raw), tc.into); err == nil {
				t.Fatalf("%s UnmarshalJSON(%s) should fail", tc.name, tc.raw)
			}
		})
	}
}

// 合法名走完 UnmarshalJSON 的赋值路径。
func TestUnmarshalJSONValidNames(t *testing.T) {
	var capEnum Capability
	var proposal ProposalStatus
	var ticket TicketStatus
	var message MessageStatus
	var feedback FeedbackStatus
	var install ExtensionInstallationStatus
	var cert CertificateStatus
	var risk Risk

	mustOK := func(t *testing.T, raw string, into any) {
		t.Helper()
		if err := json.Unmarshal([]byte(raw), into); err != nil {
			t.Fatalf("Unmarshal(%s) error = %v", raw, err)
		}
	}
	mustOK(t, `"collection_query"`, &capEnum)
	mustOK(t, `"pending"`, &proposal)
	mustOK(t, `"open"`, &ticket)
	mustOK(t, `"unread"`, &message)
	mustOK(t, `"open"`, &feedback)
	mustOK(t, `"pending"`, &install)
	mustOK(t, `"active"`, &cert)
	mustOK(t, `"medium"`, &risk)

	if capEnum != CapabilityCollectionQuery || proposal != ProposalStatusPending {
		t.Fatal("enum assignment after unmarshal mismatched")
	}
}
