package httpserver

import (
    "net"
    "os"
    "path/filepath"
    "strings"
    "sync"
    // Always build with optional IP2Location support; if DB BIN files are missing, we just return empty and caller may fallback.
    // Dependency: github.com/ip2location/ip2location-go/v9 (add to go.mod)
    ip2location "github.com/ip2location/ip2location-go/v9"
)

var ip2db4 *ip2location.DB
var ip2db6 *ip2location.DB
var ip2once sync.Once

// ip2locRegion tries to resolve IP to region using IP2Location BIN database when built with tag 'ip2location'.
// Configure via env:
//   IP2LOCATION_BIN_PATH       -> IPv4 BIN (or IPv6 if that's the only one you have)
//   IP2LOCATION_BIN_PATH_V6    -> IPv6 BIN (optional)
// If paths are empty, will try default files under ./configs:
//   configs/IP2LOCATION-LITE-DB3.BIN and configs/IP2LOCATION-LITE-DB3.IPV6.BIN
func ip2locRegion(_ *Server, ip string) string {
    ip2once.Do(func(){
        p4 := strings.TrimSpace(os.Getenv("IP2LOCATION_BIN_PATH"))
        p6 := strings.TrimSpace(os.Getenv("IP2LOCATION_BIN_PATH_V6"))
        // If neither provided, try to auto-detect under configs/
        if p4 == "" && p6 == "" {
            d := "configs"
            c4 := filepath.Join(d, "IP2LOCATION-LITE-DB3.BIN")
            c6 := filepath.Join(d, "IP2LOCATION-LITE-DB3.IPV6.BIN")
            if _, err := os.Stat(c4); err == nil { p4 = c4 }
            if _, err := os.Stat(c6); err == nil { p6 = c6 }
        }
        // Heuristic: if only p4 provided but filename contains IPV6, treat it as v6
        if p6 == "" && p4 != "" {
            low := strings.ToLower(filepath.Base(p4))
            if strings.Contains(low, "ipv6") { p6, p4 = p4, "" }
        }
        if p4 != "" {
            if db, err := ip2location.OpenDB(p4); err == nil { ip2db4 = db }
        }
        if p6 != "" {
            if db, err := ip2location.OpenDB(p6); err == nil { ip2db6 = db }
        }
    })
    // Pick DB by IP family
    nip := net.ParseIP(ip)
    if nip == nil { return "" }
    var db *ip2location.DB
    if nip.To4() != nil {
        db = ip2db4
    } else {
        db = ip2db6
    }
    if db == nil { return "" }
    rec, err := db.Get_all(ip)
    if err != nil { return "" }
    country := strings.TrimSpace(rec.Country_long)
    if country == "" { country = strings.TrimSpace(rec.Country_short) }
    region := strings.TrimSpace(rec.Region)
    city := strings.TrimSpace(rec.City)
    parts := []string{}
    if country != "" { parts = append(parts, country) }
    if region != "" { parts = append(parts, region) }
    if city != "" { parts = append(parts, city) }
    return strings.Join(parts, "/")
}
