package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Digest returns the SHA-256 of the JSON canonical form of value.
// Each call uses its own workspace so concurrent callers (for example,
// independent cases sealing in parallel) never observe cross-call
// contamination of the bytes being hashed.
func Digest(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// DigestBytes returns the SHA-256 of content using a per-call workspace,
// keeping concurrent fingerprint and manifest digest computations isolated.
func DigestBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
