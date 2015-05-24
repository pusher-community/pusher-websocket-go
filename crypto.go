package pusher

import (
	"crypto/hmac"
	// "crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func hmacSignature(toSign, secret string) string {
	return hex.EncodeToString(hmacBytes([]byte(toSign), []byte(secret)))
}

func hmacBytes(toSign, secret []byte) []byte {
	_authSignature := hmac.New(sha256.New, secret)
	_authSignature.Write(toSign)
	return _authSignature.Sum(nil)
}

func createAuthString(key, secret, stringToSign string) string {
	authSignature := hmacSignature(stringToSign, secret)
	return strings.Join([]string{key, authSignature}, ":")
}
