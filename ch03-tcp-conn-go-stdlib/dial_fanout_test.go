package ch03

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// --- STEP 7 ---
// ## Canceling Multiple Dialers
// You can pass the same context to multiple `DialContext` calls and cancel all the calls at the same time by executing the context’s cancel function.
//	- For example, let’s assume you need to retrieve a resource via TCP that is on several servers.
//	- You can asynchronously dial each server, passing each dialer the same context.
//	- You can then abort the remaining dialers after you receive a response from one of the servers.
//		- In Listing 3-8, you pass the same context to multiple dialers.
//		- When you receive the first response, you cancel the context and abort the remaining dialers.

// Listing 3-8: Canceling all outstanding dialers after receiving the first response
func TestDialContextCancelFanOut(t *testing.T) {
	t.Run("with at least one answer", func(t *testing.T) {
		ctx, cancel := context.WithDeadline( // (1)
			context.Background(),
			time.Now().Add(10*time.Second),
		)
		defer cancel()

		listener, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()

		go func() { //(2)
			// Only accepting a single connection.
			conn, err := listener.Accept()
			if err == nil {
				conn.Close()
			}
		}()

		dial := func(ctx context.Context, address string, response chan int, id int) { // (3)
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", address)
			if err != nil {
				return
			}
			c.Close()

			select {
			case <-ctx.Done():
			case response <- id:
			}
		}

		res := make(chan int)
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ { // (4)
			wg.Go(func() { dial(ctx, listener.Addr().String(), res, i+1) })
		}

		var response int
		select {
		case <-ctx.Done():
		case response = <-res:
			cancel()
		}

		wg.Wait()

		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Errorf("expected canceled context; actual: %s",
				ctx.Err(),
			)
		}

		t.Logf("dialer %d retrieved the resource", response)
	})

	t.Run("without an answer", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(
			context.Background(),
			time.Now().Add(10*time.Second),
		)
		defer cancel()

		listener, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}
		// close the listener immediately to prevent a connection
		listener.Close()

		dial := func(ctx context.Context, address string, response chan int, id int) {
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", address)
			if err != nil {
				return
			}
			c.Close()

			select {
			case <-ctx.Done():
			case response <- id:
			}
		}

		res := make(chan int)
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ { // (4)
			wg.Go(func() { dial(ctx, listener.Addr().String(), res, i+1) })
		}

		var response int
		select {
		case <-ctx.Done():
		case response = <-res: // (5)
			cancel()
		}

		wg.Wait()

		if !errors.Is(ctx.Err(), context.DeadlineExceeded) { // (6)
			t.Errorf("expected deadline exceeded; actual: %s",
				ctx.Err(),
			)
		}

		if response != 0 {
			t.Fatalf("expected a response of 0; actual: %d", response)
		}

		t.Log("no dialer retrieved the resource")
	})
}
