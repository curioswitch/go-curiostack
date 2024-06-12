package testutil

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/identitytoolkit/v1"
	"google.golang.org/api/option"

	"github.com/curioswitch/go-curiostack/config"
)

// FirebaseIDToken returns a real ID token that can be used in e2e tests involving Firebase authentication.
// The token is created for the given userID, which must be specified. tenantID is optional, when unset,
// the user is assumed to be in the default tenant.
//
// This method will use the service account integration-test@<gcp project>.iam.gserviceaccount.com for
// issuing tokens and also set the quota project to the repo's configured GCP project.
func FirebaseIDToken(ctx context.Context, userID string, tenantID string, google *config.Google) (string, error) {
	fbApp, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID:        google.Project,
		ServiceAccountID: fmt.Sprintf("integration-test@%s.iam.gserviceaccount.com", google.Project),
	})
	if err != nil {
		return "", fmt.Errorf("curiostack/testutil: creating firebase app: %w", err)
	}

	fbAuth, err := fbApp.Auth(ctx)
	if err != nil {
		return "", fmt.Errorf("curiostack/testutil: getting firebase auth: %w", err)
	}

	var customToken string
	if tenantID != "" {
		tAuth, _ := fbAuth.TenantManager.AuthForTenant(tenantID)
		ct, err := tAuth.CustomToken(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("curiostack/testutil: creating custom token for tenant: %w", err)
		}
		customToken = ct
	} else {
		ct, err := fbAuth.CustomToken(ctx, userID)
		if err != nil {
			return "", fmt.Errorf("curiostack/testutil: creating custom token: %w", err)
		}
		customToken = ct
	}

	gcpIdentity, err := identitytoolkit.NewService(ctx, option.WithQuotaProject(google.Project))
	if err != nil {
		return "", fmt.Errorf("curiostack/testutil: creating identitytoolkit: %w", err)
	}

	res, err := gcpIdentity.Accounts.SignInWithCustomToken(&identitytoolkit.GoogleCloudIdentitytoolkitV1SignInWithCustomTokenRequest{
		Token:             customToken,
		TenantId:          tenantID,
		ReturnSecureToken: true,
	}).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("curiostack/testutil: signing in with custom token: %w", err)
	}

	return res.IdToken, nil
}
