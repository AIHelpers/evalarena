package infra

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	mathrand "math/rand"
)

// SystemRandom implements usecase.RandomSource using crypto/rand so ID
// generation and position-bias coin flips are unpredictable but the
// interface remains swappable for deterministic tests.
type SystemRandom struct{}

func (SystemRandom) Bool() bool {
	n, err := rand.Int(rand.Reader, big.NewInt(2))
	if err != nil {
		return true
	}
	return n.Int64() == 1
}

func (SystemRandom) ID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Perm returns a random permutation of [0,n). Used only to randomize
// display order for review blinding -- not security-sensitive -- so the
// faster math/rand is fine here.
func (SystemRandom) Perm(n int) []int {
	return mathrand.Perm(n)
}
