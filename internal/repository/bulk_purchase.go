package repository

import (
	"math"
)

// BulkPurchase is one order of a switch or EC part: where, how much and how
// many, plus its order lifecycle. Price is the total for Quantity units,
// not a per-unit price.
type BulkPurchase struct {
	Vendor                *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price                 *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate             *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate          *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	EstimatedDeliveryDate *string  `dynamodbav:"estimated_delivery_date,omitempty" json:"estimated_delivery_date,omitempty"`
	OrderStatus           *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
	Quantity              *int     `dynamodbav:"quantity,omitempty" json:"quantity,omitempty"`
}

// BulkPurchases is every order of one switch or EC part. The units are
// interchangeable, so a build draws on all of them at their average cost.
type BulkPurchases []BulkPurchase

// WithoutEmpty drops purchases with no field set, returning nil if none
// remain.
func (ps BulkPurchases) WithoutEmpty() BulkPurchases {
	var out BulkPurchases
	for _, p := range ps {
		if p != (BulkPurchase{}) {
			out = append(out, p)
		}
	}
	return out
}

func (p BulkPurchase) cancelled() bool {
	return p.OrderStatus != nil && *p.OrderStatus == "Cancelled"
}

// TotalQuantity sums the non-Cancelled purchases with a known quantity, or
// returns nil if none has one.
func (ps BulkPurchases) TotalQuantity() *int {
	var total int
	known := false
	for _, p := range ps {
		if p.Quantity == nil || p.cancelled() {
			continue
		}
		total += *p.Quantity
		known = true
	}
	if !known {
		return nil
	}
	return &total
}

// TotalPrice sums the non-Cancelled purchases with a known price, rounded
// to cents, or returns nil if none has one.
func (ps BulkPurchases) TotalPrice() *float64 {
	var total float64
	priced := false
	for _, p := range ps {
		if p.Price == nil || p.cancelled() {
			continue
		}
		total += *p.Price
		priced = true
	}
	if !priced {
		return nil
	}

	total = math.Round(total*100) / 100
	return &total
}

// UnitPrice is the average price per unit across the non-Cancelled
// purchases with both a price and a non-zero quantity, or nil if none has
// both.
func (ps BulkPurchases) UnitPrice() *float64 {
	var price float64
	var quantity int
	for _, p := range ps {
		if p.Price == nil || p.Quantity == nil || *p.Quantity == 0 || p.cancelled() {
			continue
		}
		price += *p.Price
		quantity += *p.Quantity
	}
	if quantity == 0 {
		return nil
	}

	unit := price / float64(quantity)
	return &unit
}

// OrderStatus is the least-progressed order_status across the purchases,
// by the same rules as [AggregateOrderStatus].
func (ps BulkPurchases) OrderStatus() *string {
	statuses := make([]*string, len(ps))
	for i, p := range ps {
		statuses[i] = p.OrderStatus
	}
	return leastProgressedOrderStatus(statuses)
}
