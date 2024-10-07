package resolvers

import (
	"github.com/miekg/dns"
)

// CNAMEHandler handles DNS queries for 'CNAME' records.
type CNAMEHandler struct{}

// HandleQuery processes a DNS query for 'CNAME' records and generates a response.
func (h *CNAMEHandler) HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error) {
	cname := "example.com." // Example CNAME; in production, query from a data source

	response := new(dns.Msg)
	response.SetReply(req)

	// Create CNAME record response
	response.Answer = append(response.Answer, &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   question.Name,
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Target: cname,
	})

	return response, nil
}
