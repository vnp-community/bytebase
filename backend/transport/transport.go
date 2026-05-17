// Package transport provides abstractions for service-to-service communication.
// It enables switching between in-process (bufconn) and network (TCP) transport
// without changing service logic.
package transport

import (
	"context"
	"net"
	"net/http"
	"sync"

	"google.golang.org/grpc/test/bufconn"
)

// BufconnListener creates an in-memory listener for internal HTTP servers.
// Used for single-binary mode where gateway and services share one process.
func BufconnListener(bufSize int) *bufconn.Listener {
	return bufconn.Listen(bufSize)
}

// BufconnHTTPClient creates an http.Client that dials through a bufconn listener.
// Requests go through the in-memory buffer — zero network overhead.
func BufconnHTTPClient(lis *bufconn.Listener) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return lis.Dial()
			},
		},
	}
}

// ServiceTransport abstracts the communication transport for service-to-service calls.
// Implementations: BufconnTransport (single binary) and TCPTransport (future multi-binary).
type ServiceTransport interface {
	// HTTPClient returns an HTTP client configured for this transport.
	HTTPClient() *http.Client

	// Listener returns the net.Listener for the internal HTTP server.
	Listener() net.Listener

	// BaseURL returns the base URL for REST calls.
	BaseURL() string

	// Close releases transport resources.
	Close() error
}

const defaultBufSize = 1024 * 1024 // 1MB buffer

// BufconnTransport implements ServiceTransport using in-memory bufconn.
// All communication stays within the same process.
type BufconnTransport struct {
	listener   *bufconn.Listener
	httpClient *http.Client
	once       sync.Once
}

// NewBufconnTransport creates a new in-process transport.
func NewBufconnTransport() *BufconnTransport {
	lis := BufconnListener(defaultBufSize)
	return &BufconnTransport{
		listener:   lis,
		httpClient: BufconnHTTPClient(lis),
	}
}

// NewBufconnTransportWithSize creates a transport with a custom buffer size.
func NewBufconnTransportWithSize(bufSize int) *BufconnTransport {
	lis := BufconnListener(bufSize)
	return &BufconnTransport{
		listener:   lis,
		httpClient: BufconnHTTPClient(lis),
	}
}

// HTTPClient returns an http.Client that routes through the bufconn listener.
func (t *BufconnTransport) HTTPClient() *http.Client {
	return t.httpClient
}

// Listener returns the underlying bufconn listener for use by internal HTTP servers.
func (t *BufconnTransport) Listener() net.Listener {
	return t.listener
}

// BaseURL returns the base URL. For bufconn, we use a synthetic address.
func (t *BufconnTransport) BaseURL() string {
	return "http://bufconn"
}

// Close releases transport resources.
func (t *BufconnTransport) Close() error {
	var err error
	t.once.Do(func() {
		err = t.listener.Close()
	})
	return err
}
