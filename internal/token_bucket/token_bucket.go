package token_bucket

import (
	"encoding/binary"
	"time"
)

const (
	// InternalKey is the reserved key under which the token bucket state is stored in the system
	// The "__" prefix marks this as an internal record not accessible to the user
	InternalKey = "__token_bucket__"
)

// TokenBucket implements the Token Bucket algorithm for rate limiting
type TokenBucket struct {
	maxTokens       int64 // maximum number of tokens (bucket capacity)
	resetIntervalMs int64 // reset interval in milliseconds
	tokens          int64 // current number of available tokens
	lastResetMs     int64 // timestamp of the last reset (UnixMilli)
}

// New creates a new TokenBucket with the given parameters.
func New(maxTokens int64, resetIntervalMs int64) *TokenBucket {
	return &TokenBucket{
		maxTokens:       maxTokens,
		resetIntervalMs: resetIntervalMs,
		tokens:          maxTokens,
		lastResetMs:     time.Now().UnixMilli(),
	}
}

// TryConsume attempts to consume one token
// Returns true if the operation is allowed and false if the rate limit has been reached
func (tb *TokenBucket) TryConsume() bool {
	tb.refill()
	if tb.tokens <= 0 {
		return false
	}
	tb.tokens--
	return true
}

// refill resets the token count if the reset interval has elapsed
func (tb *TokenBucket) refill() {
	now := time.Now().UnixMilli()
	elapsed := now - tb.lastResetMs
	if elapsed >= tb.resetIntervalMs {
		tb.tokens = tb.maxTokens
		tb.lastResetMs = now
	}
}

// Serialize encodes the bucket state into a byte slice for storage in the system
// Format: maxTokens(8) | resetIntervalMs(8) | tokens(8) | lastResetMs(8)
func (tb *TokenBucket) Serialize() []byte {
	buf := make([]byte, 32)
	n := 0
	n += binary.PutVarint(buf[n:], tb.maxTokens)
	n += binary.PutVarint(buf[n:], tb.resetIntervalMs)
	n += binary.PutVarint(buf[n:], tb.tokens)
	n += binary.PutVarint(buf[n:], tb.lastResetMs)
	return buf[:n]
}

// Deserialize decodes the bucket state from a byte slice
func Deserialize(data []byte) *TokenBucket {
	if len(data) == 0 {
		return nil
	}
	tb := &TokenBucket{}
	offset := 0
	var n int
	tb.maxTokens, n = binary.Varint(data[offset:])
	offset += n
	tb.resetIntervalMs, n = binary.Varint(data[offset:])
	offset += n
	tb.tokens, n = binary.Varint(data[offset:])
	offset += n
	tb.lastResetMs, _ = binary.Varint(data[offset:])
	return tb
}

// IsEnabled reports whether the token bucket is configured (maxTokens > 0)
func (tb *TokenBucket) IsEnabled() bool {
	return tb.maxTokens > 0
}
