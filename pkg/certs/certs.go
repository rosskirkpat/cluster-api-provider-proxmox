package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

const (
	rsaBits  = 2048
	validFor = time.Hour * 24 * 90
)

// NewSelfSignedCert takes inspiration from https://github.com/golang/go/blob/ca8b31920a23541dda56bc76d3ddcaef3c3c0866/src/crypto/tls/generate_cert.go
// it only creates rsa keys for now
func NewSelfSignedCert() (cert []byte, key []byte, e error) {
	const (
		localhost = "localhost"
		loopback  = "127.0.0.1"
	)

	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)

	if err != nil {
		return nil, nil, err
	}

	keyUsage := x509.KeyUsageDigitalSignature
	keyUsage |= x509.KeyUsageKeyEncipherment

	notBefore := time.Now()

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Spectro Cloud"},
		},
		DNSNames: []string{
			localhost,
			"stylus-registry",
			"stylus-webhook.spectro-system.svc",
		},
		IPAddresses: []net.IP{
			net.ParseIP(loopback).To4(),
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(validFor),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IsCA = true
	template.KeyUsage |= x509.KeyUsageCertSign

	cert, err = x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to marshal private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}),
		nil

}
