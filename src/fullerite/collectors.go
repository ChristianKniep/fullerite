package main

import (
	"fullerite/collector"
	"fullerite/config"
	"fullerite/handler"
	"fullerite/metric"
	"fullerite/util"

	"fmt"
	"strings"
	"time"
)

var metricsBlacklist = make(map[string]string)

func startCollectors(c config.Config) (collectors []collector.Collector) {
	log.Info("Starting collectors...")
	var element string

	for _, name := range c.Collectors {
		configFile := strings.Join([]string{c.CollectorsConfigPath, name}, "/") + ".conf"
		// Since collector naems can be defined with a space in order to instantiate multiple
		// instances of the same collector, we want their files
		// will not have that space and needs to have it replaced with an underscore
		// instead
		configFile = strings.Replace(configFile, " ", "_", -1)
		conf, err := config.ReadCollectorConfig(configFile)
		if err != nil {
			log.Error("Collector config failed to load for: ", name)
			continue
		}
		if asInterface, exists := conf["metrics_blacklist"]; exists {
			for _, element = range config.GetAsSlice(asInterface) {
				metricsBlacklist[element] = name
			}
		}

		collectorInst := startCollector(name, c, conf)
		if collectorInst != nil {
			collectors = append(collectors, collectorInst)
		}
	}
	return collectors
}

func startCollector(name string, globalConfig config.Config, instanceConfig map[string]interface{}) collector.Collector {
	log.Debug("Starting collector ", name)
	collectorInst := collector.New(name)
	if collectorInst == nil {
		return nil
	}

	// apply the global configs
	collectorInst.SetInterval(config.GetAsInt(globalConfig.Interval, collector.DefaultCollectionInterval))

	// apply the instance configs
	collectorInst.Configure(instanceConfig)

	go runCollector(collectorInst)
	return collectorInst
}

func runCollector(collector collector.Collector) {
	log.Info("Running ", collector)

	ticker := time.NewTicker(time.Duration(collector.Interval()) * time.Second)
	collect := ticker.C

	staggerValue := 1
	collectionDeadline := time.Duration(collector.Interval() + staggerValue)

	for {
		select {
		case <-collect:
			if collector.CollectorType() == "listener" {
				collector.Collect()
			} else {
				countdownTimer := time.AfterFunc(collectionDeadline*time.Second, func() {
					reportCollector(collector)
				})
				collector.Collect()
				countdownTimer.Stop()
			}
		}
	}
	ticker.Stop()
}

func readFromCollectors(collectors []collector.Collector,
	handlers []handler.Handler,
	collectorStatChans ...chan<- metric.CollectorEmission) {
	for i := range collectors {
		go readFromCollector(collectors[i], handlers, collectorStatChans...)
	}
}

func readFromCollector(collector collector.Collector,
	handlers []handler.Handler,
	collectorStatChans ...chan<- metric.CollectorEmission) {
	// In case of Diamond collectors, metric from multiple collectors are read
	// from Single channel (owned by Go Diamond Collector) and hence we use a map
	// for keeping track of metrics from each individual collector
	emissionCounter := map[string]uint64{}
	lastEmission := time.Now()
	statDuration := time.Duration(collector.Interval()) * time.Second
	for m := range collector.Channel() {
		var exists bool
		c := collector.CanonicalName()
		if _, exists = m.GetDimensionValue("collector"); !exists {
			m.AddDimension("collector", collector.Name())
		}
		// We allow external collectors to provide us their collector's CanonicalName
		// by sending it as a metric dimension. For example in the case of Diamond the
		// individual python collectors can send their names this way.
		if val, ok := m.GetDimensionValue("collectorCanonicalName"); ok {
			c = val
			m.RemoveDimension("collectorCanonicalName")
		}
		// check if the metric is blacklisted, if so will continue
		// processing the next one
		//log.Error("GIULIA ", metricsBlacklist)
		if util.StringInSlice(m.Name, c, metricsBlacklist) {
			continue
		}
		emissionCounter[c]++
		// collectorStatChans is an optional parameter. In case of ad-hoc collector
		// this parameter is not supplied at all. Using variadic arguments is pretty much
		// only way of doing this in go.
		if len(collectorStatChans) > 0 {
			collectorStatChan := collectorStatChans[0]
			currentTime := time.Now()
			if currentTime.After(lastEmission.Add(statDuration)) {
				emitCollectorStats(emissionCounter, collectorStatChan)
				lastEmission = time.Now()
			}
		}

		if len(collector.Prefix()) > 0 {
			m.Name = collector.Prefix() + m.Name
		}

		for i := range handlers {
			if _, exists := handlers[i].CollectorChannels()[c]; exists {
				handlers[i].CollectorChannels()[c] <- m
			}
		}
	}
	// Closing the stat channel after collector loop finishes
	for _, statChannel := range collectorStatChans {
		close(statChannel)
	}
}

func emitCollectorStats(data map[string]uint64,
	collectorStatChan chan<- metric.CollectorEmission) {
	for collectorName, count := range data {
		collectorStatChan <- metric.CollectorEmission{collectorName, count}
	}
}

func reportCollector(collector collector.Collector) {
	log.Warn(fmt.Sprintf("%s collector took too long to run, reporting incident!", collector.Name()))
	metric := metric.New("fullerite.collection_time_exceeded")
	metric.Value = 1
	metric.AddDimension("interval", fmt.Sprintf("%d", collector.Interval()))
	collector.Channel() <- metric
}
