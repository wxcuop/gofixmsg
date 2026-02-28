package idgen

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ClOrdIDGenerator encodes and decodes sequence numbers to/from client order IDs.
// Encode must be reversible by Decode.
type ClOrdIDGenerator interface {
	Encode(n int) string
	Decode(s string) (int, error)
}

// NumericGenerator is a simple numeric encoder/decoder.
type NumericGenerator struct{}

func NewNumericGenerator() *NumericGenerator { return &NumericGenerator{} }

func (g *NumericGenerator) Encode(n int) string { return strconv.Itoa(n) }

func (g *NumericGenerator) Decode(s string) (int, error) { return strconv.Atoi(s) }

// YMDGenerator prefixes the ID with the current date in YYYYMMDD and a zero-padded sequence.
// Example: 20260228000042
type YMDGenerator struct {
	Now  func() time.Time // inject for tests; nil defaults to time.Now
	Width int            // number of digits for the sequence (default 6)
}

func NewYMDGenerator(now func() time.Time) *YMDGenerator {
	if now == nil {
		now = time.Now
	}
	return &YMDGenerator{Now: now, Width: 6}
}

func (g *YMDGenerator) Encode(n int) string {
	date := g.Now().Format("20060102")
	fmtStr := fmt.Sprintf("%%s%%0%dd", g.Width)
	return fmt.Sprintf(fmtStr, date, n)
}

func (g *YMDGenerator) Decode(s string) (int, error) {
	if len(s) <= 8 {
		return 0, errors.New("invalid YMD ClOrdID: too short")
	}
	seq := s[8:]
	i, err := strconv.Atoi(seq)
	if err != nil {
		return 0, fmt.Errorf("invalid sequence: %w", err)
	}
	return i, nil
}
