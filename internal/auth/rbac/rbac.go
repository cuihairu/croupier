package rbac

// Minimal RBAC policy stub.
type Policy struct {
    // allow[user][permission] = true
    allow map[string]map[string]bool
}

func NewPolicy() *Policy { return &Policy{allow: map[string]map[string]bool{}} }

func (p *Policy) Grant(user, perm string) {
    m := p.allow[user]
    if m == nil { m = map[string]bool{}; p.allow[user] = m }
    m[perm] = true
}

func (p *Policy) Can(user, perm string) bool {
    if m := p.allow[user]; m != nil {
        if m[perm] { return true }
        if m["*"] { return true }
    }
    return false
}
