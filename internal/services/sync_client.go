package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/teslamate"
)

// formatTeslaMateError enriches error messages with actionable troubleshooting hints.
func formatTeslaMateError(err error, rawURL string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	isDockerLocalhost := strings.Contains(rawURL, "localhost") || strings.Contains(rawURL, "127.0.0.1")
	if isDockerLocalhost && (strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp")) {
		return apierror.Newf("teslamate.docker_localhost", "%s (Note: inside Docker, 'localhost' is the AutoLedger container itself. Use 'http://host.docker.internal:PORT' or your machine's local IP)", msg)
	}
	if strings.Contains(msg, "Client.Timeout exceeded") || strings.Contains(msg, "context deadline exceeded") {
		return apierror.Newf("teslamate.timeout", "%s (Timed out: check that the address and port are reachable and that TeslaMate is responding)", msg)
	}
	return err
}

// TestConnection verifies if connection to TeslaMate works for a given vehicle config.
func (s *SyncService) TestConnection(ctx context.Context, v *models.Vehicle) (*teslamate.StatusDetails, error) {
	client, err := s.buildClient(v)
	if err != nil {
		return nil, err
	}

	carID := 1
	if v.TeslaMateCarID != nil && *v.TeslaMateCarID > 0 {
		carID = *v.TeslaMateCarID
	}

	status, _, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		rawURL := ""
		if v.TeslaMateAPIURL != nil {
			rawURL = *v.TeslaMateAPIURL
		}
		return nil, formatTeslaMateError(err, rawURL)
	}

	return status, nil
}

// TestConnectionRaw verifies if connection to TeslaMate works using raw credentials without existing vehicle.
func (s *SyncService) TestConnectionRaw(ctx context.Context, apiURL string, authType models.AuthMode, apiKey, basicUser, basicPass string, carID int) (*teslamate.StatusDetails, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return nil, fmt.Errorf("l'URL de l'API TeslaMate est requise")
	}

	cfg := teslamate.Config{
		BaseURL:  apiURL,
		AuthType: teslamate.AuthType(authType),
		Timeout:  15 * time.Second,
	}

	switch cfg.AuthType {
	case teslamate.AuthBearer:
		cfg.APIToken = apiKey
	case teslamate.AuthBasic:
		cfg.Username = basicUser
		cfg.Password = basicPass
	}

	client, err := teslamate.NewClient(cfg)
	if err != nil {
		return nil, formatTeslaMateError(err, apiURL)
	}

	if carID <= 0 {
		carID = 1
	}

	status, _, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		return nil, formatTeslaMateError(err, apiURL)
	}

	return status, nil
}

func (s *SyncService) buildClient(v *models.Vehicle) (*teslamate.Client, error) {
	return buildTeslaMateClient(v, s.encryptor)
}

// buildTeslaMateClient constructs an authenticated TeslaMateAPI client for a vehicle,
// decrypting whichever credential its configured auth type requires. Shared by SyncService
// and any other service that needs to call TeslaMateAPI on a vehicle's behalf.
func buildTeslaMateClient(v *models.Vehicle, encryptor *crypto.Encryptor) (*teslamate.Client, error) {
	if v.TeslaMateAPIURL == nil || *v.TeslaMateAPIURL == "" {
		return nil, fmt.Errorf("no TeslaMate API URL provided")
	}

	cfg := teslamate.Config{
		BaseURL:  *v.TeslaMateAPIURL,
		AuthType: teslamate.AuthType(v.TeslaMateAuthType),
		Timeout:  60 * time.Second,
	}

	switch cfg.AuthType {
	case teslamate.AuthBearer:
		if v.TeslaMateAPIKeyEncrypted != nil && *v.TeslaMateAPIKeyEncrypted != "" {
			token, err := encryptor.Decrypt(*v.TeslaMateAPIKeyEncrypted)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt API key: %w", err)
			}
			cfg.APIToken = token
		}
	case teslamate.AuthBasic:
		if v.TeslaMateBasicUser != nil {
			cfg.Username = *v.TeslaMateBasicUser
		}
		if v.TeslaMateBasicPassEnc != nil && *v.TeslaMateBasicPassEnc != "" {
			pass, err := encryptor.Decrypt(*v.TeslaMateBasicPassEnc)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt basic password: %w", err)
			}
			cfg.Password = pass
		}
	}

	return teslamate.NewClient(cfg)
}
