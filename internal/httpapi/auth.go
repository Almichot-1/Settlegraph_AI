package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const demoIssuer = "settlegraph-local-demo"

type claims struct {
	Subject string `json:"sub"`
	Issuer  string `json:"iss"`
	Expires int64  `json:"exp"`
	Issued  int64  `json:"iat"`
}

func mintToken(secret, subject string, now time.Time) (string, time.Time) {
	expires := now.Add(time.Hour)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	data, _ := json.Marshal(claims{Subject: subject, Issuer: demoIssuer, Expires: expires.Unix(), Issued: now.Unix()})
	payload := base64.RawURLEncoding.EncodeToString(data)
	unsigned := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expires
}

func verifyToken(secret, token string, now time.Time) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "HS256" || header.Typ != "JWT" {
		return "", errors.New("unsupported token header")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", errors.New("invalid signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var c claims
	if json.Unmarshal(payload, &c) != nil || c.Subject == "" || c.Issuer != demoIssuer || c.Expires <= now.Unix() || c.Issued > now.Add(time.Minute).Unix() {
		return "", errors.New("invalid token claims")
	}
	return c.Subject, nil
}
