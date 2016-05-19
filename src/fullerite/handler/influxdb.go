package handler

import (
	"fmt"
	"fullerite/metric"
	"time"

	l "github.com/Sirupsen/logrus"
	influxClient "github.com/influxdata/influxdb/client/v2"
)

func init() {
	RegisterHandler("InfluxDB", newInfluxDB)
}

// InfluxDB type
type InfluxDB struct {
	BaseHandler
	server   string
	port     string
	database string
	username string
	password string
}

// newInfluxDB returns a new InfluxDB handler.
func newInfluxDB(
	channel chan metric.Metric,
	initialInterval int,
	initialBufferSize int,
	initialTimeout time.Duration,
	log *l.Entry) Handler {
	inst := new(InfluxDB)
	inst.name = "InfluxDB"
	inst.interval = initialInterval
	inst.maxBufferSize = initialBufferSize
	inst.timeout = initialTimeout
	inst.log = log
	inst.channel = channel

	return inst
}

// Server returns the InfluxDB server's name or IP
func (i InfluxDB) Server() string {
	return i.server
}

// Port returns the InfluxDB server's port number
func (i InfluxDB) Port() string {
	return i.port
}

// Configure accepts the different configuration options for the InfluxDB handler
func (i *InfluxDB) Configure(configMap map[string]interface{}) {
	if server, exists := configMap["server"]; exists {
		i.server = server.(string)
	} else {
		i.log.Error("There was no server specified for the InfluxDB Handler, there won't be any emissions")
	}

	if port, exists := configMap["port"]; exists {
		i.port = fmt.Sprint(port)
	} else {
		i.log.Error("There was no port specified for the InfluxDB Handler, there won't be any emissions")
	}
	if username, exists := configMap["username"]; exists {
		i.username = username.(string)
	} else {
		i.log.Error("There was no user specified for the InfluxDB Handler, there won't be any emissions")
	}
	if password, exists := configMap["password"]; exists {
		i.password = password.(string)
	} else {
		i.log.Error("There was no password specified for the InfluxDB Handler, there won't be any emissions")
	}
	if database, exists := configMap["database"]; exists {
		i.database = database.(string)
	} else {
		i.log.Error("There was no database specified for the InfluxDB Handler, there won't be any emissions")
	}
	i.configureCommonParams(configMap)
}

// Run runs the handler main loop
func (i *InfluxDB) Run() {
	i.run(i.emitMetrics)
}

func (i InfluxDB) createDatapoint(incomingMetric metric.Metric) (datapoint *influxClient.Point) {
	tags := incomingMetric.GetDimensions(i.DefaultDimensions())
	// Assemble field (could be improved to convey multiple fields)
	fields := map[string]interface{}{
		"value": incomingMetric.Value,
	}
	pt, _ := influxClient.NewPoint(incomingMetric.Name, tags, fields, time.Now())
	return pt
}

func (i *InfluxDB) emitMetrics(metrics []metric.Metric) bool {
	i.log.Info("Starting to emit ", len(metrics), " metrics")

	if len(metrics) == 0 {
		i.log.Warn("Skipping send because of an empty payload")
		return false
	}

	// Make client
	addr := fmt.Sprintf("http://%s:%s", i.server, i.port)
	c, err := influxClient.NewHTTPClient(influxClient.HTTPConfig{
		Addr:     addr,
		Username: i.username,
		Password: i.password,
	})
	if err != nil {
		i.log.Warn("Not able to connect to DB: ", err)
	} else {
		i.log.Debug("Connected to ", addr, ", using '", i.database, "' database")
	}
	// Create a new point batch to be send in bulk
	bp, _ := influxClient.NewBatchPoints(influxClient.BatchPointsConfig{
		Database:  i.database,
		Precision: "s",
	})

	//iterate over metrics
	for _, m := range metrics {
		bp.AddPoint(i.createDatapoint(m))
	}

	// Write the batch
	c.Write(bp)
	return true
}
