package db

import (
	"context"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
)

// SeedECPart PutItems a domes EC part directly into DynamoDB, bypassing
// the API, for specs that need fixture data in place first.
func SeedECPart(ctx context.Context, ownerID, id, visibility string) error {
	return SeedECPartOfType(ctx, ownerID, id, visibility, "domes")
}

// SeedECPartOfType is [SeedECPart] with the given part_type and no
// type-specific details.
func SeedECPartOfType(ctx context.Context, ownerID, id, visibility, partType string) error {
	table := NewDynamoTable(ctx, support.ECPartTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "DynaCap",
		"name":       "Heavy",
		"part_type":  partType,
		"visibility": visibility,
		"purchase": map[string]any{
			"vendor":       "Omnitype",
			"price":        36.0,
			"quantity":     72,
			"order_status": "Delivered",
		},
	})
}

// DeleteECPart removes an EC part directly from DynamoDB.
func DeleteECPart(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.ECPartTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
