package svc

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/repo/gorm/support"
)

type SupportRepository interface {
	ListTickets(ctx context.Context, opts SupportListOptions) ([]*support.Ticket, int64, error)
	CreateTicket(ctx context.Context, t *support.Ticket) error
	UpdateTicket(ctx context.Context, t *support.Ticket) error
	DeleteTicket(ctx context.Context, id uint) error
	GetTicket(ctx context.Context, id uint) (*support.Ticket, error)
	ListComments(ctx context.Context, ticketID uint) ([]*support.TicketComment, error)
	CreateComment(ctx context.Context, cmt *support.TicketComment) error
	ListFAQ(ctx context.Context, opts SupportFAQListOptions) ([]*support.FAQ, error)
	CreateFAQ(ctx context.Context, faq *support.FAQ) error
	UpdateFAQ(ctx context.Context, faq *support.FAQ) error
	DeleteFAQ(ctx context.Context, id uint) error
	GetFAQ(ctx context.Context, id uint) (*support.FAQ, error)
	ListFeedback(ctx context.Context, opts SupportFeedbackListOptions) ([]*support.Feedback, int64, error)
	CreateFeedback(ctx context.Context, fb *support.Feedback) error
	UpdateFeedback(ctx context.Context, fb *support.Feedback) error
	DeleteFeedback(ctx context.Context, id uint) error
	GetFeedback(ctx context.Context, id uint) (*support.Feedback, error)
}

type SupportListOptions struct {
	Query    string
	Status   string
	Priority string
	Category string
	Assignee string
	GameID   string
	Env      string
	Page     int
	Size     int
}

type SupportFAQListOptions struct {
	Query    string
	Category string
	Visible  *bool
}

type SupportFeedbackListOptions struct {
	Query    string
	Category string
	Status   string
	GameID   string
	Env      string
	Page     int
	Size     int
}

var (
	ErrSupportTicketNotFound   = errors.New("ticket not found")
	ErrSupportFAQNotFound      = errors.New("faq not found")
	ErrSupportFeedbackNotFound = errors.New("feedback not found")
)

type memorySupportRepo struct {
	mu             sync.Mutex
	nextID         uint
	tickets        map[uint]*support.Ticket
	comments       map[uint][]*support.TicketComment
	nextFAQID      uint
	faqs           map[uint]*support.FAQ
	nextFeedbackID uint
	feedback       map[uint]*support.Feedback
}

func newMemorySupportRepo() *memorySupportRepo {
	return &memorySupportRepo{
		nextID:         1,
		tickets:        map[uint]*support.Ticket{},
		comments:       map[uint][]*support.TicketComment{},
		nextFAQID:      1,
		faqs:           map[uint]*support.FAQ{},
		nextFeedbackID: 1,
		feedback:       map[uint]*support.Feedback{},
	}
}

func (m *memorySupportRepo) ListTickets(ctx context.Context, opts SupportListOptions) ([]*support.Ticket, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	filtered := make([]*support.Ticket, 0, len(m.tickets))
	for _, t := range m.tickets {
		if matchTicket(t, opts) {
			cp := *t
			filtered = append(filtered, &cp)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	total := int64(len(filtered))
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	size := opts.Size
	if size <= 0 || size > 200 {
		size = 20
	}
	start := (page - 1) * size
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}

func matchTicket(t *support.Ticket, opts SupportListOptions) bool {
	if t == nil {
		return false
	}
	if opts.Query != "" {
		q := strings.ToLower(opts.Query)
		if !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.Content), q) {
			return false
		}
	}
	if opts.Status != "" && !strings.EqualFold(t.Status, opts.Status) {
		return false
	}
	if opts.Priority != "" && !strings.EqualFold(t.Priority, opts.Priority) {
		return false
	}
	if opts.Category != "" && !strings.EqualFold(t.Category, opts.Category) {
		return false
	}
	if opts.Assignee != "" && !strings.EqualFold(t.Assignee, opts.Assignee) {
		return false
	}
	if opts.GameID != "" && t.GameID != opts.GameID {
		return false
	}
	if opts.Env != "" && t.Env != opts.Env {
		return false
	}
	return true
}

func (m *memorySupportRepo) CreateTicket(ctx context.Context, t *support.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID
	m.nextID++
	now := time.Now()
	cp := *t
	cp.ID = id
	cp.CreatedAt = now
	cp.UpdatedAt = now
	m.tickets[id] = &cp
	if t != nil {
		t.ID = id
		t.CreatedAt = now
		t.UpdatedAt = now
	}
	return nil
}

func (m *memorySupportRepo) UpdateTicket(ctx context.Context, t *support.Ticket) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.tickets[t.ID]; ok {
		cp := *existing
		if t.Title != "" {
			cp.Title = t.Title
		}
		if t.Content != "" {
			cp.Content = t.Content
		}
		if t.Category != "" {
			cp.Category = t.Category
		}
		if t.Priority != "" {
			cp.Priority = t.Priority
		}
		if t.Status != "" {
			cp.Status = t.Status
		}
		if t.Assignee != "" {
			cp.Assignee = t.Assignee
		}
		if t.Tags != "" || t.Tags == "" {
			cp.Tags = t.Tags
		}
		cp.PlayerID = t.PlayerID
		cp.Contact = t.Contact
		cp.GameID = t.GameID
		cp.Env = t.Env
		cp.Source = t.Source
		cp.UpdatedAt = time.Now()
		m.tickets[t.ID] = &cp
		return nil
	}
	return ErrSupportTicketNotFound
}

func (m *memorySupportRepo) DeleteTicket(ctx context.Context, id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tickets, id)
	delete(m.comments, id)
	return nil
}

func (m *memorySupportRepo) GetTicket(ctx context.Context, id uint) (*support.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.tickets[id]; t != nil {
		cp := *t
		return &cp, nil
	}
	return nil, ErrSupportTicketNotFound
}

func (m *memorySupportRepo) ListComments(ctx context.Context, ticketID uint) ([]*support.TicketComment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	arr := m.comments[ticketID]
	out := make([]*support.TicketComment, 0, len(arr))
	for _, c := range arr {
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

func (m *memorySupportRepo) CreateComment(ctx context.Context, cmt *support.TicketComment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cmt == nil {
		return errors.New("comment required")
	}
	if _, ok := m.tickets[cmt.TicketID]; !ok {
		return ErrSupportTicketNotFound
	}
	cpy := *cmt
	cpy.ID = uint(len(m.comments[cmt.TicketID]) + 1)
	cpy.CreatedAt = time.Now()
	m.comments[cmt.TicketID] = append(m.comments[cmt.TicketID], &cpy)
	cmt.ID = cpy.ID
	cmt.CreatedAt = cpy.CreatedAt
	return nil
}

func (m *memorySupportRepo) ListFAQ(ctx context.Context, opts SupportFAQListOptions) ([]*support.FAQ, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	arr := make([]*support.FAQ, 0, len(m.faqs))
	for _, faq := range m.faqs {
		if matchFAQ(faq, opts) {
			cp := *faq
			arr = append(arr, &cp)
		}
	}
	sort.Slice(arr, func(i, j int) bool {
		if arr[i].Sort == arr[j].Sort {
			return arr[i].UpdatedAt.After(arr[j].UpdatedAt)
		}
		return arr[i].Sort > arr[j].Sort
	})
	return arr, nil
}

func matchFAQ(f *support.FAQ, opts SupportFAQListOptions) bool {
	if f == nil {
		return false
	}
	if opts.Query != "" {
		q := strings.ToLower(opts.Query)
		if !strings.Contains(strings.ToLower(f.Question), q) && !strings.Contains(strings.ToLower(f.Answer), q) {
			return false
		}
	}
	if opts.Category != "" && !strings.EqualFold(f.Category, opts.Category) {
		return false
	}
	if opts.Visible != nil && f.Visible != *opts.Visible {
		return false
	}
	return true
}

func (m *memorySupportRepo) CreateFAQ(ctx context.Context, faq *support.FAQ) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if faq == nil {
		return errors.New("faq required")
	}
	id := m.nextFAQID
	m.nextFAQID++
	now := time.Now()
	cp := *faq
	cp.ID = id
	cp.CreatedAt = now
	cp.UpdatedAt = now
	m.faqs[id] = &cp
	faq.ID = id
	faq.CreatedAt = now
	faq.UpdatedAt = now
	return nil
}

func (m *memorySupportRepo) UpdateFAQ(ctx context.Context, faq *support.FAQ) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.faqs[faq.ID]
	if !ok {
		return ErrSupportFAQNotFound
	}
	cp := *faq
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = time.Now()
	m.faqs[faq.ID] = &cp
	return nil
}

func (m *memorySupportRepo) DeleteFAQ(ctx context.Context, id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.faqs, id)
	return nil
}

func (m *memorySupportRepo) GetFAQ(ctx context.Context, id uint) (*support.FAQ, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := m.faqs[id]; f != nil {
		cp := *f
		return &cp, nil
	}
	return nil, ErrSupportFAQNotFound
}

func (m *memorySupportRepo) ListFeedback(ctx context.Context, opts SupportFeedbackListOptions) ([]*support.Feedback, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	arr := make([]*support.Feedback, 0, len(m.feedback))
	for _, fb := range m.feedback {
		if matchFeedback(fb, opts) {
			cp := *fb
			arr = append(arr, &cp)
		}
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].UpdatedAt.After(arr[j].UpdatedAt)
	})
	total := int64(len(arr))
	page := opts.Page
	if page <= 0 {
		page = 1
	}
	size := opts.Size
	if size <= 0 || size > 200 {
		size = 20
	}
	start := (page - 1) * size
	if start > len(arr) {
		start = len(arr)
	}
	end := start + size
	if end > len(arr) {
		end = len(arr)
	}
	return arr[start:end], total, nil
}

func matchFeedback(f *support.Feedback, opts SupportFeedbackListOptions) bool {
	if f == nil {
		return false
	}
	if opts.Query != "" && !strings.Contains(strings.ToLower(f.Content), strings.ToLower(opts.Query)) {
		return false
	}
	if opts.Category != "" && !strings.EqualFold(f.Category, opts.Category) {
		return false
	}
	if opts.Status != "" && !strings.EqualFold(f.Status, opts.Status) {
		return false
	}
	if opts.GameID != "" && f.GameID != opts.GameID {
		return false
	}
	if opts.Env != "" && f.Env != opts.Env {
		return false
	}
	return true
}

func (m *memorySupportRepo) CreateFeedback(ctx context.Context, fb *support.Feedback) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if fb == nil {
		return errors.New("feedback required")
	}
	id := m.nextFeedbackID
	m.nextFeedbackID++
	now := time.Now()
	cp := *fb
	cp.ID = id
	cp.CreatedAt = now
	cp.UpdatedAt = now
	m.feedback[id] = &cp
	fb.ID = id
	fb.CreatedAt = now
	fb.UpdatedAt = now
	return nil
}

func (m *memorySupportRepo) UpdateFeedback(ctx context.Context, fb *support.Feedback) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.feedback[fb.ID]
	if !ok {
		return ErrSupportFeedbackNotFound
	}
	cp := *fb
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = time.Now()
	m.feedback[fb.ID] = &cp
	return nil
}

func (m *memorySupportRepo) DeleteFeedback(ctx context.Context, id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.feedback, id)
	return nil
}

func (m *memorySupportRepo) GetFeedback(ctx context.Context, id uint) (*support.Feedback, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if f := m.feedback[id]; f != nil {
		cp := *f
		return &cp, nil
	}
	return nil, ErrSupportFeedbackNotFound
}
