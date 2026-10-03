package frame

import (
	"crypto/hmac"
	"errors"
	"fmt"
)

// HMACMode is the gateway's FRAME_HMAC setting (section 5).
type HMACMode int

const (
	HMACOff HMACMode = iota
	HMACOptional
	HMACRequired
)

// ParseHMACMode parses "off", "optional" or "required".
func ParseHMACMode(s string) (HMACMode, error) {
	switch s {
	case "", "off":
		return HMACOff, nil
	case "optional":
		return HMACOptional, nil
	case "required":
		return HMACRequired, nil
	}
	return HMACOff, fmt.Errorf("frame: FRAME_HMAC must be off, optional or required, got %q", s)
}

var (
	ErrUntagged   = errors.New("frame: untagged frame with FRAME_HMAC=required")
	ErrBadTag     = errors.New("frame: HMAC tag mismatch")
	ErrKeyMissing = errors.New("frame: FRAME_HMAC is on but no key is configured")
)

// VerifyTag applies the section 5 gateway rules to a parsed frame.
func VerifyTag(f Frame, mode HMACMode, key []byte) error {
	if mode == HMACOff {
		return nil
	}
	if f.Tag == "" {
		if mode == HMACRequired {
			return ErrUntagged
		}
		return nil
	}
	if key == nil {
		return ErrKeyMissing
	}
	want := Tag(key, f.signed)
	if !hmac.Equal([]byte(want), []byte(f.Tag)) {
		return ErrBadTag
	}
	return nil
}
