package common

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	rbac "github.com/cuihairu/croupier/internal/security/rbac"
	users "github.com/cuihairu/croupier/internal/security/users"
	"github.com/spf13/viper"
)

func fileExists(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func ValidateTLS(cert, key, ca string, strict bool) error {
	if strict {
		if err := fileExists(cert); err != nil {
			return fmt.Errorf("cert: %w", err)
		}
		if err := fileExists(key); err != nil {
			return fmt.Errorf("key: %w", err)
		}
		if err := fileExists(ca); err != nil {
			return fmt.Errorf("ca: %w", err)
		}
		return nil
	}
	// non-strict: allow empty (devcert fallback), but if provided then must exist
	if cert != "" {
		if err := fileExists(cert); err != nil {
			return fmt.Errorf("cert: %w", err)
		}
	}
	if key != "" {
		if err := fileExists(key); err != nil {
			return fmt.Errorf("key: %w", err)
		}
	}
	if ca != "" {
		if err := fileExists(ca); err != nil {
			return fmt.Errorf("ca: %w", err)
		}
	}
	return nil
}

func ValidateAddr(addr string) error {
	if addr == "" {
		return fmt.Errorf("empty address")
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return err
	}
	return nil
}

func ValidateServerConfig(v *viper.Viper, strict bool) error {
	// prefer section if exists
	if sub := v.Sub("server"); sub != nil {
		v = sub
	}
	if err := ValidateAddr(v.GetString("addr")); err != nil {
		return fmt.Errorf("addr: %w", err)
	}
	if err := ValidateAddr(v.GetString("http_addr")); err != nil {
		return fmt.Errorf("http_addr: %w", err)
	}
	if e := v.GetString("edge_addr"); e != "" {
		if err := ValidateAddr(e); err != nil {
			return fmt.Errorf("edge_addr: %w", err)
		}
	}
	if err := ValidateTLS(v.GetString("cert"), v.GetString("key"), v.GetString("ca"), strict); err != nil {
		return err
	}
	// data files
	if p := v.GetString("rbac_config"); p != "" {
		if _, err := rbac.LoadPolicy(p); err != nil {
			return fmt.Errorf("rbac_config: %w", err)
		}
	} else if strict {
		return fmt.Errorf("rbac_config missing")
	}
	if p := v.GetString("users_config"); p != "" {
		if _, err := users.Load(p); err != nil {
			return fmt.Errorf("users_config: %w", err)
		}
	} else if strict {
		return fmt.Errorf("users_config missing")
	}
	if p := v.GetString("games_config"); p != "" {
		// only check file exists (loader lives elsewhere and is invoked at runtime)
		if err := fileExists(filepath.Clean(p)); err != nil {
			return fmt.Errorf("games_config: %w", err)
		}
	} else if strict {
		return fmt.Errorf("games_config missing")
	}
	return nil
}

func ValidateAgentConfig(v *viper.Viper, strict bool) error {
	if sub := v.Sub("agent"); sub != nil {
		v = sub
	}
	if err := ValidateAddr(v.GetString("local_addr")); err != nil {
		return fmt.Errorf("local_addr: %w", err)
	}
	// prefer server_addr but accept core_addr
	server := v.GetString("server_addr")
	core := v.GetString("core_addr")
	addr := server
	if addr == "" {
		addr = core
	}
	if err := ValidateAddr(addr); err != nil {
		return fmt.Errorf("server_addr: %w", err)
	}
	if err := ValidateTLS(v.GetString("cert"), v.GetString("key"), v.GetString("ca"), strict); err != nil {
		return err
	}
	if err := ValidateAddr(v.GetString("http_addr")); err != nil {
		return fmt.Errorf("http_addr: %w", err)
	}
	return nil
}
