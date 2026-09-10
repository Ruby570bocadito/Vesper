package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"time"
)

type SPIFFEIdentity struct {
	TrustDomain string
	SVID        *x509.Certificate
	PrivateKey  *rsa.PrivateKey
	CertFile    string
	KeyFile     string
}

func NewSPIFFEIdentity(trustDomain string) (*SPIFFEIdentity, error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("spiffe key gen: %w", err)
	}

	spiffeURI, _ := url.Parse(fmt.Sprintf("spiffe://%s/agent/vesper", trustDomain))

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: fmt.Sprintf("spiffe://%s/agent/vesper", trustDomain)},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(1 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{spiffeURI},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("spiffe cert create: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("spiffe cert parse: %w", err)
	}

	return &SPIFFEIdentity{TrustDomain: trustDomain, SVID: cert, PrivateKey: key}, nil
}

func (si *SPIFFEIdentity) SaveToDisk(certPath, keyPath string) error {
	certFile, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certFile.Close()
	_ = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: si.SVID.Raw})

	keyFile, err := os.Create(keyPath)
	if err != nil {
		return err
	}
	defer keyFile.Close()
	_ = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(si.PrivateKey)})

	si.CertFile = certPath
	si.KeyFile = keyPath
	return nil
}

func (si *SPIFFEIdentity) TLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(si.CertFile, si.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("spiffe tls load: %w", err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}, nil
}
