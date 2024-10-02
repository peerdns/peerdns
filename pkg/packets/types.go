package packets

// PacketType represents the type of a packet, encompassing both consensus and network messages.
type PacketType uint8

const (
	PacketTypeUnknown PacketType = 0 // Represents an unknown message type

	// Consensus Packet Types
	PacketTypeProposal     PacketType = 1 // Indicates a new block proposal
	PacketTypeApproval     PacketType = 2 // Indicates approval of a proposal
	PacketTypeFinalization PacketType = 3 // Indicates block finalization

	// Network Packet Types
	PacketTypePing     PacketType = 5 // Represents a simple ping message
	PacketTypeRequest  PacketType = 6 // Represents a request message
	PacketTypeResponse PacketType = 7 // Represents a response message
)
