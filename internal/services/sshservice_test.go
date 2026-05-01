package services

import (
	"crypto/rsa"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"golang.org/x/crypto/ssh"
)

var _ = Describe("DefaultSSHService", func() {
	Describe("GenerateRSAKeyPair", func() {
		It("generates non-empty private and public keys", func() {
			svc := &DefaultSSHService{RSAKeyLength: 2048}
			privateKey, publicKey, err := svc.GenerateRSAKeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(privateKey).NotTo(BeEmpty())
			Expect(publicKey).NotTo(BeEmpty())
		})

		It("returns a valid PEM-encoded RSA private key", func() {
			svc := &DefaultSSHService{RSAKeyLength: 2048}
			privateKey, _, err := svc.GenerateRSAKeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Contains(privateKey, "-----BEGIN RSA PRIVATE KEY-----")).To(BeTrue())
			Expect(strings.Contains(privateKey, "-----END RSA PRIVATE KEY-----")).To(BeTrue())
		})

		It("returns a public key with ssh-rsa prefix", func() {
			svc := &DefaultSSHService{RSAKeyLength: 2048}
			_, publicKey, err := svc.GenerateRSAKeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(publicKey).To(HavePrefix("ssh-rsa "))
		})

		It("returns keys that can be parsed back and are usable", func() {
			svc := &DefaultSSHService{RSAKeyLength: 2048}
			privateKey, publicKey, err := svc.GenerateRSAKeyPair()

			Expect(err).NotTo(HaveOccurred())

			parsedPrivateKey, err := ssh.ParseRawPrivateKey([]byte(privateKey))
			Expect(err).NotTo(HaveOccurred())

			rsaKey, ok := parsedPrivateKey.(*rsa.PrivateKey)
			Expect(ok).To(BeTrue())
			Expect(rsaKey.N).NotTo(BeNil())

			_, _, _, _, err = ssh.ParseAuthorizedKey([]byte(publicKey))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns an error when key length is zero", func() {
			svc := &DefaultSSHService{RSAKeyLength: 0}
			_, _, err := svc.GenerateRSAKeyPair()

			Expect(err).To(HaveOccurred())
		})

		It("returns an error when key length is below the minimum", func() {
			svc := &DefaultSSHService{RSAKeyLength: 256}
			_, _, err := svc.GenerateRSAKeyPair()

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("GenerateED25519KeyPair", func() {
		It("generates non-empty private and public keys", func() {
			svc := &DefaultSSHService{}
			privateKey, publicKey, err := svc.GenerateED25519KeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(privateKey).NotTo(BeEmpty())
			Expect(publicKey).NotTo(BeEmpty())
		})

		It("returns a valid PEM-encoded OpenSSH private key", func() {
			svc := &DefaultSSHService{}
			privateKey, _, err := svc.GenerateED25519KeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(strings.Contains(privateKey, "-----BEGIN OPENSSH PRIVATE KEY-----")).To(BeTrue())
			Expect(strings.Contains(privateKey, "-----END OPENSSH PRIVATE KEY-----")).To(BeTrue())
		})

		It("returns a public key with ssh-ed25519 prefix", func() {
			svc := &DefaultSSHService{}
			_, publicKey, err := svc.GenerateED25519KeyPair()

			Expect(err).NotTo(HaveOccurred())
			Expect(publicKey).To(HavePrefix("ssh-ed25519 "))
		})

		It("returns keys that can be parsed and used as a valid pair", func() {
			svc := &DefaultSSHService{}
			privateKey, publicKey, err := svc.GenerateED25519KeyPair()

			Expect(err).NotTo(HaveOccurred())

			signer, err := ssh.ParsePrivateKey([]byte(privateKey))
			Expect(err).NotTo(HaveOccurred())

			pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
			Expect(err).NotTo(HaveOccurred())

			Expect(string(signer.PublicKey().Marshal())).To(Equal(string(pubKey.Marshal())))
		})

		It("produces signatures in ssh-ed25519 format", func() {
			svc := &DefaultSSHService{}
			privateKey, _, err := svc.GenerateED25519KeyPair()
			Expect(err).NotTo(HaveOccurred())

			signer, err := ssh.ParsePrivateKey([]byte(privateKey))
			Expect(err).NotTo(HaveOccurred())

			sig, err := signer.Sign(nil, []byte("test data"))
			Expect(err).NotTo(HaveOccurred())
			Expect(sig.Format).To(Equal("ssh-ed25519"))
		})
	})
})

func TestSSHService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SSH Service Suite")
}
