package identity

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

type fakeLDAPConn struct {
	serviceBindUser string
	serviceBindPass string
	bindDN          string
	bindPass        string
	bindErr         map[string]error // userDN -> bind error
	searchEntries   []*ldap.Entry
	searchErr       error
	closed          bool
}

func (f *fakeLDAPConn) Bind(username, password string) error {
	if strings.HasPrefix(username, "uid=svc") {
		f.serviceBindUser = username
		f.serviceBindPass = password
		return nil
	}
	f.bindDN = username
	f.bindPass = password
	if err, ok := f.bindErr[username]; ok {
		return err
	}
	return nil
}

func (f *fakeLDAPConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return &ldap.SearchResult{Entries: f.searchEntries}, nil
}

func (f *fakeLDAPConn) StartTLS(config *tls.Config) error { return nil }

func (f *fakeLDAPConn) Close() error { f.closed = true; return nil }

func newTestLDAPProvider(conn *fakeLDAPConn) *LDAPProvider {
	p := NewLDAPProvider(LDAPConfig{
		Addr:       "ldap://ldap.example.com:389",
		BaseDN:     "dc=example,dc=com",
		BindDN:     "uid=svc,ou=system,dc=example,dc=com",
		UserFilter: "(uid=%s)",
	})
	p.dial = func(addr string) (ldapConn, error) { return conn, nil }
	return p
}

func TestLDAPProvider_Authenticate_SearchAndBind(t *testing.T) {
	conn := &fakeLDAPConn{
		searchEntries: []*ldap.Entry{
			ldap.NewEntry("uid=alice,ou=people,dc=example,dc=com", map[string][]string{
				"cn":   {"Alice"},
				"mail": {"alice@example.com"},
			}),
		},
	}
	p := newTestLDAPProvider(conn)

	ident, err := p.Authenticate(context.Background(), "alice", "s3cret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.serviceBindUser == "" {
		t.Fatal("expected service bind before search")
	}
	if conn.bindDN != "uid=alice,ou=people,dc=example,dc=com" || conn.bindPass != "s3cret" {
		t.Fatalf("user bind mismatch: %s", conn.bindDN)
	}
	if ident.Provider != KindLDAP || ident.Username != "alice" || ident.Nickname != "Alice" || ident.Email != "alice@example.com" {
		t.Fatalf("unexpected identity: %+v", ident)
	}
	if !conn.closed {
		t.Fatal("connection must be closed after authenticate")
	}
}

func TestLDAPProvider_Authenticate_NoEntry(t *testing.T) {
	conn := &fakeLDAPConn{}
	p := newTestLDAPProvider(conn)

	_, err := p.Authenticate(context.Background(), "nobody", "x")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLDAPProvider_Authenticate_BindRejected(t *testing.T) {
	conn := &fakeLDAPConn{
		searchEntries: []*ldap.Entry{
			ldap.NewEntry("uid=alice,ou=people,dc=example,dc=com", nil),
		},
		bindErr: map[string]error{
			"uid=alice,ou=people,dc=example,dc=com": ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("bad password")),
		},
	}
	p := newTestLDAPProvider(conn)

	_, err := p.Authenticate(context.Background(), "alice", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestLDAPProvider_Authenticate_DNTemplate(t *testing.T) {
	conn := &fakeLDAPConn{
		bindErr: map[string]error{
			"uid=bob,ou=people,dc=example,dc=com": ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("no")),
		},
	}
	p := NewLDAPProvider(LDAPConfig{
		Addr:           "ldap://ldap.example.com:389",
		UserDNTemplate: "uid=%s,ou=people,dc=example,dc=com",
	})
	p.dial = func(addr string) (ldapConn, error) { return conn, nil }

	if _, err := p.Authenticate(context.Background(), "bob", "pw"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if conn.bindDN != "uid=bob,ou=people,dc=example,dc=com" {
		t.Fatalf("template DN mismatch: %s", conn.bindDN)
	}

	// 正确密码：直接绑定成功，nickname 回退为用户名。
	conn.bindErr = nil
	ident, err := p.Authenticate(context.Background(), "bob", "pw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ident.Nickname != "bob" {
		t.Fatalf("nickname should fall back to username, got %q", ident.Nickname)
	}
}

func TestLDAPProvider_Authenticate_FilterInjectionEscaped(t *testing.T) {
	var capturedFilter string
	conn := &fakeLDAPConn{
		searchEntries: []*ldap.Entry{},
	}
	p := NewLDAPProvider(LDAPConfig{
		Addr:       "ldap://ldap.example.com:389",
		BaseDN:     "dc=example,dc=com",
		BindDN:     "uid=svc,ou=system,dc=example,dc=com",
		UserFilter: "(uid=%s)",
	})
	conn2 := conn
	p.dial = func(addr string) (ldapConn, error) {
		return &filterCapturingConn{fake: conn2, filter: &capturedFilter}, nil
	}

	_, _ = p.Authenticate(context.Background(), "alice)(objectClass=*", "x")
	// ldap.EscapeFilter: ) -> \29, ( -> \28, * -> \2a，注入的过滤器结构被破坏。
	if !strings.Contains(capturedFilter, `uid=alice\29\28objectClass=\2a`) {
		t.Fatalf("filter injection not escaped: %q", capturedFilter)
	}
}

type filterCapturingConn struct {
	fake   *fakeLDAPConn
	filter *string
}

func (f *filterCapturingConn) Bind(username, password string) error {
	return f.fake.Bind(username, password)
}
func (f *filterCapturingConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	*f.filter = req.Filter
	return f.fake.Search(req)
}
func (f *filterCapturingConn) StartTLS(config *tls.Config) error { return f.fake.StartTLS(config) }
func (f *filterCapturingConn) Close() error                      { return f.fake.Close() }
