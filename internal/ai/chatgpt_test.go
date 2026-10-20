package ai

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type chatGPTFixture struct {
	service       *chatGPTService
	server        *httptest.Server
	key           *rsa.PrivateKey
	nonce         string
	issuer        string
	subject       string
	audience      string
	scope         string
	clientID      string
	codeChallenge string
	redirect      string
	hostID        string
	refreshCode   string
	refreshStatus int
	tokenScope    string
	tokenNonce    string
	revokeStatus  int
	stream        string
	refreshes     atomic.Int32
	revokes       atomic.Int32
	requests      atomic.Int32
}

func newChatGPTFixture(t *testing.T) *chatGPTFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &chatGPTFixture{key: key, subject: "subject-one", scope: chatGPTScopes, clientID: "oaiapp_fixture", revokeStatus: 200}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.handle))
	t.Cleanup(fixture.server.Close)
	fixture.issuer = fixture.server.URL
	fixture.service = &chatGPTService{issuer: fixture.server.URL, api: fixture.server.URL + "/v1", path: filepath.Join(t.TempDir(), "credentials", "chatgpt.json"), http: &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect refused") }}}
	fixture.stream = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"SELECT 1;\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"
	return fixture
}

func (fixture *chatGPTFixture) sign(claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "fixture-key"})
	payload, _ := json.Marshal(claims)
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, fixture.key, crypto.SHA256, digest[:])
	if err != nil {
		panic(err)
	}
	return message + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (fixture *chatGPTFixture) handle(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/json")
	switch request.URL.Path {
	case "/.well-known/openid-configuration":
		json.NewEncoder(writer).Encode(map[string]any{"issuer": fixture.issuer, "authorization_endpoint": fixture.server.URL + "/authorize", "token_endpoint": fixture.server.URL + "/token", "revocation_endpoint": fixture.server.URL + "/revoke", "jwks_uri": fixture.server.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	case "/jwks":
		json.NewEncoder(writer).Encode(map[string]any{"keys": []map[string]string{{"kty": "RSA", "kid": "fixture-key", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(fixture.key.N.Bytes()), "e": "AQAB"}}})
	case "/token":
		request.ParseForm()
		if request.Form.Get("client_id") != fixture.clientID || request.Form.Get("resource") != chatGPTResource || request.Form.Get("client_secret") != "" {
			writer.WriteHeader(400)
			io.WriteString(writer, `{"error":"invalid_client"}`)
			return
		}
		scope := fixture.scope
		if request.Form.Get("grant_type") == "refresh_token" {
			fixture.refreshes.Add(1)
			if fixture.refreshStatus != 0 {
				writer.WriteHeader(fixture.refreshStatus)
				json.NewEncoder(writer).Encode(map[string]string{"error": fixture.refreshCode})
				return
			}
			if request.Form.Get("refresh_token") != "refresh-one" || request.Form.Has("scope") {
				writer.WriteHeader(400)
				return
			}
			if fixture.tokenScope != "" {
				scope = fixture.tokenScope
			}
			json.NewEncoder(writer).Encode(map[string]any{"access_token": "access-refreshed", "refresh_token": "refresh-two", "token_type": "Bearer", "expires_in": 3600, "scope": scope})
			return
		}
		hash := sha256.Sum256([]byte(request.Form.Get("code_verifier")))
		if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "single-use-code" || request.Form.Get("redirect_uri") != fixture.redirect || base64.RawURLEncoding.EncodeToString(hash[:]) != fixture.codeChallenge {
			writer.WriteHeader(400)
			io.WriteString(writer, `{"error":"invalid_grant"}`)
			return
		}
		nonce := fixture.nonce
		if fixture.tokenNonce != "" {
			nonce = fixture.tokenNonce
		}
		audience := fixture.clientID
		if fixture.audience != "" {
			audience = fixture.audience
		}
		identity := fixture.sign(map[string]any{"iss": fixture.issuer, "sub": fixture.subject, "aud": audience, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "email": "user@example.com"})
		json.NewEncoder(writer).Encode(map[string]any{"access_token": "access-one", "refresh_token": "refresh-one", "id_token": identity, "token_type": "Bearer", "expires_in": 3600, "scope": scope})
	case "/revoke":
		fixture.revokes.Add(1)
		request.ParseForm()
		if request.Form.Get("token") == "" || request.Form.Get("token_type_hint") != "refresh_token" || request.Form.Get("client_id") != fixture.clientID {
			writer.WriteHeader(400)
			return
		}
		writer.WriteHeader(fixture.revokeStatus)
	case "/v1/models":
		fixture.requests.Add(1)
		if request.Header.Get("Authorization") == "" {
			writer.WriteHeader(401)
			return
		}
		io.WriteString(writer, `{"models":[{"slug":"account-first","display_name":"First Model","visibility":"list"},{"slug":"hidden","visibility":"hidden"},{"slug":"account-second","display_name":"Second Model","visibility":"list"},{"slug":"account-first","visibility":"list"}]}`)
	case "/v1/responses":
		fixture.requests.Add(1)
		var body map[string]any
		json.NewDecoder(request.Body).Decode(&body)
		if body["stream"] != true || body["store"] != false || body["model"] != "account-first" || body["max_output_tokens"] != nil || body["temperature"] != nil || body["tools"] != nil {
			writer.WriteHeader(400)
			return
		}
		if _, ok := body["input"].([]any); !ok {
			writer.WriteHeader(400)
			return
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(writer, fixture.stream)
	default:
		writer.WriteHeader(404)
	}
}

func (fixture *chatGPTFixture) present(t *testing.T, returning bool) func(string) error {
	t.Helper()
	return func(address string) error {
		target, err := url.Parse(address)
		if err != nil {
			return err
		}
		query := target.Query()
		if query.Get("ext_agent_host_id") == "" || query.Get("state") == "" || query.Get("nonce") == "" || query.Get("code_challenge_method") != "S256" || query.Get("scope") != chatGPTScopes || query.Get("resource") != chatGPTResource {
			t.Error("authorization request omitted required values")
		}
		if returning {
			if query.Get("client_id") != fixture.clientID || query.Get("agent_name_hint") != "" {
				t.Error("returning sign-in changed registration")
			}
		} else if query.Get("client_id") != "dynamic_agent_client" || query.Get("agent_name_hint") != "LazySQL" {
			t.Error("new registration did not use public dynamic registration")
		}
		if query.Get("id_token_hint") != "" {
			t.Error("manual authorization URL exposes retained identity token")
		}
		fixture.nonce = query.Get("nonce")
		fixture.codeChallenge = query.Get("code_challenge")
		fixture.redirect = query.Get("redirect_uri")
		fixture.hostID = query.Get("ext_agent_host_id")
		callback, err := url.Parse(fixture.redirect)
		if err != nil {
			return err
		}
		if callback.Hostname() != "127.0.0.1" || callback.Path != "/auth/callback" {
			t.Error("callback is not pinned to IPv4 loopback")
		}
		callback.RawQuery = url.Values{"state": {query.Get("state")}, "code": {"single-use-code"}, "client_id": {fixture.clientID}}.Encode()
		response, err := http.Get(callback.String())
		if err != nil {
			return err
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Errorf("callback status=%d", response.StatusCode)
		}
		return nil
	}
}

func TestNativeChatGPTLoginModelsInferenceAndIndependentLogout(t *testing.T) {
	fixture := newChatGPTFixture(t)
	welcome, err := fixture.service.login(context.Background(), "", fixture.present(t, false))
	if err != nil || !welcome {
		t.Fatalf("native OAuth login=%v %v", welcome, err)
	}
	store, err := readChatGPTStore(fixture.service.path)
	if err != nil || store.Active != fixture.clientID || len(store.Accounts) != 1 || !hasChatGPTPlan(store.Accounts[0]) {
		t.Fatalf("credentials=%+v err=%v", store, err)
	}
	host := store.HostID
	accounts, err := fixture.service.accounts(context.Background())
	if err != nil || !accounts[0].Active || accounts[0].AccessToken != "" || accounts[0].RefreshToken != "" || accounts[0].IDToken != "" {
		t.Fatal("account picker exposed credentials")
	}
	models, err := fixture.service.models(context.Background())
	if err != nil || len(models) != 2 || models[0].ID != "account-first" || models[1].ID != "account-second" {
		t.Fatalf("model list=%v err=%v", models, err)
	}
	output, err := (&chatGPTClient{model: models[0].ID, service: fixture.service}).Complete(context.Background(), "Generate SQL")
	if err != nil || output != "SELECT 1;" {
		t.Fatalf("inference=%q %v", output, err)
	}
	if err := fixture.service.logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	store, err = readChatGPTStore(fixture.service.path)
	if err != nil || store.HostID != host || store.Active != "" || store.Accounts[0].AccessToken != "" || store.Accounts[0].RefreshToken != "" || store.Accounts[0].IDToken != "" || store.Accounts[0].ClientID != fixture.clientID {
		t.Fatal("logout failed to clear tokens or deleted stable registration/host")
	}
	if fixture.revokes.Load() != 1 {
		t.Fatal("logout did not revoke the renewable session")
	}
	if _, err := fixture.service.activeToken(context.Background()); err == nil {
		t.Fatal("logged out session remains usable")
	}
	if _, err := fixture.service.login(context.Background(), fixture.clientID, fixture.present(t, true)); err != nil {
		t.Fatal(err)
	}
	if fixture.hostID != host {
		t.Fatal("reauthorization changed stable host identity")
	}
}

func TestNativeChatGPTRejectsInvalidIdentityAndDoesNotReplaceActiveCredentials(t *testing.T) {
	for _, test := range []struct{ name, nonce, audience string }{
		{name: "nonce", nonce: "wrong-nonce"}, {name: "audience", audience: "oaiapp_attacker"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newChatGPTFixture(t)
			fixture.tokenNonce = test.nonce
			fixture.audience = test.audience
			if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err == nil {
				t.Fatal("invalid identity accepted")
			}
			store, err := readChatGPTStore(fixture.service.path)
			if err != nil || len(store.Accounts) != 0 || store.Active != "" {
				t.Fatal("invalid OAuth result persisted tokens")
			}
		})
	}
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(fixture.service.path)
	fixture.subject = "different-subject"
	if _, err := fixture.service.login(context.Background(), fixture.clientID, fixture.present(t, true)); err == nil {
		t.Fatal("returning sign-in switched account identity")
	}
	after, _ := os.ReadFile(fixture.service.path)
	if !bytesEqual(before, after) {
		t.Fatal("failed login overwrote active credentials")
	}
}

func bytesEqual(left, right []byte) bool { return string(left) == string(right) }

func TestChatGPTConsentWithoutPlanNeverAllowsInference(t *testing.T) {
	fixture := newChatGPTFixture(t)
	fixture.scope = "openid profile email offline_access"
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err == nil {
		t.Fatal("missing plan permission was not surfaced")
	}
	accounts, err := fixture.service.accounts(context.Background())
	if err != nil || len(accounts) != 1 {
		t.Fatal("identity sign-in should be retained when plan permission is missing")
	}
	if _, err := fixture.service.models(context.Background()); err == nil || fixture.requests.Load() != 0 {
		t.Fatal("missing plan scope reached inference/model endpoints")
	}
}

func TestChatGPTRefreshIsSerializedAndRotatingCredentialsAreSavedTogether(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	if err := withChatGPTStore(context.Background(), fixture.service.path, func(store *chatGPTStore) (bool, error) {
		store.Accounts[0].ExpiresAt = time.Now().Add(-time.Minute)
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			token, err := fixture.service.activeToken(context.Background())
			if err != nil {
				failures <- err
			} else if token != "access-refreshed" {
				failures <- fmt.Errorf("stale token returned")
			}
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	store, err := readChatGPTStore(fixture.service.path)
	if err != nil || fixture.refreshes.Load() != 1 || store.Accounts[0].RefreshToken != "refresh-two" || store.Accounts[0].AccessToken != "access-refreshed" {
		t.Fatal("refreshes raced or rotation was not saved atomically")
	}
}

func TestChatGPTTemporaryRefreshErrorsPreserveTokensAndTerminalErrorsClearThem(t *testing.T) {
	for _, code := range []string{"invalid_grant", "server_error"} {
		fixture := newChatGPTFixture(t)
		if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
			t.Fatal(err)
		}
		fixture.refreshCode = code
		fixture.refreshStatus = 500
		if code == "invalid_grant" {
			fixture.refreshStatus = 400
		}
		if err := withChatGPTStore(context.Background(), fixture.service.path, func(store *chatGPTStore) (bool, error) {
			store.Accounts[0].ExpiresAt = time.Now().Add(-time.Minute)
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.service.activeToken(context.Background()); err == nil {
			t.Fatal("refresh failure was ignored")
		}
		store, err := readChatGPTStore(fixture.service.path)
		if err != nil {
			t.Fatal(err)
		}
		cleared := store.Accounts[0].RefreshToken == ""
		if cleared != (code == "invalid_grant") {
			t.Fatal("temporary refresh failure erased tokens or terminal failure retained tokens")
		}
	}
}

func TestChatGPTLogoutClearsLocallyWhenRemoteRevocationCannotBeConfirmed(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	fixture.revokeStatus = 503
	err := fixture.service.logout(context.Background())
	if err == nil || !strings.Contains(err.Error(), "Remote revocation was not confirmed") {
		t.Fatalf("unconfirmed logout=%v", err)
	}
	store, _ := readChatGPTStore(fixture.service.path)
	if store.Active != "" || store.Accounts[0].RefreshToken != "" || fixture.revokes.Load() != 2 {
		t.Fatal("unconfirmed logout did not clear local tokens or retry revocation")
	}
}

func TestChatGPTCallbackRejectsStateMismatchDuplicatesAndChangedClient(t *testing.T) {
	for _, query := range []string{"state=wrong&code=code&client_id=oaiapp_selected", "state=state&state=state&code=code&client_id=oaiapp_selected"} {
		result := make(chan oauthCallback, 1)
		handler := callbackHandler("state", "oaiapp_selected", result)
		writer := httptest.NewRecorder()
		handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/auth/callback?"+query, nil))
		if writer.Code != 400 || len(result) != 0 {
			t.Fatal("invalid state consumed the sign-in attempt")
		}
	}
	result := make(chan oauthCallback, 1)
	handler := callbackHandler("state", "oaiapp_selected", result)
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/auth/callback?state=state&code=code&client_id=oaiapp_other", nil))
	if (<-result).err == nil {
		t.Fatal("callback replaced the selected registration")
	}
	writer = httptest.NewRecorder()
	handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/auth/callback?state=state&code=code&client_id=oaiapp_selected", nil))
	if writer.Code != 409 {
		t.Fatal("OAuth callback replay accepted")
	}
}

func TestChatGPTIdentityVerifierRejectsExpiryIssuerAndSignature(t *testing.T) {
	fixture := newChatGPTFixture(t)
	provider, _, err := fixture.service.discovery(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"expired", "issuer", "signature"} {
		claims := map[string]any{"iss": fixture.issuer, "sub": "subject", "aud": fixture.clientID, "exp": time.Now().Add(time.Hour).Unix(), "nonce": "nonce"}
		if variant == "expired" {
			claims["exp"] = time.Now().Add(-time.Hour).Unix()
		}
		if variant == "issuer" {
			claims["iss"] = "https://untrusted.invalid"
		}
		raw := fixture.sign(claims)
		if variant == "signature" {
			parts := strings.Split(raw, ".")
			parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, 256))
			raw = strings.Join(parts, ".")
		}
		if _, err := fixture.service.validateIdentity(context.Background(), provider, fixture.clientID, raw, "nonce"); err == nil {
			t.Fatalf("%s ID token accepted", variant)
		}
	}
}

func TestChatGPTOAuthCancellationLeavesExistingCredentialsUntouched(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(fixture.service.path)
	ctx, cancel := context.WithCancel(context.Background())
	var callbackURL string
	_, err := fixture.service.login(ctx, fixture.clientID, func(address string) error {
		target, _ := url.Parse(address)
		callbackURL = target.Query().Get("redirect_uri")
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled browser login=%v", err)
	}
	after, _ := os.ReadFile(fixture.service.path)
	if !bytesEqual(before, after) {
		t.Fatal("cancelled sign-in replaced active credentials")
	}
	client := &http.Client{Timeout: time.Second}
	if response, err := client.Get(callbackURL); err == nil {
		response.Body.Close()
		t.Fatal("cancelled OAuth callback listener remains open")
	}
}

func TestChatGPTMultipleRegistrationsDoNotShareTokens(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	firstID := fixture.clientID
	fixture.clientID = "oaiapp_second"
	fixture.subject = "second-subject"
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	store, err := readChatGPTStore(fixture.service.path)
	if err != nil || len(store.Accounts) != 2 || store.Active != "oaiapp_second" || store.Accounts[0].Subject == store.Accounts[1].Subject {
		t.Fatal("adding account overwrote a different registration")
	}
	if err := fixture.service.logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	store, _ = readChatGPTStore(fixture.service.path)
	if store.account(firstID).RefreshToken == "" || store.account("oaiapp_second").RefreshToken != "" {
		t.Fatal("logout touched another account's tokens")
	}
}

func TestChatGPTPlanWelcomeIsAcknowledgedOnlyOncePerRegistration(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if welcome, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil || !welcome {
		t.Fatal("first plan authorization must show a welcome")
	}
	if err := fixture.service.acknowledgePlan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if welcome, err := fixture.service.login(context.Background(), fixture.clientID, fixture.present(t, true)); err != nil || welcome {
		t.Fatal("reauthorization repeated the acknowledged welcome")
	}
	if err := fixture.service.logout(context.Background()); err != nil {
		t.Fatal(err)
	}
	if welcome, err := fixture.service.login(context.Background(), fixture.clientID, fixture.present(t, true)); err != nil || welcome {
		t.Fatal("logout forgot that the plan welcome was acknowledged")
	}
}

func TestChatGPTCredentialFilesUseOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission checks")
	}
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{fixture.service.path, 0600}, {fixture.service.path + ".lock", 0600}, {filepath.Dir(fixture.service.path), 0700}} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() != item.mode {
			t.Fatalf("private credential permissions were not enforced for %s", filepath.Base(item.path))
		}
	}
	if err := os.Chmod(fixture.service.path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.activeToken(context.Background()); err == nil {
		t.Fatal("world-readable token file accepted")
	}
}
