package nats

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// NewConn creates a new NATS connection.
func NewConn(natsURL string) (*nats.Conn, error) {
	nc, err := nats.Connect(natsURL,
		nats.Name("go-saas-api"),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}
	return nc, nil
}
