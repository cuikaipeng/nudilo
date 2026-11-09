package storage

// Like Kivrix, Nudilo keeps an encrypted vault and a local key in its app data
// directory. The security boundary is the current OS user's file permissions.
// No OS credential service is used by this store.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"callbell/internal/model"
)

var ErrCredentialsMissing = errors.New("No local credentials found. Save your configuration again.")

const maxCredentialsSize = 1024 * 1024
const credentialsContext = "nudilo-credentials-v1"

type LocalVault struct {
	paths Paths
	mu    sync.Mutex
}

type credentialsEnvelope struct {
	Version int    `json:"version"`
	Nonce   []byte `json:"nonce"`
	Data    []byte `json:"data"`
}

func NewLocalVault(paths Paths) *LocalVault { return &LocalVault{paths: paths} }
func (p Paths) Credentials() string         { return filepath.Join(p.Root, "credentials.vault") }
func (p Paths) CredentialsKey() string      { return filepath.Join(p.Root, "credentials.key") }

func readPrivate(path string, max int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("credential file is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err = f.Chmod(0600); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if int64(len(data)) > max {
		return nil, errors.New("credential file exceeds size limit")
	}
	return data, err
}

func (v *LocalVault) key(create bool) ([]byte, error) {
	key, err := readPrivate(v.paths.CredentialsKey(), 32)
	if errors.Is(err, os.ErrNotExist) && create {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		if err = os.MkdirAll(v.paths.Root, 0700); err != nil {
			return nil, err
		}
		f, openErr := os.OpenFile(v.paths.CredentialsKey(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(openErr, os.ErrExist) {
			return v.key(false)
		}
		if openErr != nil {
			return nil, openErr
		}
		_, err = f.Write(key)
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(v.paths.CredentialsKey())
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("invalid local credential key")
	}
	return key, nil
}

func credentialCipher(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (v *LocalVault) load() (map[model.Channel]model.Credentials, error) {
	data, err := readPrivate(v.paths.Credentials(), maxCredentialsSize)
	if errors.Is(err, os.ErrNotExist) {
		return map[model.Channel]model.Credentials{}, nil
	}
	if err != nil {
		return nil, err
	}
	var envelope credentialsEnvelope
	if err = json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if envelope.Version != 1 {
		return nil, errors.New("unsupported credential file version")
	}
	// Never replace a missing key for an existing vault: doing so would make
	// its saved credentials permanently unreadable.
	key, err := v.key(false)
	if err != nil {
		return nil, err
	}
	aead, err := credentialCipher(key)
	if err != nil {
		return nil, err
	}
	if len(envelope.Nonce) != aead.NonceSize() {
		return nil, errors.New("invalid credential nonce")
	}
	plain, err := aead.Open(nil, envelope.Nonce, envelope.Data, []byte(credentialsContext))
	if err != nil {
		return nil, err
	}
	defer clear(plain)
	var entries map[model.Channel]model.Credentials
	if err = json.Unmarshal(plain, &entries); err != nil {
		return nil, err
	}
	if entries == nil {
		return nil, errors.New("invalid credential data")
	}
	return entries, nil
}

func (v *LocalVault) save(entries map[model.Channel]model.Credentials) error {
	plain, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	defer clear(plain)
	if len(plain) > maxCredentialsSize/2 {
		return errors.New("credential data exceeds size limit")
	}
	key, err := v.key(true)
	if err != nil {
		return err
	}
	aead, err := credentialCipher(key)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	return WriteJSON(v.paths.Credentials(), credentialsEnvelope{Version: 1, Nonce: nonce, Data: aead.Seal(nil, nonce, plain, []byte(credentialsContext))})
}

func (v *LocalVault) Get(ch model.Channel) (model.Credentials, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return model.Credentials{}, errors.New("Cannot read local credentials. Check the data folder or restore its backup.")
	}
	c, ok := entries[ch]
	if !ok {
		return c, ErrCredentialsMissing
	}
	c.Normalize(ch)
	return c, nil
}

func (v *LocalVault) Set(ch model.Channel, c model.Credentials) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return errors.New("Cannot read local credentials. The original files were preserved.")
	}
	c.Normalize(ch)
	entries[ch] = c
	if err = v.save(entries); err != nil {
		return errors.New("Cannot save local credentials. Check folder permissions and free disk space.")
	}
	return nil
}

func (v *LocalVault) Delete(ch model.Channel) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	entries, err := v.load()
	if err != nil {
		return errors.New("Cannot read local credentials. The original files were preserved.")
	}
	if _, ok := entries[ch]; !ok {
		return nil
	}
	delete(entries, ch)
	if err = v.save(entries); err != nil {
		return errors.New("Cannot remove local credentials. Check folder permissions and free disk space.")
	}
	return nil
}
