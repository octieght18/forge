package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/octieght18/forge/internal/store"
)

var errCursor = errors.New("invalid cursor")

type cursors struct {
	key []byte
	now func() time.Time
}
type cursor struct {
	Caller, Collection, Parent, ID string
	At                             time.Time
	Number                         int64
	Expires                        int64
}

func caller(p store.Principal) string {
	b, _ := json.Marshal(p)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func (c cursors) encode(p store.Principal, value cursor) string {
	value.Caller = caller(p)
	value.Expires = c.now().Add(15 * time.Minute).Unix()
	b, _ := json.Marshal(value)
	mac := hmac.New(sha256.New, c.key)
	mac.Write(b)
	return base64.RawURLEncoding.EncodeToString(append(b, mac.Sum(nil)...))
}
func (c cursors) decode(p store.Principal, collection, parent, encoded string) (cursor, error) {
	var value cursor
	if len(encoded) > 2048 {
		return value, errCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(decoded) <= sha256.Size {
		return value, errCursor
	}
	body, sig := decoded[:len(decoded)-sha256.Size], decoded[len(decoded)-sha256.Size:]
	mac := hmac.New(sha256.New, c.key)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return value, errCursor
	}
	if json.Unmarshal(body, &value) != nil || value.Caller != caller(p) || value.Collection != collection || value.Parent != parent || value.Expires <= c.now().Unix() || value.Expires > c.now().Add(15*time.Minute).Unix() {
		return cursor{}, errCursor
	}
	return value, nil
}
