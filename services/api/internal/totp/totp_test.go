package totp

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 Appendix B publishes SHA-1 vectors for secret
// "12345678901234567890" (ASCII, base32 "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ").
// These pin the implementation to the standard rather than to itself.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestValidateRFC6238Vectors(t *testing.T) {
	cases := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, testCase := range cases {
		now := time.Unix(testCase.unix, 0).UTC()
		if !Validate(testCase.code, rfcSecret, now) {
			t.Errorf("Validate(%q, t=%d) = false, want true", testCase.code, testCase.unix)
		}
	}
}

func TestValidateRejectsWrongCodeAndAdjacentSteps(t *testing.T) {
	now := time.Unix(1234567890, 0).UTC()
	if Validate("005925", rfcSecret, now) {
		t.Error("wrong code accepted")
	}
	if Validate("", rfcSecret, now) {
		t.Error("empty code accepted")
	}
	if Validate("00592", rfcSecret, now) {
		t.Error("short code accepted")
	}
	if Validate("005924", "not-base32!!", now) {
		t.Error("malformed secret accepted")
	}
	// One step of skew either side is accepted; two is not.
	previousStep := now.Add(-period * time.Second)
	if !Validate("005924", rfcSecret, previousStep) {
		t.Error("code from previous step rejected inside skew window")
	}
	twoStepsAgo := now.Add(-2 * period * time.Second)
	if Validate("005924", rfcSecret, twoStepsAgo) {
		t.Error("code from two steps ago accepted")
	}
}

func TestGenerateSecretShape(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	// 20 bytes, base32 without padding, is 32 characters.
	if len(secret) != 32 {
		t.Fatalf("secret length = %d, want 32 (%q)", len(secret), secret)
	}
	if strings.Contains(secret, "=") {
		t.Fatalf("secret contains padding: %q", secret)
	}
	second, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	if secret == second {
		t.Fatal("two generated secrets are identical")
	}
	if !Validate(currentCode(t, secret), secret, time.Now()) {
		t.Fatal("freshly generated secret does not validate its own current code")
	}
}

func TestOTPAuthURL(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	link := OTPAuthURL("DevPulse", "ada@example.com", secret)
	for _, fragment := range []string{
		"otpauth://totp/",
		"DevPulse:ada%40example.com",
		"secret=" + secret,
		"issuer=DevPulse",
		"algorithm=SHA1",
		"digits=6",
		"period=30",
	} {
		if !strings.Contains(link, fragment) {
			t.Errorf("otpauth URL missing %q: %s", fragment, link)
		}
	}
}

func TestQRCodePNG(t *testing.T) {
	link := OTPAuthURL("DevPulse", "ada@example.com", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ")
	png, err := QRCodePNG(link, 256)
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 100 {
		t.Fatalf("png suspiciously small: %d bytes", len(png))
	}
	// PNG magic number.
	if png[0] != 0x89 || png[1] != 'P' || png[2] != 'N' || png[3] != 'G' {
		t.Fatalf("not a PNG: %x", png[:4])
	}
}

// currentCode computes the expected code for now so the test does not
// need its own HOTP reimplementation — it exercises Validate against
// GenerateSecret end-to-end instead of pinning another constant.
func currentCode(t *testing.T, secret string) string {
	t.Helper()
	// Validate is the oracle: walk candidate codes only in tests is
	// silly, so recompute via the same hotp path the package uses.
	key, err := decodeSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	return hotp(key, uint64(time.Now().Unix()/period))
}

func decodeSecret(secret string) ([]byte, error) {
	return base32NoPad.DecodeString(strings.ToUpper(secret))
}
