package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

type KeyPair struct {
	KeyID      string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

type Service struct {
	keyDir string
}

func NewService(keyDir string) *Service {
	_ = os.MkdirAll(keyDir, 0o700)
	return &Service{keyDir: keyDir}
}

func (s *Service) GenerateKeyPair(keyID string) (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	kp := &KeyPair{
		KeyID:      keyID,
		PublicKey:  pub,
		PrivateKey: priv,
	}

	pubPath := filepath.Join(s.keyDir, keyID+".pub")
	privPath := filepath.Join(s.keyDir, keyID+".key")

	if err := os.WriteFile(pubPath, pub, 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(privPath, priv, 0o600); err != nil {
		return nil, err
	}

	return kp, nil
}

func (s *Service) LoadKeyPair(keyID string) (*KeyPair, error) {
	pubPath := filepath.Join(s.keyDir, keyID+".pub")
	privPath := filepath.Join(s.keyDir, keyID+".key")

	pub, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}

	priv, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	return &KeyPair{
		KeyID:      keyID,
		PublicKey:  pub,
		PrivateKey: priv,
	}, nil
}

func (s *Service) Sign(keyID string, data []byte) ([]byte, error) {
	kp, err := s.LoadKeyPair(keyID)
	if err != nil {
		return nil, err
	}
	return ed25519.Sign(kp.PrivateKey, data), nil
}

func (s *Service) Verify(publicKey, data, signature []byte) error {
	if !ed25519.Verify(publicKey, data, signature) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}

func (s *Service) ExportPublicKey(keyID string) (string, error) {
	kp, err := s.LoadKeyPair(keyID)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(kp.PublicKey), nil
}
