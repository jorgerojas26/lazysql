package ai

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

func OpenCodeCredentialPath() (string, error) {
	directory := os.Getenv("XDG_CONFIG_HOME")
	if directory == "" {
		var err error
		directory, err = os.UserConfigDir()
		if err != nil {
			return "", errors.New("cannot locate user credential directory")
		}
	}
	return filepath.Join(directory, "lazysql", "credentials", "opencode-go"), nil
}

func validateOpenCodeKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || len(key) > 8192 {
		return "", errors.New("enter a nonempty OpenCode Go subscription API key")
	}
	for _, character := range key {
		if unicode.IsSpace(character) || unicode.IsControl(character) {
			return "", errors.New("the subscription key cannot contain whitespace or control characters")
		}
	}
	return key, nil
}

func SaveOpenCodeKey(key string) error {
	key, err := validateOpenCodeKey(key)
	if err != nil {
		return err
	}
	path, err := OpenCodeCredentialPath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return errors.New("cannot create private credential directory")
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("credential directory must be a real private directory")
	}
	if err := os.Chmod(directory, 0700); err != nil {
		return errors.New("cannot secure credential directory")
	}
	file, err := os.CreateTemp(directory, ".opencode-go-*")
	if err != nil {
		return errors.New("cannot create private credential file")
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(key); err != nil {
		file.Close()
		return errors.New("cannot write subscription credential")
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return errors.New("cannot save subscription credential")
	}
	if err := file.Close(); err != nil {
		return errors.New("cannot close subscription credential file")
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return errors.New("cannot install subscription credential")
	}
	return nil
}

func LoadOpenCodeKey() (string, error) {
	path, err := OpenCodeCredentialPath()
	if err != nil {
		return "", err
	}
	directory, err := os.Lstat(filepath.Dir(path))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil || !directory.IsDir() || directory.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("invalid private credential directory; run lazysql ai login")
	}
	if runtime.GOOS != "windows" && directory.Mode().Perm()&0077 != 0 {
		return "", errors.New("credential directory is not private; run lazysql ai login to secure it")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("invalid credential file; run lazysql ai login")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return "", errors.New("credential file is not private; run lazysql ai login")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("cannot read saved subscription credential")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 8193))
	if err != nil || len(data) > 8192 {
		return "", errors.New("invalid saved subscription credential; run lazysql ai login")
	}
	return validateOpenCodeKey(string(data))
}

func DeleteOpenCodeKey() error {
	path, err := OpenCodeCredentialPath()
	if err != nil {
		return err
	}
	directory, err := os.Lstat(filepath.Dir(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !directory.IsDir() || directory.Mode()&os.ModeSymlink != 0 {
		return errors.New("invalid private credential directory")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errors.New("cannot remove saved OpenCode Go credential")
	}
	return nil
}
