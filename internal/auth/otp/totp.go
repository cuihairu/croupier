package otp

import (
    "crypto/hmac"
    "crypto/sha1"
    "encoding/base32"
    "encoding/binary"
    "strconv"
    "strings"
    "time"
)

// VerifyTOTP verifies an RFC 6238 TOTP code with 30s step and given skew steps.
// secret can be base32 (no padding) as common authenticator apps export.
func VerifyTOTP(secret string, code string, skew int) bool {
    if len(code) < 6 || len(code) > 8 { return false }
    // decode base32 (ignore padding and case)
    s := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
    s = strings.TrimSpace(s)
    dec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
    if err != nil || len(dec) == 0 { return false }
    now := time.Now().Unix()
    step := now / 30
    // check within [-skew, +skew]
    for i := -skew; i <= skew; i++ {
        if hotp(dec, uint64(step+int64(i)), 6) == code { return true }
    }
    return false
}

func hotp(key []byte, counter uint64, digits int) string {
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, counter)
    mac := hmac.New(sha1.New, key)
    mac.Write(buf)
    sum := mac.Sum(nil)
    // dynamic truncation
    offset := int(sum[len(sum)-1] & 0x0F)
    bin := (int(sum[offset])&0x7f)<<24 | int(sum[offset+1])<<16 | int(sum[offset+2])<<8 | int(sum[offset+3])
    mod := 1
    for i := 0; i < digits; i++ { mod *= 10 }
    val := bin % mod
    s := strconv.Itoa(val)
    for len(s) < digits { s = "0" + s }
    return s
}

