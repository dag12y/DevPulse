package users

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"

	"github.com/dag12y/devpulse/internal/auth"
	"github.com/dag12y/devpulse/internal/totp"
)

// otpauthIssuer labels enrolled tokens in authenticator apps so a user
// with several TOTP entries can tell ours apart.
const otpauthIssuer = "DevPulse"

// TwoFactorSetup generates a fresh enrollment secret and returns the
// otpauth URL plus a rendered QR image. The secret is stored in the
// pending state (not enabled): login keeps demanding only the password
// until Enable verifies a code against it, so abandoning setup mid-way
// can never lock anyone out.
func (handler *Handler) TwoFactorSetup(w http.ResponseWriter, r *http.Request) {
	userID, user, ok := handler.sessionUser(w, r)
	if !ok {
		return
	}
	if user.TOTPEnabled {
		writeError(w, http.StatusConflict, "two-factor authentication is already enabled; disable it first")
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		slog.Error("generate totp secret", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to set up two-factor authentication")
		return
	}
	if err := handler.store.SetUserTOTPSecret(r.Context(), userID, secret); err != nil {
		slog.Error("store totp secret", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to set up two-factor authentication")
		return
	}
	otpauthURL := totp.OTPAuthURL(otpauthIssuer, user.Email, secret)
	qr, err := totp.QRCodePNG(otpauthURL, 256)
	if err != nil {
		slog.Error("render totp qr", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to set up two-factor authentication")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"secret":        secret,
		"otpauth_url":   otpauthURL,
		"qr_png_base64": base64.StdEncoding.EncodeToString(qr),
	})
}

type totpCodeInput struct {
	Code string `json:"code"`
}

// TwoFactorEnable verifies a code against the pending secret and flips
// 2FA on. The verification is what makes the enrollment safe: a secret
// the user cannot actually generate codes from is never made binding.
func (handler *Handler) TwoFactorEnable(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := handler.sessionUser(w, r)
	if !ok {
		return
	}
	var input totpCodeInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	secret, enabled, err := handler.store.GetUserTOTP(r.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err != nil {
		slog.Error("load totp secret", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to enable two-factor authentication")
		return
	}
	if enabled {
		writeError(w, http.StatusConflict, "two-factor authentication is already enabled")
		return
	}
	if secret == "" {
		writeError(w, http.StatusBadRequest, "start two-factor setup before enabling")
		return
	}
	if !totp.Validate(input.Code, secret, handler.now().UTC()) {
		writeError(w, http.StatusUnauthorized, "invalid two-factor code")
		return
	}
	if err := handler.store.EnableUserTOTP(r.Context(), userID); err != nil {
		slog.Error("enable totp", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to enable two-factor authentication")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": true})
}

// TwoFactorDisable turns 2FA off after verifying a live code. Requiring
// the code (not just the session) means a stolen session cookie alone
// cannot strip the second factor — the attacker would also need the
// authenticator.
func (handler *Handler) TwoFactorDisable(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := handler.sessionUser(w, r)
	if !ok {
		return
	}
	var input totpCodeInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	secret, enabled, err := handler.store.GetUserTOTP(r.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if err != nil {
		slog.Error("load totp secret", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to disable two-factor authentication")
		return
	}
	if !enabled {
		writeError(w, http.StatusConflict, "two-factor authentication is not enabled")
		return
	}
	if !totp.Validate(input.Code, secret, handler.now().UTC()) {
		writeError(w, http.StatusUnauthorized, "invalid two-factor code")
		return
	}
	if err := handler.store.DisableUserTOTP(r.Context(), userID); err != nil {
		slog.Error("disable totp", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to disable two-factor authentication")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": false})
}

// sessionUser resolves the caller for session-guarded endpoints,
// answering 401 itself when the context carries no user (the middleware
// normally guarantees one; stub stores in tests may not).
func (handler *Handler) sessionUser(w http.ResponseWriter, r *http.Request) (string, *User, bool) {
	userID, present := auth.UserFromContext(r.Context())
	if !present {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return "", nil, false
	}
	user, err := handler.store.FindUserByID(r.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return "", nil, false
	}
	if err != nil {
		slog.Error("find user", "error", err)
		writeError(w, http.StatusInternalServerError, "unable to load account")
		return "", nil, false
	}
	return userID, user, true
}
