package auth

import (
	"context"
	"fmt"
	"net"
	"time"
)

type rentalGameLinkConnection interface {
	SendGameStart(map[string]interface{}) error
	Conn() net.Conn
	Close() error
}

// sendRentalGameStart always releases the established Link session and sends
// exactly once. Cancellation closes the socket to interrupt a blocked write.
func sendRentalGameStart(ctx context.Context, conn rentalGameLinkConnection, payload map[string]interface{}) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	defer conn.Close()
	if err := ctx.Err(); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Conn().Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.Conn().SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("GameStart write deadline: %w", err)
	}
	if err := conn.SendGameStart(payload); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("SendGameStart: %w", err)
	}
	return ctx.Err()
}
