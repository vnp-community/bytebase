package server

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufConnSize = 16 * 1024 * 1024 // 16MB buffer

// BufConnDialer creates an in-process gRPC connection using bufconn.
// This eliminates TCP loopback overhead for the REST gateway proxy.
//
// Usage:
//
//	lis, conn, err := NewBufConnGateway(ctx, opts...)
//	// Register gRPC services on lis
//	// Use conn for gateway handler registration
type BufConnGateway struct {
	listener *bufconn.Listener
}

// NewBufConnGateway creates a bufconn listener and a gRPC client connection.
func NewBufConnGateway(ctx context.Context, opts ...grpc.DialOption) (*BufConnGateway, *grpc.ClientConn, error) {
	lis := bufconn.Listen(bufConnSize)
	gw := &BufConnGateway{listener: lis}

	// Build dial options: insecure + bufconn dialer + any extra opts
	baseOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	}
	allOpts := append(baseOpts, opts...)

	conn, err := grpc.NewClient("passthrough:///bufconn", allOpts...)
	if err != nil {
		lis.Close()
		return nil, nil, err
	}

	return gw, conn, nil
}

// Listener returns the bufconn listener for registering gRPC servers.
func (gw *BufConnGateway) Listener() net.Listener {
	return gw.listener
}

// Close shuts down the bufconn listener.
func (gw *BufConnGateway) Close() error {
	return gw.listener.Close()
}
