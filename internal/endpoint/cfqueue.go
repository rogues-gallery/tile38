package endpoint

import (
	"context"
	"sync"
	"time"

	"github.com/cloudflare/cloudflare-go/v4"
	"github.com/cloudflare/cloudflare-go/v4/option"
	"github.com/cloudflare/cloudflare-go/v4/queues"
)

const cfqueueExpiresAfter = time.Second * 30

// CFQueueConn is an endpoint connection
type CFQueueConn struct {
	mu     sync.Mutex
	ep     Endpoint
	client *cloudflare.Client
	ex     bool
	t      time.Time
}

// Expired returns true if the connection has expired
func (conn *CFQueueConn) Expired() bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !conn.ex {
		if time.Since(conn.t) > cfqueueExpiresAfter {
			conn.close()
			conn.ex = true
		}
	}
	return conn.ex
}

// ExpireNow forces the connection to expire
func (conn *CFQueueConn) ExpireNow() {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.close()
	conn.ex = true
}

func (conn *CFQueueConn) close() {
	if conn.client != nil {
		conn.client = nil
	}
}

// Send sends a message
func (conn *CFQueueConn) Send(msg string) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.ex {
		return errExpired
	}
	conn.t = time.Now()

	// Initialize client if not already done
	if conn.client == nil {
		conn.client = cloudflare.NewClient(
			option.WithAPIToken(conn.ep.CFQueue.APIToken),
		)
	}

	// Push message to CF Queue
	_, err := conn.client.Queues.Messages.Push(
		context.TODO(),
		conn.ep.CFQueue.QueueID,
		queues.MessagePushParams{
			AccountID: cloudflare.String(conn.ep.CFQueue.AccountID),
			Body: queues.MessagePushParamsBodyMqQueueMessageText{
				Body:        cloudflare.String(msg),
				ContentType: cloudflare.F(queues.MessagePushParamsBodyMqQueueMessageTextContentTypeText),
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func newCFQueueConn(ep Endpoint) *CFQueueConn {
	return &CFQueueConn{
		ep: ep,
		t:  time.Now(),
	}
}
