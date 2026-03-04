package pubsub

import (
	"fmt"
	"log"

	ipfsnode "github.com/ipfs/go-ipfs-api"
)

type PubSubCallback func(peerID string, topic string, data []byte)

type PubSub struct {
	ipfs *ipfsnode.Shell
	sub  map[string]PubSubCallback
}

func NewPubSub(ipfsHost string) (*PubSub, error) {
	return &PubSub{
		ipfs: ipfsnode.NewShell(ipfsHost),
		sub:  make(map[string]PubSubCallback),
	}, nil
}

func (ps *PubSub) SubscribeTopic(topic string, cb PubSubCallback) error {
	if _, exists := ps.sub[topic]; exists {
		log.Printf("⚠️ topic already subscribed: %s", topic)
		return fmt.Errorf("topic already subscribed")
	}
	ps.sub[topic] = cb

	p, err := ps.ipfs.PubSubSubscribe(topic)
	if err != nil {
		log.Printf("⚠️ topic failed to subscribe: %v", err)
		return err
	}

	log.Printf("📡 Successfully subscribed to IPFS PubSub topic: %s", topic)
	go ps.receivePub(topic, p)
	return nil
}

func (ps *PubSub) receivePub(topic string, p *ipfsnode.PubSubSubscription) {
	for {
		m, err := p.Next()
		if err != nil {
			// Similar to fullnode, we can ignore or break on specific errors, but for now log and continue
			// log.Printf("failed to read message: %v", err)
			continue
		}
		if cb, exists := ps.sub[topic]; exists {
			go cb(m.From.String(), topic, m.Data)
		}
	}
}
