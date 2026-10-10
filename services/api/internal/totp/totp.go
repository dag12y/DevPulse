// Package totp implements RFC 6238 time-based one-time passwords with
// the stdlib only (crypto/hmac + SHA-1), plus otpauth:// enrollment URLs
// and a QR PNG for authenticator apps. SHA-1, 6 digits, and a 30-second
// period are the interoperable defaults every authenticator understands;
// no configuration is offered because none is worth the surface area.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	// secretByteCount is the RFC 4226 minimum recommended secret size
	// (160 bits). Shorter secrets are the weak link in most TOTP
	// deployments; we never generate less.
	secretByteCount = 20
	// digits and period are the authenticator-app defaults. Both are
	// encoded into the otpauth URL so apps do not have to guess.
	digits = 6
	period = 30
	// window is how many time steps either side of "now" a code is
	// accepted for. One step of skew covers clock drift between the
	// server and a phone without meaningfully widening the guessing
	// surface (codes still die in 30 seconds).
	window = 1
)

// base32NoPad is the otpauth convention: secrets travel unpadded so
// users can retype them without trailing "=" confusion.
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a fresh base32-encoded TOTP secret.
func GenerateSecret() (string, error) {
	raw := make([]byte, secretByteCount)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32NoPad.EncodeToString(raw), nil
}

// Validate reports whether code is a live TOTP for secret, accepting one
// time step of clock skew either side of now. Codes are compared in
// constant time; malformed codes are simply wrong (never an error, so
// callers cannot branch on input shape).
func Validate(code, secret string, now time.Time) bool {
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return false
	}
	digitsInCode := strings.TrimSpace(code)
	if len(digitsInCode) != digits {
		return false
	}
	counter := uint64(now.Unix() / period)
	for offset := -window; offset <= window; offset++ {
		var step uint64
		if offset < 0 && uint64(-offset) > counter {
			continue
		}
		step = counter + uint64(offset)
		if hmacEqual(digitsInCode, hotp(key, step)) {
			return true
		}
	}
	return false
}

// GenerateCode returns the code a correct authenticator would show at
// now. Exposed so tests (and only tests — production validates rather
// than produces) can exercise round-trips without reimplementing HOTP.
func GenerateCode(secret string, now time.Time) (string, error) {
	key, err := base32NoPad.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(key) == 0 {
		return "", fmt.Errorf("invalid totp secret")
	}
	return hotp(key, uint64(now.Unix()/period)), nil
}

// OTPAuthURL builds the otpauth:// enrollment link authenticator apps
// scan. The account label is the user's email so the app shows a
// recognizable entry next to any other TOTP tokens they hold.
func OTPAuthURL(issuer, account, secret string) string {
	// QueryEscape (not PathEscape) so "@" in the email becomes %40 —
	// every authenticator app accepts that form; "+" from spaces is
	// corrected because QueryEscape's plus is not a path escape.
	escape := func(value string) string {
		return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
	}
	label := escape(issuer) + ":" + escape(account)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprint(digits))
	query.Set("period", fmt.Sprint(period))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// QRCodePNG renders the otpauth URL as a PNG suitable for an <img> tag.
// Size is the image edge in pixels; larger codes scan more reliably on
// low-end phone cameras.
func QRCodePNG(otpauthURL string, size int) ([]byte, error) {
	if size < 128 {
		size = 128
	}
	png, err := qrcode.Encode(otpauthURL, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("encode totp qr: %w", err)
	}
	return png, nil
}

// hotp is RFC 4226 with SHA-1: HMAC-SHA1 over the 8-byte big-endian
// counter, dynamic truncation to 31 bits, modulo 10^digits.
func hotp(key []byte, counter uint64) string {
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(counterBytes[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, binaryCode%pow10(digits))
}

func pow10(n int) uint32 {
	result := uint32(1)
	for range n {
		result *= 10
	}
	return result
}

// hmacEqual compares equal-length hex-free digit strings in constant
// time so timing cannot distinguish a near-miss from a wild guess.
func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
