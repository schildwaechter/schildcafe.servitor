// SchildCafé Servitør
// Copyright Carsten Thiel 2023
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofrs/uuid"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	log "github.com/sirupsen/logrus"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type order struct {
	ID string `json:"orderId" gorm:"type:varchar(50);primaryKey"`
	// Timestamp when the order was received
	OrderReceived time.Time `json:"orderReceived"`
	// Timestamp when the order was ready (0 before that)
	OrderReady time.Time `json:"orderReady" gorm:"default:null"`
	// Timestamp when the order was retrieved (0 before that)
	OrderRetrieved time.Time `json:"orderRetrieved" gorm:"default:null"`
	// Total number of coffees in the order
	OrderSize int `json:"orderSize"`
	// Number of coffees brewed so far
	OrderBrewed int `json:"orderBrewed"`
}

// incoming order submission
type orderSubmission struct {
	// (optional) orderID
	ID      string       `json:"orderId" gorm:"primaryKey"`
	Coffees []orderEntry `json:"coffeeOrder"`
}

// A single entry in an order
type orderEntry struct {
	Product string `json:"product" example:"espresso"`
	Count   int    `json:"count" example:"2"`
}

type coffeeListItem struct {
	ID            string `json:"jobID" gorm:"type:varchar(50);primary_key"`
	Product       string `json:"coffeeProduct"`
	OrderID       string `json:"orderId"`
	Order         order
	OrderReceived time.Time `json:"orderReceived"`
	Machine       string    `json:"machine" gorm:"default:null"`
	JobStarted    time.Time `json:"jobStarted" gorm:"default:null"`
	JobReady      time.Time `json:"jobReady" gorm:"default:null"`
	JobRetrieved  time.Time `json:"jobRetrieved" gorm:"default:null"`
}

// Initially empty lists
var orderList = []order{}
var coffeeList = []coffeeListItem{}

// check overall system status
func systemStatus() (int, int, string) {
	var systemStatusCode int
	var systemHTTPStatusCode int
	var systemStatusMessage string

	systemStatusCode = 0
	systemHTTPStatusCode = http.StatusOK
	systemStatusMessage = "System Ready!"

	return systemStatusCode, systemHTTPStatusCode, systemStatusMessage
}

func listOrders(ctx context.Context, tracer trace.Tracer) []order {
	ctx, span := tracer.Start(ctx, "listOrders")
	defer span.End()

	db.WithContext(ctx).Find(&orderList)
	return orderList
}

func getStats() (int, int, int, int) {

	var ordersReceivedInt, ordersReadyInt, ordersRetrievedInt, jobQueueLengthInt int64

	db.Model(&order{}).Where("order_received IS NOT NULL").Count(&ordersReceivedInt)
	db.Model(&order{}).Where("order_ready IS NOT NULL").Count(&ordersReadyInt)
	db.Model(&order{}).Where("order_retrieved IS NOT NULL").Count(&ordersRetrievedInt)
	db.Model(&coffeeListItem{}).Where("job_retrieved IS NULL").Count(&jobQueueLengthInt)

	return int(ordersReceivedInt), int(ordersReadyInt), int(ordersRetrievedInt), int(jobQueueLengthInt)
}

// create a new order
func newOrder(ctx context.Context, tracer trace.Tracer, sentOrderID string, orderedCoffees []orderEntry) (string, bool, int, string) {
	systemStatusCode, systemHTTPStatusCode, systemStatusMessage := systemStatus()

	ctx, span := tracer.Start(ctx, "NewOrder")
	defer span.End()

	if !(systemStatusCode == 0) {
		return "", false, systemHTTPStatusCode, systemStatusMessage
	}

	var newOrder order
	myOrderIDUUID, _ := uuid.NewV4()
	newOrder.ID = myOrderIDUUID.String()
	newOrder.OrderReceived = time.Now().UTC()

	var newOrderSize int = 0
	for _, item := range orderedCoffees {
		newOrderSize += item.Count
	}

	span.SetAttributes(attribute.String("orderUUID", myOrderIDUUID.String()))
	span.AddEvent("Creating Order in database")

	newOrder.OrderSize = newOrderSize
	db.WithContext(ctx).Create(&newOrder)

	for _, item := range orderedCoffees {
		for i := 0; i < item.Count; i++ {
			span.AddEvent("adding item in database")
			var newCoffee coffeeListItem
			myCoffeeIDUUID, _ := uuid.NewV4()
			newCoffee.ID = myCoffeeIDUUID.String()
			newCoffee.OrderID = newOrder.ID
			newCoffee.Product = item.Product
			newCoffee.OrderReceived = newOrder.OrderReceived
			coffeeList = append(coffeeList, newCoffee)
			db.WithContext(ctx).Create(&newCoffee)
			log.Info(newCoffee)
		}
	}

	return newOrder.ID, true, systemHTTPStatusCode, ""
}

func retrieveOrder(ctx context.Context, tracer trace.Tracer, id string) (*order, bool, int, string) {
	ctx, span := tracer.Start(ctx, "retrieveOrder")
	defer span.End()

	systemStatusCode, systemHTTPStatusCode, systemStatusMessage := systemStatus()

	if !(systemStatusCode == 0) {
		return nil, false, systemHTTPStatusCode, systemStatusMessage
	}

	var thisOrder = order{ID: id}
	result := db.WithContext(ctx).Limit(1).Find(&thisOrder)

	if !(result.RowsAffected == 1) {
		return nil, false, http.StatusNotFound, "Order not found!"
	}

	if !(thisOrder.OrderRetrieved.IsZero()) {
		return nil, false, http.StatusGone, "Order already delivered"
	}

	if thisOrder.OrderSize == thisOrder.OrderBrewed {
		thisOrder.OrderRetrieved = time.Now().UTC()
		db.WithContext(ctx).Save(&thisOrder)
		return &thisOrder, true, http.StatusOK, "Order delivered"
	}

	return nil, false, http.StatusServiceUnavailable, "Order not ready"
}
