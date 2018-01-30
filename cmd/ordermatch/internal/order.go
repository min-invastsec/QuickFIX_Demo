package internal

import (
	"time"

	"github.com/quickfixgo/enum"
	"github.com/shopspring/decimal"
)

// Order order type
type Order struct {
	ClOrdID              string
	Symbol               string
	SenderCompID         string
	TargetCompID         string
	Side                 enum.Side
	OrdType              enum.OrdType
	Price                decimal.Decimal
	Quantity             decimal.Decimal
	ExecutedQuantity     decimal.Decimal
	openQuantity         *decimal.Decimal
	AvgPx                decimal.Decimal
	insertTime           time.Time
	LastExecutedQuantity decimal.Decimal
	LastExecutedPrice    decimal.Decimal
}

// IsClosed order open qty is 0 or not
func (o Order) IsClosed() bool {
	return o.OpenQuantity().Equals(decimal.Zero)
}

// OpenQuantity total qty - executed qty
func (o Order) OpenQuantity() decimal.Decimal {
	if o.openQuantity == nil {
		return o.Quantity.Sub(o.ExecutedQuantity)
	}

	return *o.openQuantity
}

// Execute order execute
func (o *Order) Execute(price, quantity decimal.Decimal) {
	o.ExecutedQuantity = o.ExecutedQuantity.Add(quantity)
	o.LastExecutedPrice = price
	o.LastExecutedQuantity = quantity
}

// Cancel order cancel
func (o *Order) Cancel() {
	openQuantity := decimal.Zero
	o.openQuantity = &openQuantity
}
