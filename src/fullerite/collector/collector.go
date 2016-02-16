package collector

import (
	"fullerite/config"
	"fullerite/metric"

	"strings"

	l "github.com/Sirupsen/logrus"
)

const (
	// DefaultCollectionInterval the interval to collect on unless overridden by a collectors config
	DefaultCollectionInterval = 10
)

var defaultLog = l.WithFields(l.Fields{"app": "fullerite", "pkg": "collector"})

// Collector defines the interface of a generic collector.
type Collector interface {
	Collect()
	Configure(map[string]interface{})

	// taken care of by the base class
	Name() string
	Channel() chan metric.Metric
	Interval() int
	SetInterval(int)
	CollectorType() string
	SetCollectorType(string)
	CanonicalName() string
	SetCanonicalName(string)
}

// New creates a new Collector based on the requested collector name.
func New(name string) Collector {
	var collector Collector

	channel := make(chan metric.Metric)
	collectorLog := defaultLog.WithFields(l.Fields{"collector": name})
	// This allows for initiating multiple collectors of the same type
	// but with a different canonical name so they can receive different
	// configs
	realName := strings.Split(name, " ")[0]

	switch realName {
	case "Test":
		collector = NewTest(channel, DefaultCollectionInterval, collectorLog)
	case "Diamond":
		collector = newDiamond(channel, DefaultCollectionInterval, collectorLog)
		collector.SetCollectorType("listener")
	case "Fullerite":
		collector = newFullerite(channel, DefaultCollectionInterval, collectorLog)
	case "ProcStatus":
		collector = newProcStatus(channel, DefaultCollectionInterval, collectorLog)
	case "FulleriteHTTP":
		collector = newFulleriteHTTP(channel, DefaultCollectionInterval, collectorLog)
		collector.SetCollectorType("listener")
	case "NerveUWSGI":
		collector = newNerveUWSGI(channel, DefaultCollectionInterval, collectorLog)
	case "DockerStats":
		collector = newDockerStats(channel, DefaultCollectionInterval, collectorLog)
	case "CPUInfo":
		collector = newCPUInfo(channel, DefaultCollectionInterval, collectorLog)
	case "MesosStats":
		collector = newMesosStats(channel, DefaultCollectionInterval, collectorLog)
	case "MesosSlaveStats":
		collector = newMesosSlaveStats(channel, DefaultCollectionInterval, collectorLog)
	case "MySQLBinlogGrowth":
		collector = newMySQLBinlogGrowth(channel, DefaultCollectionInterval, collectorLog)
	case "AdHoc":
		collector = newAdHoc(channel, DefaultCollectionInterval, collectorLog)
		collector.SetCollectorType("listener")
	default:
		defaultLog.Error("Cannot create collector: ", realName)
		return nil
	}
	if collector.CollectorType() == "" {
		collector.SetCollectorType("collector")
	}
	collector.SetCanonicalName(name)
	return collector
}

type baseCollector struct {
	// fulfill most of the rote parts of the collector interface
	channel       chan metric.Metric
	name          string
	interval      int
	collectorType string
	canonicalName string

	// intentionally exported
	log *l.Entry
}

func (col *baseCollector) configureCommonParams(configMap map[string]interface{}) {
	if interval, exists := configMap["interval"]; exists {
		col.interval = config.GetAsInt(interval, DefaultCollectionInterval)
	}
}

// SetInterval : set the interval to collect on
func (col *baseCollector) SetInterval(interval int) {
	col.interval = interval
}

// SetCollectorType : collector type
func (col *baseCollector) SetCollectorType(collectorType string) {
	col.collectorType = collectorType
}

// SetCanonicalName : collector canonical name
func (col *baseCollector) SetCanonicalName(name string) {
	col.canonicalName = name
}

// CanonicalName : collector canonical name
func (col *baseCollector) CanonicalName() string {
	return col.canonicalName
}

// CollectorType : collector type
func (col *baseCollector) CollectorType() string {
	return col.collectorType
}

// Channel : the channel on which the collector should send metrics
func (col baseCollector) Channel() chan metric.Metric {
	return col.channel
}

// Name : the name of the collector
func (col baseCollector) Name() string {
	return col.name
}

// Interval : the interval to collect the metrics on
func (col baseCollector) Interval() int {
	return col.interval
}

// String returns the collector name in printable format.
func (col baseCollector) String() string {
	return col.Name() + "Collector"
}
