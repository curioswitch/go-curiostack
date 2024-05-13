package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/curioswitch/go-curiostack/config"
)

func TestFirebaseIDToken(t *testing.T) {
	if os.Getenv("E2E") == "" {
		t.Skip("skipping e2e test")
	}

	tests := []struct {
		name     string
		userID   string
		tenantID string
	}{
		{
			name:     "default tenant",
			userID:   "TX4qUr43tVfF3KcETebGdE9u2Q03",
			tenantID: "",
		},
		{
			name:     "tenant",
			userID:   "oQbBmForw6QbH0G3ZXU9SsAKaex2",
			tenantID: "e2e-test-b1kyr",
		},
	}

	var conf config.Common
	require.NoError(t, config.Load(&conf, nil))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			token, err := FirebaseIDToken(ctx, tc.userID, tc.tenantID, conf.Google)
			require.NoError(t, err)
			require.NotEmpty(t, token)
		})
	}
}
