package resolvers

import (
	"github.com/miekg/dns"
)

// MXHandler handles DNS queries for 'MX' records.
type MXHandler struct{}

// HandleQuery processes a DNS query for 'MX' records and generates a response.
func (h *MXHandler) HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error) {
	mx := "mail.example.com." // Example MX record; in production, query from a data source

	response := new(dns.Msg)
	response.SetReply(req)

	// Create MX record response
	response.Answer = append(response.Answer, &dns.MX{
		Hdr: dns.RR_Header{
			Name:   question.Name,
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Mx:         mx,
		Preference: 10,
	})

	return response, nil
}
