// Package infra provides infrastructure-level dependencies such as the
// default random source used by the usecase layer in production.
package infra

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// SystemRandom implements usecase.RandomSource using crypto/rand so ID
// generation and position-bias coin flips are unpredictable but the
// interface remains swappable for deterministic tests.
type SystemRandom struct{}

// Bool returns a cryptographically random boolean.
func (SystemRandom) Bool() bool {
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return true
	}
	return n.Int64() == 1
}

// ID returns a cryptographically random hex-encoded id.
func (SystemRandom) ID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Perm returns a cryptographically random permutation of [0,n). We could
// use math/rand here since review blinding isn't security-sensitive, but
// a Fisher-Yates shuffle over crypto/rand is cheap and keeps this package
// free of weak RNG warnings.
func (SystemRandom) Perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			// crypto/rand essentially never fails; if it does, returning
			// the identity permutation is a safe degraded fallback.
			return append([]int(nil), p...)
		}
		p[i], p[j.Int64()] = p[j.Int64()], p[i]
	}
	return p
}
