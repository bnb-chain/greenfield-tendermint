package votepool

// EventType defines the types for voting.
type EventType uint8

// ToBscCrossChainEvent defines the type of cross chain events from the current chain to BSC.
const ToBscCrossChainEvent EventType = 1

// FromBscCrossChainEvent defines the type of cross chain events from BSC to the current chain.
const FromBscCrossChainEvent EventType = 2
