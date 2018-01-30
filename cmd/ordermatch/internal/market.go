package internal

import (
	"fmt"
	"sort"
	"time"

	"github.com/quickfixgo/enum"
)

// orderList sorted order list
type orderList struct {
	orders []*Order
	sortBy func(o1, o2 *Order) bool
}

func (l orderList) Len() int { return len(l.orders) }

func (l orderList) Swap(i, j int) {
	l.orders[i], l.orders[j] = l.orders[j], l.orders[i]
}

func (l orderList) Less(i, j int) bool { return l.sortBy(l.orders[i], l.orders[j]) }

func (l orderList) Insert(o *Order) {
	l.orders = append(l.orders, o)
	sort.Sort(l)
}

func (l orderList) Remove(clOrdID string) (order *Order) {
	for index, order := range l.orders {
		if order.ClOrdID == clOrdID {
			l.orders = append(l.orders[:index], l.orders[index+1:]...)
			return order
		}
	}

	return
}

// bids: the price that the holders are able to sale
func bids() (b orderList) {
	b.sortBy = func(i, j *Order) bool {
		// first sort by order price (higher to lower)
		switch i.Price.Cmp(j.Price) {
		case 1:
			return true
		case -1:
			return false
		}

		// then sort by insert time (earlier to later)
		return i.insertTime.Before(j.insertTime)
	}

	return
}

// offers: the price that the investors will be able to purchase
func offers() (o orderList) {
	o.sortBy = func(i, j *Order) bool {
		// first sort by order price (lower to higher)
		switch i.Price.Cmp(j.Price) {
		case 1:
			return false
		case -1:
			return true
		}

		// then sort by insert time (earlier to later)
		return i.insertTime.Before(j.insertTime)
	}

	return
}

//Market is a simple CLOB
type Market struct {
	Bids   orderList
	Offers orderList
}

//NewMarket returns an initialized Market instance
func NewMarket() *Market {
	return &Market{bids(), offers()}
}

// Display display orders in the market
func (m Market) Display() {
	fmt.Println("BIDS:")
	fmt.Println("-----")
	fmt.Println()

	for _, bid := range m.Bids.orders {
		fmt.Printf("%+v\n", bid)
	}

	fmt.Println()
	fmt.Println("OFFERS:")
	fmt.Println("*****")

	for _, offer := range m.Offers.orders {
		fmt.Printf("%+v\n", offer)
	}
}

// Insert insert order into market from either Buy side or Sell side
func (m *Market) Insert(order Order) {
	order.insertTime = time.Now()
	if order.Side == enum.Side_BUY {
		m.Bids.Insert(&order)
	} else {
		m.Offers.Insert(&order)
	}
}

// Cancel cancel orders in either Buy side or Sell side
func (m *Market) Cancel(clordID string, side enum.Side) (order *Order) {
	if side == enum.Side_BUY {
		order = m.Bids.Remove(clordID)
	} else {
		order = m.Offers.Remove(clordID)
	}

	if order != nil {
		order.Cancel()
	}

	return
}

// Match match the best Bid with best Offer
func (m *Market) Match() (matched []Order) {
	// loop till there is no match in either Bid or Offer side
	for m.Bids.Len() > 0 && m.Offers.Len() > 0 {
		bestBid := m.Bids.orders[0]
		bestOffer := m.Offers.orders[0]

		price := bestOffer.Price
		quantity := bestBid.OpenQuantity()
		if offerQuant := bestOffer.OpenQuantity(); offerQuant.Cmp(quantity) == -1 {
			quantity = offerQuant
		}

		bestBid.Execute(price, quantity)
		bestOffer.Execute(price, quantity)

		matched = append(matched, *bestBid, *bestOffer)

		if bestBid.IsClosed() {
			m.Bids.orders = m.Bids.orders[1:]
		}

		if bestOffer.IsClosed() {
			m.Offers.orders = m.Offers.orders[1:]
		}
	}

	return
}
