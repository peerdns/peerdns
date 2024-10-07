package resolvers

import (
	"github.com/miekg/dns"
)

// TXTHandler handles DNS queries for 'TXT' records.
type TXTHandler struct{}

// HandleQuery processes a DNS query for 'TXT' records and generates a response.
func (h *TXTHandler) HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error) {
	txt := "v=spf1 include:example.com ~all" // Example SPF record; in production, query from a data source

	response := new(dns.Msg)
	response.SetReply(req)

	// Create TXT record response
	response.Answer = append(response.Answer, &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   question.Name,
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Txt: []string{txt},
	})

	return response, nil
}
