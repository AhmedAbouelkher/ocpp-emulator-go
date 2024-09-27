package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

const (
	year = 365 * 24 * time.Hour
)

func main() {
	// Generate a private key for the CA
	caPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate CA private key: %v", err)
	}

	// Create a template for the CA certificate
	caTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{"Ahmed, INC."},
			Country:      []string{"EG"},
			Province:     []string{"Alexandria"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * year), // Valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	// Self-sign the CA certificate
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		log.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Save the CA private key and certificate to files
	saveToPEM("ca_cert.pem", "CERTIFICATE", caCertDER)
	savePrivateKey("ca_key.pem", caPriv)
	println("DO NOT SHARE THE CA PRIVATE KEY WITH ANYONE!")

	// Generate a private key for the server certificate
	serverPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate server private key: %v", err)
	}

	// Create a template for the server certificate
	serverTemplate := &x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			Organization: []string{"My OCPP Server"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(2 * year), // Valid for 2 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"ocpp-server.com"}, // Important: SAN contains 'ocpp-server.com'
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Sign the server certificate with the CA
	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverPriv.PublicKey, caPriv)
	if err != nil {
		log.Fatalf("Failed to create server certificate: %v", err)
	}

	// Save the server private key and certificate to files
	saveToPEM("server_cert.pem", "CERTIFICATE", serverCertDER)
	savePrivateKey("server_key.pem", serverPriv)
}

// saveToPEM saves the certificate to a PEM file
func saveToPEM(filename, fileType string, derBytes []byte) {
	certOut, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to open %s for writing: %v", filename, err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: fileType, Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to %s: %v", filename, err)
	}
	certOut.Close()
	log.Printf("Wrote %s\n", filename)
}

// savePrivateKey saves the private key to a PEM file
func savePrivateKey(filename string, key *ecdsa.PrivateKey) {
	keyOut, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to open %s for writing: %v", filename, err)
	}
	privBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Fatalf("Failed to marshal private key: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()
	log.Printf("Wrote %s\n", filename)
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}
	return serialNumber
}
