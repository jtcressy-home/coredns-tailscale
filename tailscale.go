package tailscale

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/test"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn/ipnstate"
)

type Tailscale struct {
	Next            plugin.Handler
	tsClient        tailscale.LocalClient
	entries         map[string]map[string][]string
	zone            string
	pollingInterval time.Duration
}

func NewTailscale(next plugin.Handler) *Tailscale {
	ts := Tailscale{Next: test.ErrorHandler()}
	ts.pollPeers()
	return &ts
}

// Name implements the Handler interface.
func (t *Tailscale) Name() string { return "tailscale" }

func (t *Tailscale) pollPeers() {
	t.entries = map[string]map[string][]string{}

	res, err := t.tsClient.Status(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Add self to list of considered hosts
	hosts := []*ipnstate.PeerStatus{res.Self}

	// Add all peers to considered host list
	for _, status := range res.Peer {
		hosts = append(hosts, status)
	}

	for _, v := range hosts {

		// Process IPs for A and AAAA records
		for _, addr := range v.TailscaleIPs {

			_, ok := t.entries[strings.ToLower(v.HostName)]
			if !ok {
				t.entries[strings.ToLower(v.HostName)] = map[string][]string{}
			}

			if addr.Is4() {
				t.entries[strings.ToLower(v.HostName)]["A"] = []string{addr.String()}
			} else if addr.Is6() {
				t.entries[strings.ToLower(v.HostName)]["AAAA"] = []string{addr.String()}
			}

		}

		// Process Tags looking for cname- prefixed ones
		if v.Tags != nil {
			for i := 0; i < v.Tags.Len(); i++ {
				raw := v.Tags.At(i)
				if v.Online && strings.HasPrefix(raw, "tag:cname-") {
					tag := strings.TrimPrefix(raw, "tag:cname-")
					_, ok := t.entries[tag]
					if !ok {
						t.entries[tag] = map[string][]string{}
					}
					t.entries[tag]["CNAME"] = []string{fmt.Sprintf("%s.%s.", v.HostName, t.zone)}
				}
				// Additionally process tags looking for dnslb- prefix
				//  collects all IP addresses for every machine with matching tags
				//  and results in multiple A and AAAA records.
				// tip: use with 'loadbalance' in coredns for better dns-based loadbalancing
				if v.Online && strings.HasPrefix(raw, "tag:dnslb-") {
					tag := strings.TrimPrefix(raw, "tag:dnslb-")
					_, ok := t.entries[tag]
					if !ok {
						t.entries[tag] = map[string][]string{}
					}
					for _, addr := range v.TailscaleIPs {
						if addr.Is4() {
							t.entries[tag]["A"] = append(t.entries[tag]["A"], addr.String())
						} else if addr.Is6() {
							t.entries[tag]["AAAA"] = append(t.entries[tag]["AAAA"], addr.String())
						}
					}
				}
			}
		}
	}

	// Add self

}
