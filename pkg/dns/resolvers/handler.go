package resolvers

import "github.com/miekg/dns"

// Handler defines the interface for handling specific DNS query types.
type Handler interface {
	HandleQuery(w dns.ResponseWriter, req *dns.Msg, question dns.Question) (*dns.Msg, error)
}
