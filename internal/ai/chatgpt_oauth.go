package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

const chatGPTIssuer = "https://auth.openai.com"
const chatGPTResource = "https://api.openai.com/v1"
const chatGPTScopes = "openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"
const ChatGPTUsageURL = "https://chatgpt.com/settings/usage"

type chatGPTService struct {
	http   *http.Client
	issuer string
	api    string
	path   string
}

func newChatGPTService() (*chatGPTService, error) {
	path, err := chatGPTPath()
	if err != nil {
		return nil, err
	}
	return &chatGPTService{issuer: chatGPTIssuer, api: chatGPTResource, path: path, http: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("ChatGPT endpoint redirects are not allowed")
	}}}, nil
}

type chatGPTDiscovery struct {
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	Revocation    string `json:"revocation_endpoint"`
	JWKS          string `json:"jwks_uri"`
}

func (service *chatGPTService) discovery(ctx context.Context) (*oidc.Provider, chatGPTDiscovery, error) {
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, service.http), service.issuer)
	if err != nil {
		if ctx.Err() != nil {
			return nil, chatGPTDiscovery{}, ctx.Err()
		}
		return nil, chatGPTDiscovery{}, errors.New("cannot load ChatGPT sign-in settings; check your connection and retry")
	}
	var discovery chatGPTDiscovery
	if provider.Claims(&discovery) != nil {
		return nil, discovery, errors.New("invalid ChatGPT sign-in settings")
	}
	issuer, _ := url.Parse(service.issuer)
	for _, endpoint := range []string{discovery.Authorization, discovery.Token, discovery.Revocation, discovery.JWKS} {
		target, err := url.Parse(endpoint)
		if err != nil || target.Scheme != issuer.Scheme || target.Host != issuer.Host || target.User != nil || target.Fragment != "" {
			return nil, discovery, errors.New("untrusted ChatGPT authentication endpoint")
		}
	}
	return provider, discovery, nil
}

func randomOAuthValue() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", errors.New("cannot prepare secure ChatGPT sign-in")
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

type oauthCallback struct {
	code, clientID string
	err            error
}

func callbackHandler(state, clientID string, result chan<- oauthCallback) http.Handler {
	var once sync.Once
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if request.URL.Path != "/auth/callback" || request.Method != http.MethodGet {
			http.NotFound(writer, request)
			return
		}
		query := request.URL.Query()
		states := query["state"]
		if len(states) != 1 || subtle.ConstantTimeCompare([]byte(states[0]), []byte(state)) != 1 {
			http.Error(writer, "Invalid sign-in state. Return to LazySQL.", http.StatusBadRequest)
			return
		}
		outcome := oauthCallback{}
		if query.Get("error") != "" {
			outcome.err = errors.New("ChatGPT sign-in was declined or could not complete; try again when ready")
		} else if len(query["code"]) != 1 || query.Get("code") == "" {
			outcome.err = errors.New("ChatGPT sign-in did not return an authorization code")
		} else {
			issued := query.Get("client_id")
			if len(query["client_id"]) > 1 || (clientID != "" && issued != "" && issued != clientID) {
				outcome.err = errors.New("ChatGPT registration changed unexpectedly; sign-in was not saved")
			} else {
				if clientID != "" {
					issued = clientID
				}
				if !strings.HasPrefix(issued, "oaiapp_") || issued == "dynamic_agent_client" {
					outcome.err = errors.New("ChatGPT registration was incomplete; sign in again")
				} else {
					outcome.code = query.Get("code")
					outcome.clientID = issued
				}
			}
		}
		accepted := false
		once.Do(func() { result <- outcome; accepted = true })
		if !accepted {
			http.Error(writer, "This sign-in callback has already been used.", http.StatusConflict)
			return
		}
		if outcome.err != nil {
			http.Error(writer, "Sign-in could not complete. Return to LazySQL.", http.StatusBadRequest)
			return
		}
		io.WriteString(writer, "You can return to LazySQL to finish signing in.")
	})
}

type chatGPTTokenResponse struct {
	AccessToken       string          `json:"access_token"`
	RefreshToken      string          `json:"refresh_token"`
	IDToken           string          `json:"id_token"`
	TokenType         string          `json:"token_type"`
	ExpiresIn         int64           `json:"expires_in"`
	Scope             string          `json:"scope"`
	EarliestRefreshAt json.RawMessage `json:"earliest_refresh_at"`
}

func (service *chatGPTService) tokenRequest(ctx context.Context, endpoint string, form url.Values) (chatGPTTokenResponse, error) {
	var tokens chatGPTTokenResponse
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokens, errors.New("cannot prepare ChatGPT authorization request")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "lazysql/ai-sql")
	response, err := service.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return tokens, ctx.Err()
		}
		return tokens, errors.New("cannot connect to ChatGPT authorization; retry later")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
	if err != nil || len(data) > 128*1024 {
		return tokens, errors.New("invalid ChatGPT authorization response")
	}
	if response.StatusCode != http.StatusOK {
		return tokens, oauthRequestError(response.StatusCode, data)
	}
	if json.Unmarshal(data, &tokens) != nil || tokens.AccessToken == "" || !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.ExpiresIn <= 0 || tokens.ExpiresIn > 86400 {
		return tokens, errors.New("incomplete ChatGPT authorization response")
	}
	return tokens, nil
}

func (service *chatGPTService) validateIdentity(ctx context.Context, provider *oidc.Provider, clientID, raw, nonce string) (ChatGPTAccount, error) {
	token, err := provider.VerifierContext(oidc.ClientContext(ctx, service.http), &oidc.Config{ClientID: clientID, SupportedSigningAlgs: []string{oidc.RS256}}).Verify(oidc.ClientContext(ctx, service.http), raw)
	if err != nil || token.Subject == "" {
		return ChatGPTAccount{}, errors.New("ChatGPT identity could not be verified; sign-in was not saved")
	}
	if nonce != "" && subtle.ConstantTimeCompare([]byte(token.Nonce), []byte(nonce)) != 1 {
		return ChatGPTAccount{}, errors.New("ChatGPT sign-in nonce did not match; sign-in was not saved")
	}
	var claims struct {
		Email           string `json:"email"`
		AuthorizedParty string `json:"azp"`
	}
	if token.Claims(&claims) != nil || (len(token.Audience) > 1 && claims.AuthorizedParty != clientID) {
		return ChatGPTAccount{}, errors.New("invalid ChatGPT identity audience")
	}
	return ChatGPTAccount{ClientID: clientID, Subject: token.Subject, Email: claims.Email, Issuer: token.Issuer}, nil
}

func applyChatGPTTokens(account *ChatGPTAccount, tokens chatGPTTokenResponse) {
	account.AccessToken = tokens.AccessToken
	account.RefreshToken = tokens.RefreshToken
	if tokens.IDToken != "" {
		account.IDToken = tokens.IDToken
	}
	if tokens.Scope != "" {
		account.Scopes = strings.Fields(tokens.Scope)
	}
	account.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	account.EarliestRefreshAt = time.Time{}
	var unix int64
	if json.Unmarshal(tokens.EarliestRefreshAt, &unix) == nil && unix > 0 {
		account.EarliestRefreshAt = time.Unix(unix, 0)
	} else {
		var date string
		if json.Unmarshal(tokens.EarliestRefreshAt, &date) == nil {
			account.EarliestRefreshAt, _ = time.Parse(time.RFC3339, date)
		}
	}
}

func hasChatGPTPlan(account ChatGPTAccount) bool {
	invoke, direct := false, false
	for _, scope := range account.Scopes {
		if scope == "resource.invoke" {
			invoke = true
		}
		if scope == "chatgpt.tokens.use.direct" {
			direct = true
		}
	}
	return invoke && direct
}

type pendingChatGPTRegistration struct{ clientID string }

func (pending *pendingChatGPTRegistration) Error() string {
	return "ChatGPT authorization code expired; sign in again"
}

func (service *chatGPTService) login(parent context.Context, clientID string, present func(string) error) (bool, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	store, selected, err := service.prepareAccount(ctx, clientID)
	if err != nil {
		return false, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		welcome, err := service.loginAttempt(ctx, store, selected, clientID, present)
		var pending *pendingChatGPTRegistration
		if errors.As(err, &pending) && attempt == 0 {
			clientID = pending.clientID
			continue
		}
		return welcome, err
	}
	return false, errors.New("ChatGPT sign-in could not complete; try again")
}

func (service *chatGPTService) loginAttempt(ctx context.Context, store chatGPTStore, selected ChatGPTAccount, clientID string, present func(string) error) (bool, error) {
	provider, discovery, err := service.discovery(ctx)
	if err != nil {
		return false, err
	}
	state, err := randomOAuthValue()
	if err != nil {
		return false, err
	}
	nonce, err := randomOAuthValue()
	if err != nil {
		return false, err
	}
	verifier, err := randomOAuthValue()
	if err != nil {
		return false, err
	}
	challenge := sha256.Sum256([]byte(verifier))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return false, errors.New("cannot start local ChatGPT callback listener")
	}
	redirect := "http://" + listener.Addr().String() + "/auth/callback"
	result := make(chan oauthCallback, 1)
	server := &http.Server{Handler: callbackHandler(state, clientID, result), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, MaxHeaderBytes: 16 * 1024}
	defer server.Close()
	go server.Serve(listener)
	authorize, _ := url.Parse(discovery.Authorization)
	params := url.Values{"client_id": {"dynamic_agent_client"}, "agent_name_hint": {"LazySQL"}, "ext_agent_host_id": {store.HostID}, "response_type": {"code"}, "redirect_uri": {redirect}, "scope": {chatGPTScopes}, "resource": {chatGPTResource}, "state": {state}, "nonce": {nonce}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}}
	if clientID != "" {
		params.Set("client_id", clientID)
		params.Del("agent_name_hint")
		if selected.Email != "" {
			params.Set("login_hint", selected.Email)
		}
		if selected.AccessToken != "" && !hasChatGPTPlan(selected) {
			params.Set("prompt", "consent")
		}
	}
	authorize.RawQuery = params.Encode()
	if err := present(authorize.String()); err != nil {
		return false, err
	}
	var callback oauthCallback
	select {
	case callback = <-result:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	if callback.err != nil {
		return false, callback.err
	}
	tokens, err := service.tokenRequest(ctx, discovery.Token, url.Values{"grant_type": {"authorization_code"}, "client_id": {callback.clientID}, "code": {callback.code}, "code_verifier": {verifier}, "redirect_uri": {redirect}, "resource": {chatGPTResource}})
	if err != nil {
		var failure *chatGPTError
		if errors.As(err, &failure) && failure.Code == "invalid_grant" {
			return false, &pendingChatGPTRegistration{clientID: callback.clientID}
		}
		return false, err
	}
	if tokens.IDToken == "" {
		return false, errors.New("ChatGPT did not return an ID token; sign-in was not saved")
	}
	account, err := service.validateIdentity(ctx, provider, callback.clientID, tokens.IDToken, nonce)
	if err != nil {
		return false, err
	}
	if selected.ClientID != "" && (account.Subject != selected.Subject || account.Issuer != selected.Issuer) {
		return false, errors.New("ChatGPT account does not match the selected registration; add it as another account")
	}
	applyChatGPTTokens(&account, tokens)
	welcome := false
	err = withChatGPTStore(ctx, service.path, func(current *chatGPTStore) (bool, error) {
		existing := current.account(account.ClientID)
		if existing != nil {
			if existing.Subject != account.Subject || existing.Issuer != account.Issuer {
				return false, errors.New("ChatGPT registration identity mismatch; credentials were not replaced")
			}
			account.Welcomed = existing.Welcomed
			*existing = account
		} else {
			current.Accounts = append(current.Accounts, account)
		}
		current.Active = account.ClientID
		welcome = hasChatGPTPlan(account) && !account.Welcomed
		return true, nil
	})
	if err != nil {
		return false, err
	}
	if !hasChatGPTPlan(account) {
		return false, errors.New("Signed in, but ChatGPT plan use was not authorized. Run login again to grant plan access, or choose another provider. AI remains unchanged.")
	}
	return welcome, nil
}

func LoginChatGPT(ctx context.Context, clientID string, present func(string) error) (bool, error) {
	service, err := newChatGPTService()
	if err != nil {
		return false, err
	}
	return service.login(ctx, clientID, present)
}

func (service *chatGPTService) acknowledgePlan(ctx context.Context) error {
	return withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		account := store.account(store.Active)
		if account == nil {
			return false, errors.New("no active ChatGPT account")
		}
		account.Welcomed = true
		return true, nil
	})
}

func AcknowledgeChatGPTPlan(ctx context.Context) error {
	service, err := newChatGPTService()
	if err != nil {
		return err
	}
	return service.acknowledgePlan(ctx)
}
