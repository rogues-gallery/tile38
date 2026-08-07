package endpoint

import (
	"context"
	"fmt"
	"time"

	"github.com/tidwall/gjson"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"
)

const ()

// HTTPConn is an endpoint connection
type EvenHubConn struct {
	ep Endpoint
}

func newEventHubConn(ep Endpoint) *EvenHubConn {
	return &EvenHubConn{
		ep: ep,
	}
}

// Expired returns true if the connection has expired
func (conn *EvenHubConn) Expired() bool {
	return false
}

// ExpireNow forces the connection to expire
func (conn *EvenHubConn) ExpireNow() {
}

// Send sends a message
func (conn *EvenHubConn) Send(msg string) error {
	hub, err := eventhub.NewHubFromConnectionString(conn.ep.EventHub.ConnectionString)

	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// parse json again to get out info for our kafka key
	key := gjson.Get(msg, "key")
	id := gjson.Get(msg, "id")
	keyValue := fmt.Sprintf("%s-%s", key.String(), id.String())

	evtHubMsg := eventhub.NewEventFromString(msg)
	evtHubMsg.PartitionKey = &keyValue
	err = hub.Send(ctx, evtHubMsg)
	if err != nil {
		return err
	}

	return nil
}
