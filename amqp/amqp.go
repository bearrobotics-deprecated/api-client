// Package amqp provides abstraction for commaon AMQP091 functions
// including service call (RPC) and Pub/Sub.
package amqp

import (
	"fmt"
	"math/rand"
	"net/url"

	amqpgo "github.com/rabbitmq/amqp091-go"
)

// randInt returns a random integer with the give reange.
func randInt(min int, max int) int {
	return min + rand.Int()%(max-min)
}

// randomString returns a random string of lowercase ASCII characters with the given length.
func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(97, 122))
	}
	return string(bytes)
}

// Delivery represents basic data received by subscribers. In addition, it
// annotates messages with their origin ID.
type Delivery struct {
	Origin string
	Body   []byte
}

// Dial returns a new Connection to rabbitMQ server given user credential and address.
//
// The connection will be over TCP and authentication will be plain using
// provided user/pass.
func Dial(user, pass, serverAddr, vHost string) (*amqpgo.Connection, error) {
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(user, pass),
		Host:   serverAddr,
		Path:   vHost,
	}
	return amqpgo.Dial(u.String())
}

// DialTLS returns a new Connection to rabbitMQ server given user credential and address.
//
// The connection will be secured with TLS and authenticated with  certificates.
// In TLS mode we expect ca_certificate.pem, certificate.pem, and key.pem
// files to be accessible from certPath. Since RabbitMQ server is configured to authenticate
// users with their certificate (see rabbitmq_auth_mechanism_ssl), username will be equal
// to certificate common name (CN).
func DialTLS(serverAddr, vHost, certPath string) (*amqpgo.Connection, error) {
	u := &url.URL{
		Scheme: "amqps",
		// User:  No user password allowed
		Host: serverAddr,
		Path: vHost,
	}
	cfg, err := getTLSConfig(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get dial config: %w", err)
	}

	return amqpgo.DialTLS_ExternalAuth(u.String(), cfg)
}
