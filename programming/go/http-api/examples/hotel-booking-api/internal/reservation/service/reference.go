package service

import (
	"crypto/rand"
)

const referenceLength = 10

// generateReference returns a random, human-friendly booking reference.
func generateReference() string {
	return rand.Text()[:referenceLength]
}
