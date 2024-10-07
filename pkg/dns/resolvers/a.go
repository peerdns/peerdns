package resolvers

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

// AHandler handles DNS queries for 'A' records (IPv4 address resolution).
type AHandler struct{}

// HandleQuery processes a DNS query for 'A' records and generates a response.
func (h *AHandler) HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error) {
	ip := net.ParseIP("127.0.0.1") // Example IP; in production, query from a data source

	if ip == nil {
		return nil, fmt.Errorf("invalid IP address for %s", question.Name)
	}

	response := new(dns.Msg)
	response.SetReply(req)

	// Create A record response
	response.Answer = append(response.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   question.Name,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		A: ip,
	})

	return response, nil
}
