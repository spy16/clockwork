package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/spy16/clockwork"
	"github.com/spy16/clockwork/client"
)

func TestClient_Validate(t *testing.T) {
	t.Parallel()

	table := []struct {
		title   string
		client  client.Client
		wantErr error
	}{
		{
			title:   "EmptyID",
			client:  client.Client{},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "EmptySecret",
			client: client.Client{
				ID: "foo",
			},
			wantErr: clockwork.ErrInvalid,
		},
		{
			title: "Valid",
			client: client.Client{
				ID:          "foo",
				Secret:      "bar",
				ChannelName: "events",
				ChannelType: "kafka",
			},
			wantErr: nil,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			got := tt.client.Validate()
			if tt.wantErr != nil {
				assert.Error(t, got)
			} else {
				assert.NoError(t, got)
			}
		})
	}
}

func TestClient_Verify(t *testing.T) {
	t.Parallel()

	const plainTextSecret = "IQSt1rGGYKKcGge3"

	sampleClient := client.Client{
		ID:     "NVLHvK3n9R",
		Secret: "$2a$04$JoYF6BOA7eixSa7b0fW5.uABlGpjgbpUDqIehB.8aFbaYhVHq59lC",
	}

	table := []struct {
		title     string
		client    client.Client
		trySecret string
		want      bool
	}{
		{
			title:     "InvalidSecret",
			client:    sampleClient,
			trySecret: "foo",
			want:      false,
		},
		{
			title:     "ValidSecret",
			client:    sampleClient,
			trySecret: plainTextSecret,
			want:      true,
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			pass := tt.client.Verify(tt.trySecret)
			assert.Equal(t, tt.want, pass)
		})
	}
}

func TestClient_GenerateCreds(t *testing.T) {
	t.Parallel()

	table := []struct {
		title      string
		client     client.Client
		wantSecret string
		wantID     string
	}{
		{
			title: "BothCustom",
			client: client.Client{
				ID:     "foo",
				Secret: "foobar",
			},
			wantID:     "foo",
			wantSecret: "foobar",
		},
		{
			title: "AutoGenerateSecret",
			client: client.Client{
				ID: "foo",
			},
			wantID: "foo",
		},
		{
			title: "AutoGenerateID",
			client: client.Client{
				ID:     "",
				Secret: "foobar",
			},
			wantSecret: "foobar",
		},
		{
			title:  "AutoGenerateBoth",
			client: client.Client{},
		},
	}

	for _, tt := range table {
		t.Run(tt.title, func(t *testing.T) {
			secret, err := tt.client.GenerateCreds()
			assert.NoError(t, err)
			assert.NotEmpty(t, tt.client.ID)
			assert.NotEmpty(t, tt.client.Secret)

			if tt.wantSecret != "" {
				assert.Equal(t, tt.wantSecret, secret)
			}
			if tt.wantID != "" {
				assert.Equal(t, tt.wantID, tt.client.ID)
			}
		})
	}
}

func TestContext(t *testing.T) {
	original := client.Client{
		ID:          "foobar",
		ChannelType: "log",
		ChannelName: "hello",
	}

	baseCtx := context.Background()
	ctx := client.Context(baseCtx, original)

	assert.Nil(t, client.From(baseCtx))
	assert.Equal(t, &original, client.From(ctx))
}
