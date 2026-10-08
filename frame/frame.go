// Package frame implements the device frame contract in FRAME_FORMAT.md (version 2).
//
// The simulator (sim/sensor_sim) encodes uplink frames and decodes downlink frames with
// it, and the gateway does the opposite, so both sides share one implementation of the
// contract. Section numbers in
// comments refer to FRAME_FORMAT.md.
package frame

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// MaxLineLen is the maximum line length in bytes, including the terminating LF (section 1).
const MaxLineLen = 128

// Field is one KEY:VALUE data field; Value is the scaled integer as sent on the wire.
type Field struct {
	Key   string
	Value int32
}

// Frame is one parsed or to-be-encoded frame.
type Frame struct {
	Seq    uint16
	Fields []Field // data fields in wire order; includes BOOT when present
	Tag    string  // HMAC tag (16 lowercase hex digits) if the frame carried one
	signed string  // the bytes the tag covers, set by Parse
}

// Get returns the value of key and whether the frame carries it.
func (f Frame) Get(key string) (int32, bool) {
	for _, fl := range f.Fields {
		if fl.Key == key {
			return fl.Value, true
		}
	}
	return 0, false
}

// Boot reports whether the frame carries the restart marker (section 4).
func (f Frame) Boot() bool {
	_, ok := f.Get("BOOT")
	return ok
}

// Errors returned by Parse. Every one means "drop the frame" (section 4).
var (
	ErrTooLong      = errors.New("frame: line exceeds 128 bytes")
	ErrNonASCII     = errors.New("frame: non-ASCII byte")
	ErrEmpty        = errors.New("frame: empty line")
	ErrNoSeq        = errors.New("frame: missing or non-first S field")
	ErrSyntax       = errors.New("frame: bad field syntax")
	ErrDuplicateKey = errors.New("frame: key repeated in one frame")
	ErrNoData       = errors.New("frame: no data field")
)

// Parse parses one line (with or without the trailing LF; a CR before the LF is
// tolerated). It checks syntax only: key ranges, unknown keys, HMAC and sequence
// rules are applied by Check, Verifier and Tracker.
func Parse(line string) (Frame, error) { return parse(line, "S") }

// ParseDownlink parses one downlink line (section 7): the same syntax with "C" as the
// sequence field.
func ParseDownlink(line string) (Frame, error) { return parse(line, "C") }

func parse(line, seqKey string) (Frame, error) {
	if len(line) > MaxLineLen {
		return Frame{}, ErrTooLong
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	if len(line)+1 > MaxLineLen {
		return Frame{}, ErrTooLong
	}
	for i := 0; i < len(line); i++ {
		if line[i] > 0x7e || line[i] < 0x20 {
			return Frame{}, ErrNonASCII
		}
	}
	if line == "" {
		return Frame{}, ErrEmpty
	}
	parts := strings.Split(line, ";")
	var f Frame
	seen := map[string]bool{}
	for i, p := range parts {
		k, v, ok := strings.Cut(p, ":")
		if !ok || k == "" || v == "" {
			return Frame{}, ErrSyntax
		}
		switch {
		case i == 0:
			if k != seqKey {
				return Frame{}, ErrNoSeq
			}
			n, err := parseDigits(v, 5)
			if err != nil || n > 65535 {
				return Frame{}, ErrSyntax
			}
			f.Seq = uint16(n)
		case k == seqKey:
			return Frame{}, ErrNoSeq
		case k == "H":
			if i != len(parts)-1 || !isLowerHex(v, 16) {
				return Frame{}, ErrSyntax
			}
			f.Tag = v
			f.signed = strings.Join(parts[:i], ";") + ";"
		default:
			if !validKey(k) {
				return Frame{}, ErrSyntax
			}
			n, err := parseValue(v)
			if err != nil {
				return Frame{}, ErrSyntax
			}
			if seen[k] {
				return Frame{}, ErrDuplicateKey
			}
			seen[k] = true
			f.Fields = append(f.Fields, Field{Key: k, Value: n})
		}
	}
	if len(f.Fields) == 0 {
		return Frame{}, ErrNoData
	}
	return f, nil
}

// Encode renders f as a wire line including the trailing LF. With a non-nil key it
// appends the HMAC tag (section 5); f.Tag is ignored.
func Encode(f Frame, key []byte) string { return encode(f, key, "S:") }

// EncodeDownlink renders a downlink frame (section 7), with "C" as the sequence field.
func EncodeDownlink(f Frame, key []byte) string { return encode(f, key, "C:") }

func encode(f Frame, key []byte, seqPrefix string) string {
	var b strings.Builder
	b.WriteString(seqPrefix)
	b.WriteString(strconv.Itoa(int(f.Seq)))
	for _, fl := range f.Fields {
		b.WriteByte(';')
		b.WriteString(fl.Key)
		b.WriteByte(':')
		b.WriteString(strconv.FormatInt(int64(fl.Value), 10))
	}
	if key != nil {
		b.WriteByte(';')
		body := b.String()
		b.WriteString("H:")
		b.WriteString(Tag(key, body))
	}
	b.WriteByte('\n')
	return b.String()
}

// Tag computes the section 5 tag: HMAC-SHA256 over signed, first 8 bytes, lowercase hex.
func Tag(key []byte, signed string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(signed))
	return hex.EncodeToString(m.Sum(nil)[:8])
}

// ParseKey decodes a 64-hex-digit HMAC key as configured in FRAME_HMAC_KEY.
func ParseKey(s string) ([]byte, error) {
	k, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("frame: HMAC key must be 64 hex digits (32 bytes)")
	}
	return k, nil
}

func validKey(k string) bool {
	if len(k) < 1 || len(k) > 8 || k[0] < 'A' || k[0] > 'Z' {
		return false
	}
	for i := 1; i < len(k); i++ {
		c := k[i]
		if !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func parseDigits(s string, max int) (int64, error) {
	if len(s) == 0 || len(s) > max {
		return 0, ErrSyntax
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, ErrSyntax
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseValue(s string) (int32, error) {
	neg := strings.HasPrefix(s, "-")
	n, err := parseDigits(strings.TrimPrefix(s, "-"), 10)
	if err != nil {
		return 0, err
	}
	if neg {
		n = -n
	}
	if n < -2147483648 || n > 2147483647 {
		return 0, ErrSyntax
	}
	return int32(n), nil
}

func isLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
