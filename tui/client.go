package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/passbolt/go-passbolt-cli/util"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
	"github.com/spf13/viper"
)

// sessionClient wraps the passbolt client for TUI lifetime management.
type sessionClient struct {
	client   *api.Client
	ctx      context.Context
	cancel   context.CancelFunc
	totpChan chan string // send TOTP code from TUI to MFA callback
	needMFA  bool       // true if interactive TOTP login is needed in-TUI
}

// newSessionClient creates the API client. For noninteractive or no-MFA modes
// it logs in immediately. For interactive-totp, it defers login to the TUI
// (so the TOTP input screen can drive it).
func newSessionClient() (*sessionClient, error) {
	serverAddress := viper.GetString("serverAddress")
	if serverAddress == "" {
		return nil, fmt.Errorf("serverAddress is not defined")
	}

	userPrivateKey := viper.GetString("userPrivateKey")
	if userPrivateKey == "" {
		return nil, fmt.Errorf("userPrivateKey is not defined")
	}

	// Read password before bubbletea takes over the terminal.
	userPassword := viper.GetString("userPassword")
	if userPassword == "" {
		var err error
		userPassword, err = util.ReadPassword("Enter Password:")
		if err != nil {
			fmt.Println()
			return nil, fmt.Errorf("Reading Password: %w", err)
		}
		fmt.Println()
	}

	httpClient, err := util.GetHttpClient()
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(httpClient, "", serverAddress, userPrivateKey, userPassword)
	if err != nil {
		return nil, fmt.Errorf("Creating Client: %w", err)
	}
	client.Debug = viper.GetBool("debug")

	// Server verification
	token := viper.GetString("serverVerifyToken")
	encToken := viper.GetString("serverVerifyEncToken")
	if token != "" {
		ctx, cancel := context.WithCancel(context.Background())
		err = client.VerifyServer(ctx, token, encToken)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("Verifying Server: %w", err)
		}
	}

	sc := &sessionClient{
		client:   client,
		totpChan: make(chan string, 1),
	}
	sc.ctx, sc.cancel = context.WithCancel(context.Background())

	mfaMode := viper.GetString("mfaMode")
	switch mfaMode {
	case "interactive-totp":
		// Set up MFA callback that blocks on a channel - the TUI drives this.
		sc.needMFA = true
		client.MFACallback = func(ctx context.Context, c *api.Client, res *api.APIResponse) (http.Cookie, error) {
			challenge := api.MFAChallenge{}
			err := json.Unmarshal(res.Body, &challenge)
			if err != nil {
				return http.Cookie{}, fmt.Errorf("Parsing MFA Challenge")
			}
			if challenge.Provider.TOTP == "" {
				return http.Cookie{}, fmt.Errorf("Server Provided no TOTP Provider")
			}

			for i := 0; i < 3; i++ {
				code := <-sc.totpChan
				req := api.MFAChallengeResponse{TOTP: code}
				raw, _, err := c.DoCustomRequestAndReturnRawResponse(ctx, "POST", "mfa/verify/totp.json", "v2", req, nil)
				if err != nil {
					if errors.Unwrap(err) != api.ErrAPIResponseErrorStatusCode {
						return http.Cookie{}, fmt.Errorf("Doing MFA Challenge Response: %w", err)
					}
					// Verification failed - TUI will show error and let user retry
					continue
				}
				for _, cookie := range raw.Cookies() {
					if cookie.Name == "passbolt_mfa" {
						return *cookie, nil
					}
				}
				return http.Cookie{}, fmt.Errorf("Unable to find Passbolt MFA Cookie")
			}
			return http.Cookie{}, fmt.Errorf("Failed MFA Challenge 3 times")
		}
		// Don't login yet - the TUI will call loginCmd after collecting TOTP.

	case "noninteractive-totp":
		totpToken := viper.GetString("mfaTotpToken")
		if totpToken == "" {
			totpToken = viper.GetString("totpToken")
		}
		totpOffset := viper.GetDuration("mfaTotpOffset")
		if totpOffset == 0 {
			totpOffset = viper.GetDuration("totpOffset")
		}
		helper.AddMFACallbackTOTP(client, viper.GetUint("mfaRetrys"), viper.GetDuration("mfaDelay"), totpOffset, totpToken)
		if err := client.Login(sc.ctx); err != nil {
			sc.cancel()
			return nil, fmt.Errorf("Logging in: %w", err)
		}

	default:
		// No MFA or mode "none"
		if err := client.Login(sc.ctx); err != nil {
			sc.cancel()
			return nil, fmt.Errorf("Logging in: %w", err)
		}
	}

	return sc, nil
}

func (sc *sessionClient) close() {
	util.SaveSessionKeysAndLogout(sc.ctx, sc.client)
	sc.cancel()
}
