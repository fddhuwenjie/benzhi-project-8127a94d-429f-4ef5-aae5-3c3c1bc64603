package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

var digestWorkspace bytes.Buffer

func Digest(value any) (string, error) {
	digestWorkspace.Reset()
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if _, err := digestWorkspace.Write(content); err != nil {
		return "", err
	}
	sum := sha256.Sum256(digestWorkspace.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

func DigestBytes(content []byte) string {
	digestWorkspace.Reset()
	_, _ = digestWorkspace.Write(content)
	sum := sha256.Sum256(digestWorkspace.Bytes())
	return hex.EncodeToString(sum[:])
}
