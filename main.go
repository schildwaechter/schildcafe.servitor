// SchildCafé Servitør
// Copyright Carsten Thiel 2023
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/atarantini/ginrequestid"
	formatters "github.com/fabienm/go-logrus-formatters"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

// Success response
// swagger:response succResp
type swaggSuccResp struct {
	// in:body
	Body struct {
		// Detailed message
		Message string `json:"message"`
	}
}

func myRequestLogger(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		path := c.Request.URL.Path

		start := time.Now()

		log.Debug(c.Request)

		log.WithFields(logrus.Fields{
			"method":    c.Request.Method,
			"requestId": c.MustGet("RequestId"),
			"endpoint":  path,
		}).Debug("request received")

		c.Next()

		elapsed := time.Since(start)

		log.WithFields(logrus.Fields{
			"method":       c.Request.Method,
			"requestId":    c.MustGet("RequestId"),
			"responsetime": elapsed,
			"endpoint":     path,
		}).Debug("request answered")

	}
}

func setupRouter() *gin.Engine {
	r := gin.New()

	r.Use(ginrequestid.RequestId())

	log := logrus.New()

	_, exists := os.LookupEnv("GELF_LOGGING")
	if exists {
		hostname, _ := os.Hostname()
		log.SetFormatter(formatters.NewGelf(hostname))
	}

	if gin.Mode() == "debug" {
		log.Level = logrus.DebugLevel
	} else {
		log.Level = logrus.InfoLevel
	}

	r.Use(myRequestLogger(log), gin.Recovery())

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Welcome to the SchildCafé!")
	})

	r.GET("/order-list", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": listOrders()})
	})

	r.GET("/metrics", func(c *gin.Context) {

		ordersReceivedInt, ordersReadyInt, ordersRetrievedInt, jobQueueLengthInt := getStats()

		ordersReceivedString := "# HELP orders_received The numbers of orders received by the system\n# TYPE orders_received counter\norders_received " + strconv.Itoa(ordersReceivedInt)
		ordersReadyString := "# HELP orders_ready The numbers of orders the system has finished\n# TYPE orders_ready counter\norders_ready " + strconv.Itoa(ordersReadyInt)
		ordersRetrievedString := "# HELP orders_retrieved The numbers of orders retrieved from the system\n# TYPE orders_retrieved counter\norders_retrieved " + strconv.Itoa(ordersRetrievedInt)
		jobQueueLengthString := "#HELP job_queue_length The number of jobs currently in the queue\n#TYPE job_queue_length gauge\njob_queue_length " + strconv.Itoa(jobQueueLengthInt)

		c.String(http.StatusOK, ordersReceivedString+"\n"+ordersReadyString+"\n"+ordersRetrievedString+"\n"+jobQueueLengthString)

	})

	// swagger:route GET /healthcheck healthReq
	// Perform a healthcheck.
	// If all is fine, this will tell you.
	// responses:
	//  200: succResp
	r.GET("/healthcheck", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Ok"})
	})

	r.POST("/submit-order", func(c *gin.Context) {
		var incomingOrder orderSubmission
		c.BindJSON(&incomingOrder)

		//c.IndentedJSON(http.StatusOK, incomingOrder)

		result, success, systemHTTPStatusCode, systemStatusMessage := newOrder(incomingOrder.ID, incomingOrder.Coffees)

		if !success {
			log.WithFields(logrus.Fields{
				"requestId": c.MustGet("RequestId"),
				"errorCode": systemHTTPStatusCode,
			}).Error("Error accepting order: " + systemStatusMessage)
			c.JSON(systemHTTPStatusCode, gin.H{"message": systemStatusMessage})
		} else {
			log.WithFields(logrus.Fields{
				"requestId": c.MustGet("RequestId"),
				"OrderID":   result,
			}).Info("Order accepted")
			c.JSON(http.StatusOK, result)
		}
	})

	r.GET("/retrieve-order/:ID", func(c *gin.Context) {
		ID := c.Param("ID")

		result, success, systemHTTPStatusCode, systemStatusMessage := retrieveOrder(ID)

		if !success {
			log.WithFields(logrus.Fields{
				"requestId": c.MustGet("RequestId"),
				"errorCode": systemHTTPStatusCode,
			}).Error("Error retrieving order: " + systemStatusMessage)
			c.JSON(systemHTTPStatusCode, gin.H{"message": systemStatusMessage})
		} else {
			log.WithFields(logrus.Fields{
				"requestId": c.MustGet("RequestId"),
				"OrderID":   result,
			}).Info("Order retrieved")
			c.JSON(http.StatusOK, result)
		}

	})

	return r
}

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

var db, dbErr = gorm.Open("mysql", getEnv("MYSQL_USER", "root")+":"+getEnv("MYSQL_PASS", "root")+"@tcp("+getEnv("MYSQL_HOST", "localhost")+":"+getEnv("MYSQL_PORT", "3306")+")/"+getEnv("MYSQL_DB", "cafe")+"?charset=utf8&parseTime=True&loc=Local")

func main() {
	if dbErr != nil {
		log.WithFields(logrus.Fields{}).Error("DB rror")
		panic("failed to connect database")
	}
	defer db.Close()

	//db.Debug().DropTableIfExists(&coffeeListItem{})
	//db.Debug().DropTableIfExists(&order{})
	db.Debug().AutoMigrate(&coffeeListItem{})
	db.Debug().AutoMigrate(&order{})

	runPort := getEnv("SERVITOR_PORT", "1333")

	r := setupRouter()
	r.Run(":" + runPort)
}
