package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

type ChatGPTAccount struct {
	ClientID          string    `json:"client_id"`
	Subject           string    `json:"subject"`
	Email             string    `json:"email,omitempty"`
	Issuer            string    `json:"issuer"`
	Active            bool      `json:"-"`
	AccessToken       string    `json:"access_token,omitempty"`
	RefreshToken      string    `json:"refresh_token,omitempty"`
	IDToken           string    `json:"id_token,omitempty"`
	Scopes            []string  `json:"scopes,omitempty"`
	ExpiresAt         time.Time `json:"expires_at,omitempty"`
	EarliestRefreshAt time.Time `json:"earliest_refresh_at,omitempty"`
	Welcomed          bool      `json:"welcomed,omitempty"`
}

type chatGPTStore struct {
	HostID   string           `json:"ext_agent_host_id"`
	Active   string           `json:"active_client_id,omitempty"`
	Accounts []ChatGPTAccount `json:"accounts"`
}

func chatGPTPath() (string, error) {
	key, err := OpenCodeCredentialPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(key), "chatgpt.json"), nil
}

func secureCredentialDirectory(path string) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return errors.New("cannot create private credential directory")
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid private credential directory")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return errors.New("credential directory must be private (chmod 700)")
	}
	return nil
}

func readChatGPTStore(path string) (chatGPTStore, error) {
	var store chatGPTStore
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return store, errors.New("invalid ChatGPT credential file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return store, errors.New("ChatGPT credential file must be private (chmod 600)")
	}
	file, err := os.Open(path)
	if err != nil {
		return store, errors.New("cannot read ChatGPT credentials")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 || json.Unmarshal(data, &store) != nil {
		return store, errors.New("invalid ChatGPT credentials; sign in again after removing the damaged credential file")
	}
	return store, nil
}

func writeChatGPTStore(path string, store chatGPTStore) error {
	data, err := json.Marshal(store)
	if err != nil {
		return errors.New("cannot encode ChatGPT credentials")
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".chatgpt-*")
	if err != nil {
		return errors.New("cannot create private ChatGPT credential file")
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return errors.New("cannot write ChatGPT credentials")
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return errors.New("cannot save ChatGPT credentials")
	}
	if err := file.Close(); err != nil {
		return errors.New("cannot close ChatGPT credential file")
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return errors.New("cannot replace ChatGPT credentials")
	}
	return nil
}

func withChatGPTStore(ctx context.Context, path string, update func(*chatGPTStore) (bool, error)) error {
	if err := secureCredentialDirectory(path); err != nil {
		return err
	}
	lockPath := path + ".lock"
	if info, err := os.Lstat(lockPath); err == nil && !info.Mode().IsRegular() {
		return errors.New("invalid ChatGPT credential lock")
	}
	lock := flock.New(lockPath, flock.SetPermissions(0600))
	locked, err := lock.TryLockContext(ctx, 25*time.Millisecond)
	if err != nil || !locked {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("cannot lock ChatGPT credentials")
	}
	defer lock.Close()
	store, err := readChatGPTStore(path)
	if err != nil {
		return err
	}
	changed, err := update(&store)
	if changed {
		if saveErr := writeChatGPTStore(path, store); saveErr != nil {
			return saveErr
		}
	}
	return err
}

func (store *chatGPTStore) account(clientID string) *ChatGPTAccount {
	for index := range store.Accounts {
		if store.Accounts[index].ClientID == clientID {
			return &store.Accounts[index]
		}
	}
	return nil
}

func clearChatGPTTokens(account *ChatGPTAccount) {
	account.AccessToken = ""
	account.RefreshToken = ""
	account.IDToken = ""
	account.Scopes = nil
	account.ExpiresAt = time.Time{}
	account.EarliestRefreshAt = time.Time{}
}

func (service *chatGPTService) accounts(ctx context.Context) ([]ChatGPTAccount, error) {
	var accounts []ChatGPTAccount
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		for _, account := range store.Accounts {
			account.Active = account.ClientID == store.Active
			clearChatGPTTokens(&account)
			accounts = append(accounts, account)
		}
		return false, nil
	})
	return accounts, err
}

func ChatGPTAccounts(ctx context.Context) ([]ChatGPTAccount, error) {
	service, err := newChatGPTService()
	if err != nil {
		return nil, err
	}
	return service.accounts(ctx)
}

func (service *chatGPTService) prepareAccount(ctx context.Context, clientID string) (chatGPTStore, ChatGPTAccount, error) {
	var snapshot chatGPTStore
	var selected ChatGPTAccount
	err := withChatGPTStore(ctx, service.path, func(store *chatGPTStore) (bool, error) {
		changed := false
		if store.HostID == "" {
			store.HostID = "urn:uuid:" + uuid.NewString()
			changed = true
		}
		if clientID != "" {
			account := store.account(clientID)
			if account == nil {
				return false, errors.New("ChatGPT account not found; choose an account again")
			}
			selected = *account
		}
		snapshot.HostID = store.HostID
		return changed, nil
	})
	return snapshot, selected, err
}
