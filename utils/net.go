package utils

import (
	"net"
	"time"

	"github.com/miekg/dns"
)

const (
	MAX_TIMEOUT = 1000
)

var dnsClient *dns.Client

func init() {
	dnsClient = new(dns.Client)
	dnsClient.Net = "udp"
	dnsClient.Timeout = time.Second * 1
}

func Hc(ip string) (int, error) {
	_, rtt, err := dnsClient.Exchange(&dns.Msg{}, net.JoinHostPort(ip, "53"))
	if err != nil {
		return MAX_TIMEOUT, err
	}

	return int(rtt.Milliseconds()), nil
}
