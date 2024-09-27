package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"math/big"
	"os"
	"time"
)

var (
	isCA, isCRT, isSignCSR bool
	csrFile                string

	certSubject = pkix.Name{
		SerialNumber: "04a970ec72639e056482",
		// CommonName:    "localhost",
		Organization:  []string{"Company, INC."},
		Country:       []string{"EG"},
		Province:      []string{"Cairo"},
		Locality:      []string{"Cairo"},
		StreetAddress: []string{"Nasr City"},
		PostalCode:    []string{"11765"},
	}
)

func main() {

	flag.BoolVar(&isCA, "ca", false, "generate a CA")
	flag.BoolVar(&isCRT, "crt", false, "generate a certificate")
	flag.BoolVar(&isSignCSR, "csr", false, "sign a CSR")
	flag.StringVar(&csrFile, "csrf", "", "CSR file to sign")
	flag.Parse()

	t := time.Now()

	if isCA {
		ca, key, err := genCA()
		if err != nil {
			panic(err)
		}
		// write the CA to disk
		writeToFile("ca.pem", ca)
		// write the key to disk
		writeToFile("ca.key", key)
	} else if isCRT {
		// load the CA
		caCert, err := os.ReadFile("ca.pem")
		if err != nil {
			panic(err)
		}
		caPrivKey, err := os.ReadFile("ca.key")
		if err != nil {
			panic(err)
		}
		// generate the client certificate
		cert, key, err := genClientCrt(bytes.NewBuffer(caCert), bytes.NewBuffer(caPrivKey))
		if err != nil {
			panic(err)
		}
		writeToFile("client_cert.pem", cert)
		writeToFile("client_cert.key", key)
	} else if isSignCSR {
		if csrFile == "" {
			flag.Usage()
			println("CSR file not specified")
			os.Exit(2)
		}
		caCert, err := os.ReadFile("ca.pem")
		if err != nil {
			panic(err)
		}
		caPrivKey, err := os.ReadFile("ca.key")
		if err != nil {
			panic(err)
		}
		clientCSR, err := os.ReadFile(csrFile)
		if err != nil {
			panic(err)
		}
		clientCRT, err := signCSR(caCert, caPrivKey, clientCSR)
		if err != nil {
			panic(err)
		}
		writeToFile("signed_client_cert.pem", clientCRT)
	} else {
		flag.Usage()
		os.Exit(1)
	}

	elapsed := time.Since(t)
	println("elapsed time:", elapsed.String())
}

func genCA() (*bytes.Buffer, *bytes.Buffer, error) {
	// set up our CA certificate
	sn, err := rand.Int(rand.Reader, big.NewInt(1000000000000000000))
	if err != nil {
		return nil, nil, err
	}
	ca := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               certSubject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// create our private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	// pem encode
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(caPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(caPrivKey),
	})

	return caPEM, caPrivKeyPEM, nil
}

func genClientCrt(rawCA *bytes.Buffer, caKey *bytes.Buffer) (*bytes.Buffer, *bytes.Buffer, error) {
	// set up our server certificate
	sn, err := rand.Int(rand.Reader, big.NewInt(1000000000000000000))
	if err != nil {
		return nil, nil, err
	}
	cert := &x509.Certificate{
		SerialNumber: sn,
		Subject:      certSubject,
		DNSNames:     []string{certSubject.CommonName},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	// parse PEM encoded CA
	pemBlock, _ := pem.Decode(rawCA.Bytes())
	if pemBlock == nil {
		return nil, nil, err
	}
	caCert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}

	// parse PEM encoded CA private key
	pemBlock, _ = pem.Decode(caKey.Bytes())
	if pemBlock == nil {
		return nil, nil, err
	}
	caPrivKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	return certPEM, certPrivKeyPEM, nil
}

func signCSR(rawCA []byte, caKey []byte, rawCSR []byte) (*bytes.Buffer, error) {
	// parse PEM encoded CA
	pemBlock, _ := pem.Decode(rawCA)
	if pemBlock == nil {
		return nil, errors.New("failed to parse CA")
	}
	caCert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}
	// parse PEM encoded CA private key
	pemBlock, _ = pem.Decode(caKey)
	if pemBlock == nil {
		return nil, errors.New("failed to parse CA private key")
	}
	caPrivKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}
	// parse PEM encoded CSR
	pemBlock, _ = pem.Decode(rawCSR)
	if pemBlock == nil {
		return nil, errors.New("failed to parse CSR")
	}
	clientCSR, err := x509.ParseCertificateRequest(pemBlock.Bytes)
	if err != nil {
		panic(err)
	}
	clientCRTTemplate := x509.Certificate{
		Signature:          clientCSR.Signature,
		SignatureAlgorithm: clientCSR.SignatureAlgorithm,

		PublicKeyAlgorithm: clientCSR.PublicKeyAlgorithm,
		PublicKey:          clientCSR.PublicKey,
		Subject:            clientCSR.Subject,

		SerialNumber: big.NewInt(2),
		Issuer:       caCert.Subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientCRTBytes, err := x509.CreateCertificate(rand.Reader, &clientCRTTemplate, caCert, clientCSR.PublicKey, caPrivKey)
	if err != nil {
		panic(err)
	}
	clientPEM := new(bytes.Buffer)
	pem.Encode(clientPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCRTBytes,
	})
	return clientPEM, nil
}

func writeToFile(filename string, data *bytes.Buffer) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data.Bytes())
	if err != nil {
		return err
	}
	return nil
}
