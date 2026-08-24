// Package ipgeo resolves IP to a region label using optional IP2Location
// LITE BIN databases. Restored from internal/server/http/ipgeo_ip2location.go
// (commit 2ed583c2f) after the http package refactor.
//
// Configure via env:
//
//	IP2LOCATION_BIN_PATH    -> IPv4 BIN (or IPv6 if that's the only one)
//	IP2LOCATION_BIN_PATH_V6 -> IPv6 BIN (optional)
//
// If unset, auto-detect under ./configs:
// IP2LOCATION-LITE-DB3.BIN / IP2LOCATION-LITE-DB3.IPV6.BIN (gitignored).
// When no BIN is present all lookups return "" — callers degrade gracefully.
package ipgeo

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ip2location "github.com/ip2location/ip2location-go/v9"
)

var (
	db4  *ip2location.DB
	db6  *ip2location.DB
	once sync.Once
)

func initDB() {
	p4 := strings.TrimSpace(os.Getenv("IP2LOCATION_BIN_PATH"))
	p6 := strings.TrimSpace(os.Getenv("IP2LOCATION_BIN_PATH_V6"))
	if p4 == "" && p6 == "" {
		const dir = "configs"
		c4 := filepath.Join(dir, "IP2LOCATION-LITE-DB3.BIN")
		c6 := filepath.Join(dir, "IP2LOCATION-LITE-DB3.IPV6.BIN")
		if _, err := os.Stat(c4); err == nil {
			p4 = c4
		}
		if _, err := os.Stat(c6); err == nil {
			p6 = c6
		}
	}
	// Heuristic: if only p4 provided but the filename contains IPV6, use it as v6.
	if p6 == "" && p4 != "" {
		if low := strings.ToLower(filepath.Base(p4)); strings.Contains(low, "ipv6") {
			p6, p4 = p4, ""
		}
	}
	if p4 != "" {
		if db, err := ip2location.OpenDB(p4); err == nil {
			db4 = db
		}
	}
	if p6 != "" {
		if db, err := ip2location.OpenDB(p6); err == nil {
			db6 = db
		}
	}
}

// Region returns "country/region/city" (long names, best effort) for ip.
// Empty string when the IP is invalid or no BIN database is configured.
func Region(ip string) string {
	once.Do(initDB)
	nip := net.ParseIP(strings.TrimSpace(ip))
	if nip == nil {
		return ""
	}
	db := db4
	if nip.To4() == nil {
		db = db6
	}
	if db == nil {
		return ""
	}
	rec, err := db.Get_all(nip.String())
	if err != nil {
		return ""
	}
	country := strings.TrimSpace(rec.Country_long)
	if country == "" {
		country = strings.TrimSpace(rec.Country_short)
	}
	region := strings.TrimSpace(rec.Region)
	city := strings.TrimSpace(rec.City)
	parts := make([]string, 0, 3)
	if country != "" {
		parts = append(parts, country)
	}
	if region != "" {
		parts = append(parts, region)
	}
	if city != "" {
		parts = append(parts, city)
	}
	return strings.Join(parts, "/")
}
