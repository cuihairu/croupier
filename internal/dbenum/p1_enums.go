package dbenum

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// MessageStatus enumerates Message.status (unread|read).
type MessageStatus int

const (
	MessageStatusUnread MessageStatus = iota
	MessageStatusRead
)

func (m MessageStatus) String() string {
	if m == MessageStatusRead {
		return "read"
	}
	return "unread"
}

func (m MessageStatus) Value() (driver.Value, error) { return ValueInt(int(m)) }

func (m *MessageStatus) Scan(src any) error { return ScanInt((*int)(m), src) }

func (m MessageStatus) MarshalJSON() ([]byte, error) { return json.Marshal(m.String()) }

func (m *MessageStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseMessageStatus(name)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

// ParseMessageStatus converts a wire message status into the enum.
func ParseMessageStatus(name string) (MessageStatus, error) {
	switch name {
	case "", "unread":
		return MessageStatusUnread, nil
	case "read":
		return MessageStatusRead, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown message status %q", name)
	}
}

// FeedbackStatus enumerates Feedback.status (open|triaged|closed).
type FeedbackStatus int

const (
	FeedbackStatusOpen FeedbackStatus = iota
	FeedbackStatusTriaged
	FeedbackStatusClosed
)

func (f FeedbackStatus) String() string {
	switch f {
	case FeedbackStatusTriaged:
		return "triaged"
	case FeedbackStatusClosed:
		return "closed"
	default:
		return "open"
	}
}

func (f FeedbackStatus) Value() (driver.Value, error) { return ValueInt(int(f)) }

func (f *FeedbackStatus) Scan(src any) error { return ScanInt((*int)(f), src) }

func (f FeedbackStatus) MarshalJSON() ([]byte, error) { return json.Marshal(f.String()) }

func (f *FeedbackStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseFeedbackStatus(name)
	if err != nil {
		return err
	}
	*f = parsed
	return nil
}

// ParseFeedbackStatus converts a wire feedback status into the enum. The
// legacy "new" value maps to open.
func ParseFeedbackStatus(name string) (FeedbackStatus, error) {
	switch name {
	case "", "open", "new":
		return FeedbackStatusOpen, nil
	case "triaged":
		return FeedbackStatusTriaged, nil
	case "closed":
		return FeedbackStatusClosed, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown feedback status %q", name)
	}
}

// ExtensionInstallationStatus enumerates extension installation state
// (pending|enabled|disabled|error).
type ExtensionInstallationStatus int

const (
	ExtensionInstallationPending ExtensionInstallationStatus = iota
	ExtensionInstallationEnabled
	ExtensionInstallationDisabled
	ExtensionInstallationError
)

func (e ExtensionInstallationStatus) String() string {
	switch e {
	case ExtensionInstallationEnabled:
		return "enabled"
	case ExtensionInstallationDisabled:
		return "disabled"
	case ExtensionInstallationError:
		return "error"
	default:
		return "pending"
	}
}

func (e ExtensionInstallationStatus) Value() (driver.Value, error) { return ValueInt(int(e)) }

func (e *ExtensionInstallationStatus) Scan(src any) error { return ScanInt((*int)(e), src) }

func (e ExtensionInstallationStatus) MarshalJSON() ([]byte, error) { return json.Marshal(e.String()) }

func (e *ExtensionInstallationStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseExtensionInstallationStatus(name)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

// ParseExtensionInstallationStatus converts a wire status into the enum.
func ParseExtensionInstallationStatus(name string) (ExtensionInstallationStatus, error) {
	switch name {
	case "", "pending":
		return ExtensionInstallationPending, nil
	case "enabled":
		return ExtensionInstallationEnabled, nil
	case "disabled":
		return ExtensionInstallationDisabled, nil
	case "error":
		return ExtensionInstallationError, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown extension installation status %q", name)
	}
}

// CertificateStatus enumerates certificate monitoring state
// (active|expiring|expired).
type CertificateStatus int

const (
	CertificateStatusActive CertificateStatus = iota
	CertificateStatusExpiring
	CertificateStatusExpired
)

func (c CertificateStatus) String() string {
	switch c {
	case CertificateStatusExpiring:
		return "expiring"
	case CertificateStatusExpired:
		return "expired"
	default:
		return "active"
	}
}

func (c CertificateStatus) Value() (driver.Value, error) { return ValueInt(int(c)) }

func (c *CertificateStatus) Scan(src any) error { return ScanInt((*int)(c), src) }

func (c CertificateStatus) MarshalJSON() ([]byte, error) { return json.Marshal(c.String()) }

func (c *CertificateStatus) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseCertificateStatus(name)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// ParseCertificateStatus converts a wire certificate status into the enum.
func ParseCertificateStatus(name string) (CertificateStatus, error) {
	switch name {
	case "", "active":
		return CertificateStatusActive, nil
	case "expiring":
		return CertificateStatusExpiring, nil
	case "expired":
		return CertificateStatusExpired, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown certificate status %q", name)
	}
}

// Risk enumerates FunctionContract.risk (safe|warning|high|danger).
type Risk int

const (
	RiskSafe Risk = iota
	RiskWarning
	RiskHigh
	RiskDanger
)

func (r Risk) String() string {
	switch r {
	case RiskWarning:
		return "warning"
	case RiskHigh:
		return "high"
	case RiskDanger:
		return "danger"
	default:
		return "safe"
	}
}

func (r Risk) Value() (driver.Value, error) { return ValueInt(int(r)) }

func (r *Risk) Scan(src any) error { return ScanInt((*int)(r), src) }

func (r Risk) MarshalJSON() ([]byte, error) { return json.Marshal(r.String()) }

func (r *Risk) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	parsed, err := ParseRisk(name)
	if err != nil {
		return err
	}
	*r = parsed
	return nil
}

// ParseRisk converts a wire risk level into the enum.
func ParseRisk(name string) (Risk, error) {
	switch name {
	case "", "safe", "low":
		return RiskSafe, nil
	case "warning", "medium":
		return RiskWarning, nil
	case "high":
		return RiskHigh, nil
	case "danger", "critical":
		return RiskDanger, nil
	default:
		return 0, fmt.Errorf("dbenum: unknown risk %q", name)
	}
}
