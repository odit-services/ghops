package services

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	// DefaultRSAKeyLength is the default length for RSA keys.
	DefaultRSAKeyLength = 4096 // 4096 bits
)

type SSHService interface {
	GenerateRSAKeyPair() (privateKey string, publicKey string, err error)
	GenerateED25519KeyPair() (privateKey string, publicKey string, err error)
}

type DefaultSSHService struct {
	RSAKeyLength int
}

func (s *DefaultSSHService) GenerateRSAKeyPair() (privateKey string, publicKey string, err error) {
	// generate private key
	privateKeyBase, err := rsa.GenerateKey(rand.Reader, s.RSAKeyLength)
	if err != nil {
		return "", "", err
	}

	// write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKeyBase)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKeyBase.PublicKey)
	if err != nil {
		return "", "", err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return pubKeyBuf.String(), privKeyBuf.String(), nil
}

func (s *DefaultSSHService) GenerateED25519KeyPair() (privateKey string, publicKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	p, err := ssh.MarshalPrivateKey(crypto.PrivateKey(priv), "")
	if err != nil {
		return "", "", err
	}
	privateKeyPem := pem.EncodeToMemory(p)
	privateKeyString := string(privateKeyPem)
	pubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", err
	}
	publicKeyString := "ssh-ed25519" + " " + base64.StdEncoding.EncodeToString(pubKey.Marshal())
	return privateKeyString, publicKeyString, nil
}
