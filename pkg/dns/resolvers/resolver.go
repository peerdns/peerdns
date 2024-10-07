package resolvers

import (
	"github.com/miekg/dns"
	"log"
)

// Resolver represents the core DNS resolver with handlers for different query types.
type Resolver struct {
	handlers map[uint16]Handler
}

// NewResolver initializes a new DNS resolver with handlers for each supported query type.
func NewResolver() *Resolver {
	handlers := make(map[uint16]Handler)

	// Register all supported handlers
	handlers[dns.TypeA] = &AHandler{}
	handlers[dns.TypeAAAA] = &AAAAHandler{}
	handlers[dns.TypeCNAME] = &CNAMEHandler{}
	handlers[dns.TypeTXT] = &TXTHandler{}
	handlers[dns.TypeMX] = &MXHandler{}

	return &Resolver{handlers: handlers}
}

// Resolve handles an incoming DNS query, using the appropriate handler based on query type.
func (r *Resolver) Resolve(w dns.ResponseWriter, req *dns.Msg) {
	response := new(dns.Msg)
	response.SetReply(req)
	response.Compress = true

	for _, question := range req.Question {
		handler, ok := r.handlers[question.Qtype]
		if !ok {
			log.Printf("Unsupported query type: %d for domain %s", question.Qtype, question.Name)
			dns.HandleFailed(w, req)
			return
		}

		// Handle query using the specific handler
		msg, err := handler.HandleQuery(w, req, question)
		if err != nil {
			log.Printf("Error handling query for %s: %v", question.Name, err)
			dns.HandleFailed(w, req)
			return
		}

		// Merge the handler's response into the main response
		response.Answer = append(response.Answer, msg.Answer...)
	}

	// Write the complete response to the client
	if err := w.WriteMsg(response); err != nil {
		log.Printf("Failed to write response for %s: %v", req.Question[0].Name, err)
	}
}
