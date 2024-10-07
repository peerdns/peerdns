package resolvers

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
)

// AAAAHandler handles DNS queries for 'AAAA' records (IPv6 address resolution).
type AAAAHandler struct{}

// HandleQuery processes a DNS query for 'AAAA' records and generates a response.
func (h *AAAAHandler) HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error) {
	ip := net.ParseIP("::1") // Example IPv6 address; in production, query from a data source

	if ip == nil {
		return nil, fmt.Errorf("invalid IPv6 address for %s", question.Name)
	}

	response := new(dns.Msg)
	response.SetReply(req)

	// Create AAAA record response
	response.Answer = append(response.Answer, &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   question.Name,
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		AAAA: ip,
	})

	return response, nil
}
