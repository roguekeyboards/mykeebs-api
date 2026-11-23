package schema

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/cascadedelete"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

var visibilities = []repository.Visibility{
	repository.VisibilityPublic,
	repository.VisibilityAuthenticated,
	repository.VisibilityPrivate,
}

const maxListLimit = 100

func checkListLimit(v *validate.V, limit *int) {
	if limit != nil && (*limit < 1 || *limit > maxListLimit) {
		v.Fail("limit", fmt.Sprintf("must be between 1 and %d", maxListLimit))
	}
}

func checkVisibility(v *validate.V, s string) {
	validate.OneOf(v, "visibility", repository.Visibility(s), visibilities)
}

func checkOnDelete(v *validate.V, s string) {
	if _, ok := cascadedelete.ParseOnDelete(s); !ok {
		v.Fail("on_delete", "must be block or cascade")
	}
}

func checkImageSize(v *validate.V, size int64) {
	if size < 1 || size > repository.MaxImageSizeBytes {
		v.Fail("size_bytes", fmt.Sprintf("must be between 1 and %d (1.5 MB); resize or recompress the image first", repository.MaxImageSizeBytes))
	}
}

func checkPurchase(v *validate.V, price *float64, orderDate, deliveryDate *string) {
	v.Money("price", price)
	v.Date("order_date", orderDate)
	v.Date("delivery_date", deliveryDate)
}
