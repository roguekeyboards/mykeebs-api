package repository

import "context"

// OwnerImagePrefixes are the object key prefixes holding every image
// ownerID owns, one per New*ImageKey format. Each ends in "/", so one
// owner's prefix never matches another owner whose ID merely starts the
// same.
func OwnerImagePrefixes(ownerID string) []string {
	return []string{
		"builds/" + ownerID + "/",
		"ec-parts/" + ownerID + "/",
		"keyboards/" + ownerID + "/",
		"keycap-sets/" + ownerID + "/",
		"profiles/" + ownerID + "/",
		"switches/" + ownerID + "/",
	}
}

// OwnerImageStore deletes every image object the caller owns, whatever
// item it belongs to, including objects no item points to any more.
type OwnerImageStore interface {
	// DeleteAll removes every object under the caller's
	// OwnerImagePrefixes. Idempotent.
	DeleteAll(ctx context.Context) error
}
