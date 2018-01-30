package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"

	"github.com/quickfixgo/enum"
	"github.com/quickfixgo/field"
	"github.com/quickfixgo/fix40/executionreport"
	"github.com/quickfixgo/fix40/newordersingle"
	"github.com/quickfixgo/fix40/ordercancelrequest"
	"github.com/quickfixgo/fix42/marketdatarequest"
	"github.com/quickfixgo/quickfix"
)

//Application implements the quickfix.Application interface
type Application struct {
	*quickfix.MessageRouter
	*internal.OrderMatcher
	execID int
}

func newApplication() *Application {
	app := &Application{
		MessageRouter: quickfix.NewMessageRouter(),
		OrderMatcher:  internal.NewOrderMatcher(),
	}
	app.AddRoute(newordersingle.Route(app.onNewOrderSingle))
	app.AddRoute(ordercancelrequest.Route(app.onOrderCancelRequest))
	app.AddRoute(marketdatarequest.Route(app.onMarketDataRequest))

	return app
}

//OnCreate implemented as part of Application interface
func (a Application) OnCreate(sessionID quickfix.SessionID) { return }

//OnLogon implemented as part of Application interface
func (a Application) OnLogon(sessionID quickfix.SessionID) { return }

//OnLogout implemented as part of Application interface
func (a Application) OnLogout(sessionID quickfix.SessionID) { return }

//ToAdmin implemented as part of Application interface
func (a Application) ToAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) { return }

//ToApp implemented as part of Application interface
func (a Application) ToApp(msg *quickfix.Message, sessionID quickfix.SessionID) error {
	return nil
}

//FromAdmin implemented as part of Application interface
func (a Application) FromAdmin(msg *quickfix.Message, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	return nil
}

//FromApp implemented as part of Application interface, uses Router on incoming application messages
func (a *Application) FromApp(msg *quickfix.Message, sessionID quickfix.SessionID) (reject quickfix.MessageRejectError) {
	return a.Route(msg, sessionID)
}

func (a *Application) onNewOrderSingle(msg newordersingle.NewOrderSingle, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	clOrdID, err := msg.GetClOrdID()
	if err != nil {
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		return err
	}

	senderCompID, err := msg.Header.GetSenderCompID()
	if err != nil {
		return err
	}

	targetCompID, err := msg.Header.GetTargetCompID()
	if err != nil {
		return err
	}

	side, err := msg.GetSide()
	if err != nil {
		return err
	}

	ordType, err := msg.GetOrdType()
	if err != nil {
		return err
	}

	price, err := msg.GetPrice()
	if err != nil {
		return err
	}

	orderQty, err := msg.GetOrderQty()
	if err != nil {
		return err
	}

	order := internal.Order{
		ClOrdID:      clOrdID,
		Symbol:       symbol,
		SenderCompID: senderCompID,
		TargetCompID: targetCompID,
		Side:         side,
		OrdType:      ordType,
		Price:        price,
		Quantity:     orderQty,
	}

	a.Insert(order)
	a.acceptOrder(order)

	matches := a.Match(order.Symbol)

	for len(matches) > 0 {
		a.fillOrder(matches[0])
		matches = matches[1:]
	}

	return nil
}

func (a *Application) onOrderCancelRequest(msg ordercancelrequest.OrderCancelRequest, sessionID quickfix.SessionID) quickfix.MessageRejectError {
	origClOrdID, err := msg.GetOrigClOrdID()
	if err != nil {
		return err
	}

	symbol, err := msg.GetSymbol()
	if err != nil {
		return err
	}

	side, err := msg.GetSide()
	if err != nil {
		return err
	}

	order := a.Cancel(origClOrdID, symbol, side)
	if order != nil {
		a.cancelOrder(*order)
	}

	return nil
}

func (a *Application) onMarketDataRequest(msg marketdatarequest.MarketDataRequest, sessionID quickfix.SessionID) (err quickfix.MessageRejectError) {
	fmt.Printf("%+v\n", msg)
	return
}

func (a *Application) acceptOrder(order internal.Order) {
	a.updateOrder(order, enum.OrdStatus_NEW)
}

func (a *Application) fillOrder(order internal.Order) {
	status := enum.OrdStatus_FILLED
	if !order.IsClosed() {
		status = enum.OrdStatus_PARTIALLY_FILLED
	}
	a.updateOrder(order, status)
}

func (a *Application) cancelOrder(order internal.Order) {
	a.updateOrder(order, enum.OrdStatus_CANCELED)
}

func (a *Application) genExecID() string {
	a.execID++
	return strconv.Itoa(a.execID)
}

func (a *Application) updateOrder(order internal.Order, status enum.OrdStatus) {
	execReport := executionreport.New(
		field.NewOrderID(order.ClOrdID),
		field.NewExecID(a.genExecID()),
		field.NewExecTransType(enum.ExecTransType_NEW),
		field.NewExecType(enum.ExecType(status)),
		field.NewOrdStatus(status),
		field.NewSymbol(order.Symbol),
		field.NewSide(order.Side),
		field.NewLeavesQty(order.OpenQuantity(), 2),
		field.NewCumQty(order.ExecutedQuantity, 2),
		field.NewAvgPx(order.AvgPx, 2),
	)
	execReport.SetOrderQty(order.Quantity, 2)
	execReport.SetClOrdID(order.ClOrdID)

	switch status {
	case enum.OrdStatus_FILLED, enum.OrdStatus_PARTIALLY_FILLED:
		execReport.SetLastShares(order.LastExecutedQuantity, 2)
		execReport.SetLastPx(order.LastExecutedPrice, 2)
	}

	execReport.Header.SetTargetCompID(order.SenderCompID)
	execReport.Header.SetSenderCompID(order.TargetCompID)

	quickfix.Send(execReport)
}

func main() {
	flag.Parse()

	cfgFileName := path.Join("config", "ordermatch.cfg")
	if flag.NArg() > 0 {
		cfgFileName = flag.Arg(0)
	}

	cfg, err := os.Open(cfgFileName)
	if err != nil {
		fmt.Printf("Error opening %v, %v\n", cfgFileName, err)
		return
	}

	appSettings, err := quickfix.ParseSettings(cfg)
	if err != nil {
		fmt.Println("Error reading cfg,", err)
		return
	}

	logFactory := quickfix.NewScreenLogFactory()
	app := newApplication()

	acceptor, err := quickfix.NewAcceptor(app, quickfix.NewMemoryStoreFactory(), appSettings, logFactory)
	if err != nil {
		fmt.Printf("Unable to create Acceptor: %s\n", err)
		return
	}

	err = acceptor.Start()
	if err != nil {
		fmt.Printf("Unable to start Acceptor: %s\n", err)
		return
	}

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Interrupt, os.Kill)
	go func() {
		<-interrupt
		acceptor.Stop()
		os.Exit(0)
	}()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()

		switch value := scanner.Text(); value {
		case "#symbols":
			app.Display()
		default:
			app.DisplayMarket(value)
		}
	}
}
