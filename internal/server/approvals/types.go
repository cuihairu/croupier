package approvals

import "time"

// Approval represents a two-person-rule pending action to be executed later.
type Approval struct {
    ID             string    `json:"id"`
    CreatedAt      time.Time `json:"created_at"`
    Actor          string    `json:"actor"`
    FunctionID     string    `json:"function_id"`
    Payload        []byte    `json:"-"` // raw payload already transport-encoded (json or pb-bin)
    IdempotencyKey string    `json:"idempotency_key"`
    Route          string    `json:"route,omitempty"`
    TargetServiceID string   `json:"target_service_id,omitempty"`
    HashKey        string    `json:"hash_key,omitempty"`
    GameID         string    `json:"game_id,omitempty"`
    Env            string    `json:"env,omitempty"`
    State          string    `json:"state"` // pending|approved|rejected
    Mode           string    `json:"mode"`  // invoke|start_job
    Reason         string    `json:"reason,omitempty"` // for rejection
}

type Filter struct {
    State      string
    FunctionID string
    GameID     string
    Env        string
    Actor      string
    Mode       string
}

type Page struct {
    Page int
    Size int
    Sort string // created_at_desc|created_at_asc
}

type Store interface {
    Create(a *Approval) error
    Get(id string) (*Approval, error)
    Approve(id string) (*Approval, error)
    Reject(id string, reason string) (*Approval, error)
    List(f Filter, p Page) (items []*Approval, total int, err error)
}

