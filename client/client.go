package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/spy16/clockwork"
)

const alphaNumLetters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Store is responsible for providing persistence for clients.
type Store interface {
	Get(ctx context.Context, id string) (*Client, error)
	Put(ctx context.Context, cl Client) error
	Del(ctx context.Context, id string) error
}

// Client represents a client of the Scheduler system.
type Client struct {
	ID          string    `json:"id"`
	Secret      string    `json:"secret,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ChannelType string    `json:"channel_type"`
	ChannelName string    `json:"channel_name"`
}

// Validate ensures the Client definition is valid by sanitising and applying
// relevant checks. Returns ErrInvalid if any check fails.
func (cl *Client) Validate() error {
	cl.ID = strings.TrimSpace(cl.ID)
	cl.ChannelType = strings.TrimSpace(cl.ChannelType)
	cl.ChannelName = strings.TrimSpace(cl.ChannelName)

	if cl.ID == "" {
		return clockwork.ErrInvalid.WithCausef("id must not be empty")
	}

	if cl.Secret == "" {
		return clockwork.ErrInvalid.WithCausef("secret must not be empty")
	}

	if cl.ChannelType == "" {
		return clockwork.ErrInvalid.WithCausef("channel_type must not be empty")
	}

	if cl.ChannelName == "" {
		return clockwork.ErrInvalid.WithCausef("channel_name must not be empty")
	}

	if cl.CreatedAt.IsZero() {
		cl.CreatedAt = time.Now()
		cl.UpdatedAt = cl.CreatedAt
	}

	return nil
}

// Verify verifies the secret against the hashed secret in the client.
func (cl *Client) Verify(secret string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(cl.Secret), []byte(secret)); err != nil {
		return false
	}
	return true
}

// GenerateCreds generates random identifier and secret and returns the plain
// secret. Identifier is set only if it is currently empty. Secret field will
// be set to BCrypt hashed version.
func (cl *Client) GenerateCreds() (string, error) {
	cl.ID = strings.TrimSpace(cl.ID)
	if cl.ID == "" {
		cl.ID = randomString(10, []rune(alphaNumLetters))
	}

	plainSecret := cl.Secret
	if plainSecret == "" {
		plainSecret = randomString(16, []rune(alphaNumLetters))
	}

	hashedSecret, err := bcrypt.GenerateFromPassword([]byte(plainSecret), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	cl.Secret = string(hashedSecret)

	return plainSecret, nil
}

func randomString(n int, letters []rune) string {
	var bb bytes.Buffer
	bb.Grow(n)
	l := uint32(len(letters))
	// on each loop, generate one random rune and append to output
	for range n {
		bb.WriteRune(letters[binary.BigEndian.Uint32(randomBytes(4))%l])
	}
	return bb.String()
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

// Context returns a new context with cl value injected.
func Context(ctx context.Context, cl Client) context.Context {
	return context.WithValue(ctx, clientKey, cl)
}

// From returns the client value in the context if present. nil otherwise.
func From(ctx context.Context) *Client {
	v, ok := ctx.Value(clientKey).(Client)
	if !ok {
		return nil
	}
	return &v
}

type ctxKey struct{}

var clientKey = ctxKey{}
