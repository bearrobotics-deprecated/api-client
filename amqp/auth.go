package amqp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"path"
)

// newTLSConfig creates a new TLS configuration given certificates for CA, client
// as well as private key for client (all int .PEM format).
func newTLSConfig(caPem, certPem, keyPem []byte) (*tls.Config, error) {
	certPool := x509.NewCertPool()
	if certPool.AppendCertsFromPEM(caPem) != true {
		return nil, fmt.Errorf("failed to append cert from pem")
	}

	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate from key pair: %w", err)
	}

	return &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
	}, nil
}

func getTLSConfig(certPath string) (*tls.Config, error) {
	caPem, err := ioutil.ReadFile(path.Join(certPath, "ca_certificate.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}

	certPem, err := ioutil.ReadFile(path.Join(certPath, "certificate.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read cert file: %w", err)
	}

	keyPem, err := ioutil.ReadFile(path.Join(certPath, "key.pem"))
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	return newTLSConfig(caPem, certPem, keyPem)
}
