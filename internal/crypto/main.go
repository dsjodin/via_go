package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// writePEM writes a single PEM block to path with the given permissions.
//
// Errors from pem.Encode and Close are reported rather than discarded: a
// partial write leaves a truncated certificate or key on disk, and TLS then
// fails later with an error that points nowhere near the real cause.
func writePEM(path string, perm os.FileMode, block *pem.Block) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if err := pem.Encode(f, block); err != nil {
		_ = f.Close()
		_ = os.Remove(path) // best effort: do not leave a truncated file behind
		return fmt.Errorf("encode %s: %w", path, err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}

func CreateCA() {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{"VMware ESX Server Default Certificate"},
			Country:      []string{"US"},
			Province:     []string{"California"},
			Locality:     []string{"Palo Alto"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logrus.Fatalf("cert: generate CA key: %v", err)
	}
	pub := &priv.PublicKey
	ca_b, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, priv)
	if err != nil {
		log.Println("create ca failed", err)
		return
	}

	// Public key
	if err := writePEM("cert/ca.crt", 0644, &pem.Block{Type: "CERTIFICATE", Bytes: ca_b}); err != nil {
		logrus.Fatalf("cert: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"cert": "ca.pem created",
	}).Info("cert")

	// Private key
	if err := writePEM("cert/ca.key", 0600, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		logrus.Fatalf("cert: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"cert": "ca.key created",
	}).Info("cert")
}

func CreateCert(path string, name string, cn string) {

	// Load CA
	catls, err := tls.LoadX509KeyPair("cert/ca.crt", "cert/ca.key")
	if err != nil {
		logrus.Fatal(err)
	}
	ca, err := x509.ParseCertificate(catls.Certificate[0])
	if err != nil {
		logrus.Fatal(err)
	}

	// Prepare certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			Organization: []string{"VMware ESX Server Default Certificate"},
			Country:      []string{"US"},
			Province:     []string{"California"},
			Locality:     []string{"Palo Alto"},
			CommonName:   cn,
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	// add SAN
	cert.DNSNames = []string{cn}
	cert.EmailAddresses = []string{"ssl-certificates@vmware.com"}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		logrus.Fatalf("cert: generate key for %s: %v", name, err)
	}
	pub := &priv.PublicKey

	// Sign the certificate
	cert_b, err := x509.CreateCertificate(rand.Reader, cert, ca, pub, catls.PrivateKey)
	if err != nil {
		logrus.Fatal(err)
	}

	// Public key
	crtPath := path + "/" + name + ".crt"
	if err := writePEM(crtPath, 0644, &pem.Block{Type: "CERTIFICATE", Bytes: cert_b}); err != nil {
		logrus.Fatalf("cert: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"cert": crtPath + " created",
	}).Info("cert")

	// Private key
	keyPath := path + "/" + name + ".key"
	if err := writePEM(keyPath, 0600, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		logrus.Fatalf("cert: %v", err)
	}
	logrus.WithFields(logrus.Fields{
		"cert": keyPath + " created",
	}).Info("cert")
}
