// SchildCafé Servitør
// Copyright Carsten Thiel 2023
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofrs/uuid"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	log "github.com/sirupsen/logrus"
)

type order struct {
	ID             string    `json:"orderId" gorm:"primaryKey"`
	OrderReceived  time.Time `json:"orderReceived"`
	OrderDelivered time.Time `json:"orderDelivered" gorm:"default:null"`
	OrderSize      int       `json:"orderSize"`
}

type orderSubmission struct {
	ID      string       `json:"orderId" gorm:"primaryKey"`
	Coffees []orderEntry `json:"coffeeOrder"`
}

type orderEntry struct {
	Product string `json:"product"`
	Count   int    `json:"count"`
}

type coffeeListItem struct {
	ID            int    `gorm:"primary_key"`
	Product       string `json:"coffeeProduct"`
	OrderID       string `json:"orderId"`
	Order         order
	OrderReceived time.Time `json:"orderReceived"`
	OrderSize     int       `json:"orderSize"`
	Machine       string    `json:"machine" gorm:"default:null"`
	JobID         string    `json:"jobID"`
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

// create a new order
func newOrder(sentOrderID string, orderedCoffees []orderEntry) (string, bool, int, string) {
	systemStatusCode, systemHTTPStatusCode, systemStatusMessage := systemStatus()

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

	newOrder.OrderSize = newOrderSize
	orderList = append(orderList, newOrder)
	db.Create(&newOrder)

	for _, item := range orderedCoffees {
		for i := 0; i < item.Count; i++ {
			var newCoffee coffeeListItem
			myCoffeeIDUUID, _ := uuid.NewV4()
			newCoffee.JobID = myCoffeeIDUUID.String()
			newCoffee.OrderID = newOrder.ID
			newCoffee.Product = item.Product
			newCoffee.OrderSize = newOrderSize
			newCoffee.OrderReceived = newOrder.OrderReceived
			coffeeList = append(coffeeList, newCoffee)
			db.Create(&newCoffee)
			log.WithFields(log.Fields{
				"animal": "walrus",
			}).Info(newCoffee)
		}
	}

	return newOrder.ID, true, systemHTTPStatusCode, ""
}

func retrieveOrder(id string) (*order, bool, int, string) {
	systemStatusCode, systemHTTPStatusCode, systemStatusMessage := systemStatus()

	if !(systemStatusCode == 0) {
		return nil, false, systemHTTPStatusCode, systemStatusMessage
	}

	var thisOrder = order{ID: id}
	result := db.Limit(1).Find(&thisOrder)

	if !(result.RowsAffected == 1) {
		return nil, false, http.StatusNotFound, "Order not found!"
	}

	if !(thisOrder.OrderDelivered.IsZero()) {
		return nil, false, http.StatusGone, "Order already delivered"
	}

	var count int
	db.Model(&coffeeListItem{}).Where(&coffeeListItem{OrderID: id}).Where("job_retrieved IS NOT NULL").Count(&count)

	if thisOrder.OrderSize == count {
		thisOrder.OrderDelivered = time.Now().UTC()
		db.Save(&thisOrder)
		return &thisOrder, true, http.StatusOK, "Order delivered"
	}

	return nil, false, http.StatusServiceUnavailable, "Order not ready"
}
