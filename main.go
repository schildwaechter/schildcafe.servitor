// SchildCafé Servitør
// Copyright Carsten Thiel 2023
//
// SPDX-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/uptrace/opentelemetry-go-extra/otelgorm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	//"go.opentelemetry.io/otel/codes"

	"github.com/atarantini/ginrequestid"
	formatters "github.com/fabienm/go-logrus-formatters"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitTracer(tracesEndpoint string) (*trace.TracerProvider, error) {
	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint(tracesEndpoint), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("servitor"))),
	)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tracerProvider, nil
}

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

// rootHandler godoc
// @Summary	Root endpoint
// @Router	/ [get]
func rootHandler(c *gin.Context) {
	c.String(http.StatusOK, "Welcome to the SchildCafé!")
}

// orderListHandler godoc
// @Summary	Get list of all orders
// @Router	/order-list [get]
func orderListHandler(c *gin.Context) {
	tracer := otel.Tracer("order-list")
	ctx, span := tracer.Start(c, "Order-List Endpoint")
	requestID, _ := c.Get("RequestId")
	span.SetAttributes(attribute.String("RequestID", requestID.(string)))
	defer span.End()
	c.JSON(http.StatusOK, gin.H{"data": listOrders(ctx, tracer)})
}

// metricsHandler godoc
// @Summary	Get Prometheus metrics
// @Router	/metrics [get]
func metricsHandler(c *gin.Context) {

	ordersReceivedInt, ordersReadyInt, ordersRetrievedInt, jobQueueLengthInt := getStats()

	ordersReceivedString := "# HELP orders_received The numbers of orders received by the system\n# TYPE orders_received counter\norders_received " + strconv.Itoa(ordersReceivedInt)
	ordersReadyString := "# HELP orders_ready The numbers of orders the system has finished\n# TYPE orders_ready counter\norders_ready " + strconv.Itoa(ordersReadyInt)
	ordersRetrievedString := "# HELP orders_retrieved The numbers of orders retrieved from the system\n# TYPE orders_retrieved counter\norders_retrieved " + strconv.Itoa(ordersRetrievedInt)
	jobQueueLengthString := "# HELP job_queue_length The number of jobs currently in the queue\n#TYPE job_queue_length gauge\njob_queue_length " + strconv.Itoa(jobQueueLengthInt)

	c.String(http.StatusOK, ordersReceivedString+"\n"+ordersReadyString+"\n"+ordersRetrievedString+"\n"+jobQueueLengthString)

}

// healthcheckHandler godoc
// @Summary	Perform a healthcheck
// @Description	If all is fine, this will tell you.
// @Success 200
// @Router	/healthcheck [get]
func healthcheckHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Ok"})
}

// submitOrderHandler godoc
// @Summary	Submit a new order
// @Router	/submit-order [post]
func submitOrderHandler(c *gin.Context) {
	tracer := otel.Tracer("submit-order")
	ctx, span := tracer.Start(c, "Submit-Order Endpoint")
	requestID, _ := c.Get("RequestId")
	span.SetAttributes(attribute.String("RequestID", requestID.(string)))
	defer span.End()

	var incomingOrder orderSubmission
	c.BindJSON(&incomingOrder)

	//c.IndentedJSON(http.StatusOK, incomingOrder)

	resultID, success, systemHTTPStatusCode, systemStatusMessage := newOrder(ctx, tracer, incomingOrder.ID, incomingOrder.Coffees)

	if !success {
		log.WithFields(logrus.Fields{
			"requestId": c.MustGet("RequestId"),
			"errorCode": systemHTTPStatusCode,
		}).Error("Error accepting order: " + systemStatusMessage)
		c.JSON(systemHTTPStatusCode, gin.H{"message": systemStatusMessage})
	} else {
		log.WithFields(logrus.Fields{
			"requestId": c.MustGet("RequestId"),
			"OrderID":   resultID,
		}).Info("Order " + resultID + " accepted")
		c.JSON(http.StatusOK, resultID)
	}
}

// retrieveOrderHandler godoc
// @Summary	Retrieve an order
// @Router	/retrieve-order/{ID} [get]
func retrieveOrderHandler(c *gin.Context) {
	tracer := otel.Tracer("submit-order")
	ctx, span := tracer.Start(c, "Submit-Order Endpoint")
	requestID, _ := c.Get("RequestId")
	span.SetAttributes(attribute.String("RequestID", requestID.(string)))
	defer span.End()

	ID := c.Param("ID")

	result, success, systemHTTPStatusCode, systemStatusMessage := retrieveOrder(ctx, tracer, ID)

	if !success {
		log.WithFields(logrus.Fields{
			"requestId": c.MustGet("RequestId"),
			"errorCode": systemHTTPStatusCode,
		}).Error("Error retrieving order " + ID + ": " + systemStatusMessage)
		c.JSON(systemHTTPStatusCode, gin.H{"message": systemStatusMessage})
	} else {
		log.WithFields(logrus.Fields{
			"requestId": c.MustGet("RequestId"),
			"OrderID":   result,
		}).Info("Order " + ID + " retrieved")
		c.JSON(http.StatusOK, result)
	}

}

// Get environment variable with a default
func getEnv(name string, defaultValue string) string {
	value, exists := os.LookupEnv(name)
	if exists {
		return value
	}
	return defaultValue
}

var dsn = getEnv("MYSQL_USER", "root") + ":" + getEnv("MYSQL_PASS", "root") +
	"@tcp(" + getEnv("MYSQL_HOST", "localhost") + ":" + getEnv("MYSQL_PORT", "3306") + ")/" +
	getEnv("MYSQL_DB", "cafe") + "?charset=utf8mb4&parseTime=True&loc=Local"
var db, dbErr = gorm.Open(mysql.Open(dsn), &gorm.Config{})

// @title	SchildCafé Servitør
// @license.name	Apache-2.0
func main() {

	if dbErr != nil {
		log.WithFields(logrus.Fields{}).Error("DB error")
		panic("failed to connect database")
	}

	err := db.Use(otelgorm.NewPlugin())
	if err != nil {
		panic("fatal error")
	}

	tracesEndpoint, ok := os.LookupEnv("OTEL_TRACES_ENDPOINT")
	if ok {
		tp, err := InitTracer(tracesEndpoint)
		if err != nil {
			log.WithFields(logrus.Fields{}).Error("Can't send traces")
		}
		defer func() {
			_ = tp.Shutdown(context.Background())
		}()
		log.WithFields(logrus.Fields{}).Info("Sending traces to " + tracesEndpoint)
	} else {
		log.WithFields(logrus.Fields{}).Info("Not sending traces")
	}

	//db.Debug().DropTableIfExists(&coffeeListItem{})
	//db.Debug().DropTableIfExists(&order{})
	db.Debug().AutoMigrate(&coffeeListItem{})
	db.Debug().AutoMigrate(&order{})

	runPort := getEnv("SERVITOR_PORT", "1333")

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

	r.GET("/", rootHandler)
	r.GET("/order-list", orderListHandler)
	r.GET("/metrics", metricsHandler)
	r.GET("/healthcheck", healthcheckHandler)
	r.POST("/submit-order", submitOrderHandler)
	r.GET("/retrieve-order/:ID", retrieveOrderHandler)

	r.Run(":" + runPort)
}
