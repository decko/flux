package model

import "github.com/google/uuid"

// TicketID generates a deterministic UUID v5 (SHA-1) for a ticket from its
// source system and external identifier. The same source+externalID pair
// always produces the same UUID, preventing duplicate tickets across
// webhook and sync ingestion paths.
//
// The key format is "<source>-<externalID>", matching the legacy webhook ID
// scheme so that existing records produced by webhook events retain their IDs.
func TicketID(source TicketSource, externalID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(string(source)+"-"+externalID)).String()
}

// PRID generates a deterministic UUID v5 (SHA-1) for a pull request from
// its source system and external identifier. Uses a different key format
// than TicketID ("<source>-pr-<externalID>") to prevent collisions between
// tickets and PRs that share the same source and external ID.
func PRID(source PRSource, externalID string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(string(source)+"-pr-"+externalID)).String()
}
