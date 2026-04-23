package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/pkg/jwt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthStateStore persists and validates OAuth state tokens
type OAuthStateStore interface {
	SaveState(ctx context.Context, state string, expiration time.Duration) error
	ValidateState(ctx context.Context, state string) (bool, error)
}

// OAuthService handles OAuth 2.0 flows
type OAuthService struct {
	users       UserRepository
	refresh     RefreshTokenStore
	jwtManager  *jwt.Manager
	stateStore  OAuthStateStore
	googleCfg   *oauth2.Config
	githubCfg   *oauth2.Config
	accessTTL   time.Duration
	refreshTTL  time.Duration
}

// NewOAuthService creates a new OAuth service
func NewOAuthService(
	users UserRepository,
	refresh RefreshTokenStore,
	jwtMgr *jwt.Manager,
	stateStore OAuthStateStore,
	googleClientID, googleClientSecret, googleRedirectURL string,
	githubClientID, githubClientSecret, githubRedirectURL string,
	accessTTL, refreshTTL time.Duration,
) *OAuthService {
	return &OAuthService{
		users:      users,
		refresh:    refresh,
		jwtManager: jwtMgr,
		stateStore: stateStore,
		googleCfg: &oauth2.Config{
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			RedirectURL:  googleRedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		githubCfg: &oauth2.Config{
			ClientID:     githubClientID,
			ClientSecret: githubClientSecret,
			RedirectURL:  githubRedirectURL,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		},
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// GetGoogleAuthURL returns the URL to redirect the user to for Google OAuth
func (s *OAuthService) GetGoogleAuthURL(ctx context.Context) (string, error) {
	state := uuid.NewString()
	if err := s.stateStore.SaveState(ctx, state, 5*time.Minute); err != nil {
		return "", fmt.Errorf("save state: %w", err)
	}
	return s.googleCfg.AuthCodeURL(state), nil
}

// HandleGoogleCallback exchanges the code for a token and creates/logs in the user
func (s *OAuthService) HandleGoogleCallback(ctx context.Context, code, state string) (*TokenPair, *User, error) {
	valid, err := s.stateStore.ValidateState(ctx, state)
	if err != nil || !valid {
		return nil, nil, fmt.Errorf("invalid state")
	}

	token, err := s.googleCfg.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code: %w", err)
	}

	userInfo, err := s.fetchGoogleUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch user info: %w", err)
	}

	return s.findOrCreateOAuthUser(ctx, "google", userInfo.ID, userInfo.Email, userInfo.Name)
}

// GetGitHubAuthURL returns the URL to redirect the user to for GitHub OAuth
func (s *OAuthService) GetGitHubAuthURL(ctx context.Context) (string, error) {
	state := uuid.NewString()
	if err := s.stateStore.SaveState(ctx, state, 5*time.Minute); err != nil {
		return "", fmt.Errorf("save state: %w", err)
	}
	return s.githubCfg.AuthCodeURL(state), nil
}

// HandleGitHubCallback exchanges the code for a token and creates/logs in the user
func (s *OAuthService) HandleGitHubCallback(ctx context.Context, code, state string) (*TokenPair, *User, error) {
	valid, err := s.stateStore.ValidateState(ctx, state)
	if err != nil || !valid {
		return nil, nil, fmt.Errorf("invalid state")
	}

	token, err := s.githubCfg.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("exchange code: %w", err)
	}

	userInfo, err := s.fetchGitHubUserInfo(ctx, token.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch user info: %w", err)
	}

	return s.findOrCreateOAuthUser(ctx, "github", fmt.Sprintf("%d", userInfo.ID), userInfo.Email, userInfo.Name)
}

func (s *OAuthService) findOrCreateOAuthUser(ctx context.Context, provider, subject, email, name string) (*TokenPair, *User, error) {
	user, err := s.users.GetByOAuth(ctx, provider, subject)
	if err != nil {
		// Create new user
		now := time.Now().UTC()
		user = &User{
			ID:            uuid.New(),
			TenantID:      uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			Email:         email,
			Name:          name,
			Role:          "member",
			OAuthProvider: provider,
			OAuthSubject:  subject,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := s.users.Create(ctx, user); err != nil {
			return nil, nil, fmt.Errorf("create user: %w", err)
		}
	}

	pair, err := s.generateOAuthTokenPair(ctx, user)
	if err != nil {
		return nil, nil, err
	}

	return pair, user, nil
}

func (s *OAuthService) generateOAuthTokenPair(ctx context.Context, user *User) (*TokenPair, error) {
	accessToken, err := s.jwtManager.GenerateToken(user.ID.String(), user.TenantID.String(), user.Role, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken := uuid.NewString()
	if err := s.refresh.Save(ctx, user.ID.String(), refreshToken, s.refreshTTL); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

// googleUserInfo represents the response from Google's userinfo endpoint
type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (s *OAuthService) fetchGoogleUserInfo(ctx context.Context, accessToken string) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google API returned %d: %s", resp.StatusCode, body)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// githubUserInfo represents the response from GitHub's user endpoint
type githubUserInfo struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// githubEmail represents an email from GitHub's emails endpoint
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func (s *OAuthService) fetchGitHubUserInfo(ctx context.Context, accessToken string) (*githubUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned %d: %s", resp.StatusCode, body)
	}

	var info githubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}

	// GitHub may not return email in /user if it's private; fetch from /user/emails
	if info.Email == "" {
		email, err := s.fetchGitHubPrimaryEmail(ctx, accessToken)
		if err == nil {
			info.Email = email
		}
	}

	if info.Name == "" {
		info.Name = info.Login
	}

	return &info, nil
}

func (s *OAuthService) fetchGitHubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails API returned %d", resp.StatusCode)
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", fmt.Errorf("no email found")
}
