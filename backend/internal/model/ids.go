package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]{6,64}$`)

func NewID(prefix string) string {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read on crypto/rand fails only if the OS is catastrophically broken.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	id := hex.EncodeToString(b[:])
	if prefix == "" {
		return id
	}
	return prefix + "_" + id
}

func ValidID(s string) bool {
	s = strings.TrimSpace(s)
	return idRe.MatchString(s)
}

func ValidColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func ClampNickname(s string) string {
	s = strings.TrimSpace(s)
	rs := []rune(s)
	if len(rs) == 0 {
		return "Guest"
	}
	if len(rs) > 24 {
		return string(rs[:24])
	}
	return s
}
