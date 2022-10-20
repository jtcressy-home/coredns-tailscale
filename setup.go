package tailscale

import (
	"fmt"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

// init registers this plugin.
func init() { plugin.Register("tailscale", setup) }

// setup is the function that gets called when the config parser see the token "example". Setup is responsible
// for parsing any extra options the example plugin may have. The first token this function sees is "example".
func setup(c *caddy.Controller) error {
	ts, err := parse(c)
	if err != nil {
		return plugin.Error("tailscale", err)
	}

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		ts.Next = next
		ts.pollPeers()
		go func() {
			for range time.Tick(ts.pollingInterval) {
				ts.pollPeers()
			}
		}()
		return ts
	})

	// All OK, return a nil error.
	return nil
}

func parse(c *caddy.Controller) (*Tailscale, error) {
	ts := &Tailscale{
		pollingInterval: 60 * time.Second,
	}

	for c.Next() {
		zones := c.RemainingArgs()

		if len(zones) == 0 {
			zones = make([]string, len(c.ServerBlockKeys))
		}
		if len(zones) != 1 {
			return nil, c.ArgErr()
		}

		ts.zone = zones[0]

		for c.NextBlock() {
			switch c.Val() {
			case "poll_interval":
				if !c.NextArg() {
					return nil, c.ArgErr()
				}
				rawdur := c.Val()
				dur, err := time.ParseDuration(rawdur)
				if err != nil {
					return nil, fmt.Errorf("poll_interval has invalid duration: %v", rawdur)
				}
				if dur < 0 {
					return nil, fmt.Errorf("poll_interval can't be negative: %d", dur)
				}
				ts.pollingInterval = dur
			default:
				return nil, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	return ts, nil
}
