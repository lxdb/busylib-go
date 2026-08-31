package pahotransport

import (
	"fmt"
	"testing"

	"github.com/eclipse/paho.golang/paho"
	"github.com/lxdb/busylib-go/remote"
)

func BenchmarkRoutePublish(b *testing.B) {
	for _, subscriberCount := range []int{1, 16} {
		b.Run(fmt.Sprintf("subscribers-%d", subscriberCount), func(b *testing.B) {
			transport := &Transport{subscriptions: make(map[string]map[*subscription]struct{})}
			registered := make(map[*subscription]struct{}, subscriberCount)
			for range subscriberCount {
				item := &subscription{
					maximum:  1024,
					messages: make(chan remote.Message, 1),
					done:     make(chan struct{}),
				}
				registered[item] = struct{}{}
			}
			transport.subscriptions["benchmark"] = registered
			packet := paho.PublishReceived{Packet: &paho.Publish{Topic: "benchmark", Payload: make([]byte, 256)}}
			b.ReportAllocs()
			for b.Loop() {
				_, _ = transport.route(packet)
				for item := range registered {
					<-item.messages
				}
			}
		})
	}
}
