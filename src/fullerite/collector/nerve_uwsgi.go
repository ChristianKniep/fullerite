package collector

import (
	"fullerite/config"
	"fullerite/metric"
	"fullerite/util"

	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	l "github.com/Sirupsen/logrus"
)

const (
	// MetricTypeCounter String for counter metric type
	MetricTypeCounter string = "COUNTER"
	// MetricTypeGauge String for Gauge metric type
	MetricTypeGauge string = "GAUGE"
)

// This is a collector that will parse a config file on collection
// that is formatted in the standard nerve way. Then it will query
// all the local endpoints on the given path. The metrics are assumed
// to be in a UWSGI format.
//
// the assumed format is:
// {
// 	"gauges": {},
// 	"histograms": {},
// 	"version": "xxx",
// 	"timers": {
// 		"pyramid_uwsgi_metrics.tweens.status.metrics": {
// 			"count": ###,
// 			"p98": ###,
// 			...
// 		},
// 		"pyramid_uwsgi_metrics.tweens.lookup": {
// 			"count": ###,
// 			...
// 		}
// 	},
// 	"meters": {
// 		"pyramid_uwsgi_metrics.tweens.XXX": {
//			"count": ###,
//			"mean_rate": ###,
// 			"m1_rate": ###
// 		}
// 	},
// 	"counters": {
//		"myname": {
//			"count": ###,
// 	}
// }
type uwsgiJSONFormat1X struct {
	ServiceDims map[string]interface{} `json:"service_dims"`
	Counters    map[string]map[string]interface{}
	Gauges      map[string]map[string]interface{}
	Histograms  map[string]map[string]interface{}
	Meters      map[string]map[string]interface{}
	Timers      map[string]map[string]interface{}
}

// For parsing Dropwizard json output
type nestedMetricMap struct {
	metricSegments []string
	metricMap      map[string]interface{}
}

type nerveUWSGICollector struct {
	baseCollector

	configFilePath    string
	queryPath         string
	timeout           int
	servicesWhitelist []string
}

// Parser map for schema matching
var schemaMap map[string]func(*[]byte, bool) ([]metric.Metric, error)

func init() {
	RegisterCollector("NerveUWSGI", newNerveUWSGI)
	// Enumerate schema-parser map:
	schemaMap = make(map[string]func(*[]byte, bool) ([]metric.Metric, error))
	schemaMap["uwsgi.1.0"] = parseUWSGIMetrics10
	schemaMap["uwsgi.1.1"] = parseUWSGIMetrics11
	schemaMap["java-1.1"] = parseJavaMetrics
	schemaMap["default"] = parseDefault
}

func newNerveUWSGI(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	col := new(nerveUWSGICollector)

	col.log = log
	col.channel = channel
	col.interval = initialInterval

	col.name = "NerveUWSGI"
	col.configFilePath = "/etc/nerve/nerve.conf.json"
	col.queryPath = "status/metrics"
	col.timeout = 2

	return col
}

func (n *nerveUWSGICollector) Configure(configMap map[string]interface{}) {
	if val, exists := configMap["queryPath"]; exists {
		n.queryPath = val.(string)
	}
	if val, exists := configMap["configFilePath"]; exists {
		n.configFilePath = val.(string)
	}
	if val, exists := configMap["servicesWhitelist"]; exists {
		n.servicesWhitelist = config.GetAsSlice(val)
	}

	n.configureCommonParams(configMap)
}

func (n *nerveUWSGICollector) Collect() {
	rawFileContents, err := ioutil.ReadFile(n.configFilePath)
	if err != nil {
		n.log.Warn("Failed to read the contents of file ", n.configFilePath, " because ", err)
		return
	}

	servicePortMap, err := util.ParseNerveConfig(&rawFileContents)
	if err != nil {
		n.log.Warn("Failed to parse the nerve config at ", n.configFilePath, ": ", err)
		return
	}
	n.log.Debug("Finished parsing Nerve config into ", servicePortMap)

	for port, service := range servicePortMap {
		go n.queryService(service.Name, port)
	}
}

func (n *nerveUWSGICollector) queryService(serviceName string, port int) {
	serviceLog := n.log.WithField("service", serviceName)

	endpoint := fmt.Sprintf("http://localhost:%d/%s", port, n.queryPath)
	serviceLog.Debug("making GET request to ", endpoint)

	rawResponse, schemaVer, err := queryEndpoint(endpoint, n.timeout)
	if err != nil {
		serviceLog.Warn("Failed to query endpoint ", endpoint, ": ", err)
		return
	}
	metrics, err := schemaMap[schemaVer](&rawResponse, n.serviceInWhitelist(serviceName))
	if err != nil {
		serviceLog.Warn("Failed to parse response into metrics: ", err)
		return
	}

	metric.AddToAll(&metrics, map[string]string{
		"service": serviceName,
		"port":    strconv.Itoa(port),
	})

	serviceLog.Debug("Sending ", len(metrics), " to channel")
	for _, m := range metrics {
		n.Channel() <- m
	}
}

func queryEndpoint(endpoint string, timeout int) ([]byte, string, error) {
	client := http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	rsp, err := client.Get(endpoint)

	if rsp != nil {
		defer func() {
			io.Copy(ioutil.Discard, rsp.Body)
			rsp.Body.Close()
		}()
	}

	if err != nil {
		return []byte{}, "", err
	}

	if rsp != nil && rsp.StatusCode != 200 {
		err := fmt.Errorf("%s returned %d error code", endpoint, rsp.StatusCode)
		return []byte{}, "", err
	}

	schemaVer := rsp.Header.Get("Metrics-Schema")
	if schemaVer == "" {
		schemaVer = "default"
	}

	txt, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return []byte{}, "", err
	}

	return txt, schemaVer, nil
}

// parseDefault is the fallback parser if no 'Metrics-Schema' is provided in the
// response header from a service query
func parseDefault(raw *[]byte, cumulCounterEnabled bool) ([]metric.Metric, error) {
	results, err := parseUWSGIMetrics10(raw, cumulCounterEnabled)
	if err != nil {
		return results, err
	}

	if len(results) == 0 {
		// If parsing using UWSGI format did not work, the output is probably
		// in Dropwizard format and should be handled as such.
		return parseDropwizardMetrics(raw)
	}
	return results, nil
}

// parseUWSGIMetrics10 takes the json returned from the endpoint and converts
// it into raw metrics. We first check that the metrics returned have a float value
// otherwise we skip the metric.
func parseUWSGIMetrics10(raw *[]byte, cumulCounterEnabled bool) ([]metric.Metric, error) {
	parsed := new(uwsgiJSONFormat1X)

	err := json.Unmarshal(*raw, parsed)
	if err != nil {
		return []metric.Metric{}, err
	}

	results := []metric.Metric{}
	appendIt := func(metrics []metric.Metric, typeDimVal string) {
		metric.AddToAll(&metrics, map[string]string{"type": typeDimVal})
		results = append(results, metrics...)
	}

	appendIt(convertToMetrics(&parsed.Gauges, metric.Gauge, false), "gauge")
	appendIt(convertToMetrics(&parsed.Counters, metric.Counter, false), "counter")
	appendIt(convertToMetrics(&parsed.Histograms, metric.Gauge, false), "histogram")
	appendIt(convertToMetrics(&parsed.Meters, metric.Gauge, cumulCounterEnabled), "meter")
	appendIt(convertToMetrics(&parsed.Timers, metric.Gauge, cumulCounterEnabled), "timer")

	return results, nil
}

// parseUWSGIMetrics11 will parse UWSGI metrics under the assumption of
// the response header containing a Metrics-Schema version 'uwsgi.1.1'.
func parseUWSGIMetrics11(raw *[]byte, cumulCounterEnabled bool) ([]metric.Metric, error) {
	parsed := new(uwsgiJSONFormat1X)

	err := json.Unmarshal(*raw, parsed)
	if err != nil {
		return []metric.Metric{}, err
	}

	results := []metric.Metric{}
	appendIt := func(metrics []metric.Metric, typeDimVal string) {
		metric.AddToAll(&metrics, map[string]string{"type": typeDimVal})
		results = append(results, metrics...)
	}

	appendIt(convertToMetrics(&parsed.Gauges, metric.Gauge, false), "gauge")
	appendIt(convertToMetrics(&parsed.Counters, metric.Counter, false), "counter")
	appendIt(convertToMetrics(&parsed.Histograms, metric.Gauge, false), "histogram")
	appendIt(convertToMetrics(&parsed.Meters, metric.Gauge, cumulCounterEnabled), "meter")
	appendIt(convertToMetrics(&parsed.Timers, metric.Gauge, cumulCounterEnabled), "timer")

	// This is necessary as Go doesn't allow us to type assert
	// map[string]interface{} as map[string]string.
	// Basically go doesn't allow type assertions for interface{}'s nested
	// inside data structures across the entire structure since it is a linearly
	// complex action
	for k, v := range parsed.ServiceDims {
		metric.AddToAll(&results, map[string]string{k: v.(string)})
	}
	return results, nil
}

// parseJavaMetrics takes the json returned from the endpoint and converts
// #TODO
// it into raw metrics. We first check that the metrics returned have a float value
// otherwise we skip the metric.
//
// @cumulCounterEnabled: if true it enables to create meter and timer counters as
// Cumultative Counters instead of Gauges
func parseJavaMetrics(raw *[]byte, cumulCounterEnabled bool) ([]metric.Metric, error) {
	parsed := new(uwsgiJSONFormat1X)

	err := json.Unmarshal(*raw, parsed)
	if err != nil {
		return []metric.Metric{}, err
	}

	results := []metric.Metric{}
	appendIt := func(metrics []metric.Metric, typeDimVal string) {
		metric.AddToAll(&metrics, map[string]string{"type": typeDimVal})
		results = append(results, metrics...)
	}

	appendIt(convertToJavaMetrics(&parsed.Gauges, metric.Gauge, false), "gauge")
	appendIt(convertToJavaMetrics(&parsed.Counters, metric.Counter, false), "counter")
	appendIt(convertToJavaMetrics(&parsed.Histograms, metric.Gauge, false), "histogram")
	appendIt(convertToJavaMetrics(&parsed.Meters, metric.Gauge, cumulCounterEnabled), "meter")
	appendIt(convertToJavaMetrics(&parsed.Timers, metric.Gauge, cumulCounterEnabled), "timer")

	return results, nil
}

// parseDropwizardMetrics takes json string in following format::
// {
//     "jettyHandler": {
// 		"trace-requests": {
//			duration: {
//				p98: 45,
//				p75: 0
//			}
// 		 }
//      }
// }
// and returns list of metrices. The map can be arbitrarily nested.
func parseDropwizardMetrics(raw *[]byte) ([]metric.Metric, error) {
	var parsed map[string]interface{}

	err := json.Unmarshal(*raw, &parsed)

	if err != nil {
		return []metric.Metric{}, err
	}

	return parseNestedMetricMaps(parsed), nil
}

// parseNestedMetricMaps takes in arbitrarily nested map of following format::
//        "jetty": {
//            "trace-requests": {
//                "put-requests": {
//                    "duration": {
//                        "30x-response": {
//                            "count": 0,
//                            "type": "counter"
//                        }
//                    }
//                }
//            }
//        }
// and returns list of metrices by unrolling the map until it finds flattened map
// and then it combines keys encountered so far to emit metrices. For above sample
// data - emitted metrices will look like:
//	metric.Metric(
//		MetricName=jetty.trace-requests.put-request.duration.30x-response,
//		MetricType=COUNTER,
//		Value=0,
//		Dimenstions={rollup:count}
//		)
func parseNestedMetricMaps(
	jsonMap map[string]interface{}) []metric.Metric {

	results := []metric.Metric{}
	unvisitedMetricMaps := []nestedMetricMap{}

	startMetricMap := nestedMetricMap{
		metricSegments: []string{},
		metricMap:      jsonMap,
	}

	unvisitedMetricMaps = append(unvisitedMetricMaps, startMetricMap)

	for len(unvisitedMetricMaps) > 0 {
		nodeToVisit := unvisitedMetricMaps[0]
		unvisitedMetricMaps = unvisitedMetricMaps[1:]

		currentMetricSegment := nodeToVisit.metricSegments

	nodeVisitorLoop:
		for k, v := range nodeToVisit.metricMap {
			switch t := v.(type) {
			case map[string]interface{}:
				unvistedNode := nestedMetricMap{
					metricSegments: append(currentMetricSegment, k),
					metricMap:      t,
				}
				unvisitedMetricMaps = append(unvisitedMetricMaps, unvistedNode)
			default:
				tempResults := parseFlattenedMetricMap(nodeToVisit.metricMap,
					currentMetricSegment)
				if len(tempResults) > 0 {
					results = append(results, tempResults...)
					break nodeVisitorLoop
				}
				m, ok := extractGaugeValue(k, v, currentMetricSegment)
				if ok {
					results = append(results, m)
				}
			}
		}
	}

	return results
}

func parseFlattenedMetricMap(jsonMap map[string]interface{}, metricName []string) []metric.Metric {
	if t, ok := jsonMap["type"]; ok {
		metricType := t.(string)
		if metricType == "gauge" {
			return collectGauge(jsonMap, metricName, "gauge")
		} else if metricType == "histogram" {
			return collectHistogram(jsonMap, metricName, "histogram")
		} else if metricType == "counter" {
			return collectCounter(jsonMap, metricName, "counter")
		} else if metricType == "meter" {
			return collectMeter(jsonMap, metricName)
		}
	}

	// if nothing else works try for rate
	return collectRate(jsonMap, metricName)
}

// serviceInWhitelist returns true if the service name passed as argument
// is found among the ones whitelisted by the user
func (n *nerveUWSGICollector) serviceInWhitelist(service string) bool {
	for _, s := range n.servicesWhitelist {
		if s == service {
			return true
		}
	}
	return false
}

// convertToMetrics takes in data formatted like this::
// "pyramid_uwsgi_metrics.tweens.4xx-response": {
// 		"count":     366116,
//		"mean_rate": 0.2333071157843687,
//		"m15_rate":  0.22693345170298124,
//		"units":     "events/second"
// }
// and outputs a metric for each name:rollup pair where the value is a float/int
// automatiically it appends the dimensions::
//		- rollup: the value in the nested map (e.g. "count", "mean_rate")
//		- collector: this collector's name
func convertToMetrics(metricMap *map[string]map[string]interface{}, metricType string, cumulCounterEnabled bool) []metric.Metric {
	results := []metric.Metric{}

	for metricName, metricData := range *metricMap {
		tempResults := metricFromMap(metricData, metricName, metricType, cumulCounterEnabled)
		results = append(results, tempResults...)
	}

	return results
}

// convertToJavaMetrics takes in data formatted like this::
// #TODO
// "pyramid_uwsgi_metrics.tweens.4xx-response": {
// 		"count":     366116,
//		"mean_rate": 0.2333071157843687,
//		"m15_rate":  0.22693345170298124,
//		"units":     "events/second"
// }
// and outputs a metric for each name:rollup pair where the value is a float/int
// automatiically it appends the dimensions::
//		- rollup: the value in the nested map (e.g. "count", "mean_rate")
//		- collector: this collector's name
func convertToJavaMetrics(metricMap *map[string]map[string]interface{}, metricType string, cumulCounterEnabled bool) []metric.Metric {
	results := []metric.Metric{}

	for metricName, metricData := range *metricMap {
		for rollup, value := range metricData {
			mName := metricName
			mType := metricType
			matched, _ := regexp.MatchString("m[0-9]+_rate", rollup)

			// If cumulCounterEnabled is true:
			//		1. change metric type meter.count and timer.count moving them to cumulative counter
			//		2. don't send back metered metrics (rollup == 'mXX_rate')
			if cumulCounterEnabled && matched {
				continue
			}
			if cumulCounterEnabled && rollup == "count" {
				mName = metricName + ".count"
				mType = metric.CumulativeCounter
			}
			tmpMetric, ok := createMetricFromDatam(rollup, value, mName, mType)
			if ok {
				tmpMetric.AddDimensionsFromName()
				results = append(results, tmpMetric)
			}
		}
	}

	return results
}

// metricFromMap takes in flattened maps formatted like this::
// {
//    "count":      3443,
//    "mean_rate": 100
// }
// and metricname and metrictype and returns metrics for each name:rollup pair
func metricFromMap(metricMap map[string]interface{}, metricName string, metricType string, cumulCounterEnabled bool) []metric.Metric {
	results := []metric.Metric{}

	for rollup, value := range metricMap {
		mName := metricName
		mType := metricType
		matched, _ := regexp.MatchString("m[0-9]+_rate", rollup)

		// If cumulCounterEnabled is true:
		//		1. change metric type meter.count and timer.count moving them to cumulative counter
		//		2. don't send back metered metrics (rollup == 'mXX_rate')
		if cumulCounterEnabled && matched {
			continue
		}
		if cumulCounterEnabled && rollup == "count" {
			mName = metricName + ".count"
			mType = metric.CumulativeCounter
		}
		tempMetric, ok := createMetricFromDatam(rollup, value, mName, mType)
		if ok {
			results = append(results, tempMetric)
		}
	}

	return results
}

// createMetricFromDatam takes in rollup, value, metricName, metricType and returns metric only if
// value was numeric
func createMetricFromDatam(rollup string, value interface{}, metricName string, metricType string) (metric.Metric, bool) {
	m := metric.New(metricName)
	m.MetricType = metricType
	m.AddDimension("rollup", rollup)

	// only add things that have a numeric base
	switch value.(type) {
	case float64:
		m.Value = value.(float64)
	case int:
		m.Value = float64(value.(int))
	default:
		return m, false
	}
	return m, true
}

// extractGaugeValue emits metric for Map data which otherwise did not conform to
// any of predefined schemas as GAUGEs. For example::
//  "jvm": {
//    "garbage-collectors": {
//      "ConcurrentMarkSweep": {
//        "runs": 13,
//        "time": 1531
//      },
//      "ParNew": {
//        "runs": 45146,
//        "time": 1324093
//      }
//    },
//    "memory": {
//      "heap_usage": 0.24599579808405247,
//      "totalInit": 12887457792,
//      "memory_pool_usages": {
//        "Par Survivor Space": 0.11678684852358097,
//        "CMS Old Gen": 0.2679682979112999,
//        "Metaspace": 0.9466757034141924
//      },
//      "totalMax": 12727877631
//    },
//    "buffers": {
//      "direct": {
//        "count": 410,
//        "memoryUsed": 23328227,
//        "totalCapacity": 23328227
//      },
//      "mapped": {
//        "count": 1,
//        "memoryUsed": 18421396,
//        "totalCapacity": 18421396
//      }
//    }
//  }
func extractGaugeValue(key string, value interface{}, metricName []string) (metric.Metric, bool) {
	compositeMetricName := strings.Join(append(metricName, key), ".")
	return createMetricFromDatam("value", value, compositeMetricName, "GAUGE")
}

// collectGauge emits metric array for maps that contain guage values:
//    "percent-idle": {
//      "value": 0.985,
//      "type": "gauge"
//    }
func collectGauge(jsonMap map[string]interface{}, metricName []string,
	metricType string) []metric.Metric {

	if _, ok := jsonMap["value"]; ok {
		compositeMetricName := strings.Join(metricName, ".")
		return metricFromMap(jsonMap, compositeMetricName, metricType, false)
	}
	return []metric.Metric{}
}

// collectHistogram returns metrics list for maps that contain following data::
//    "prefix-length": {
//        "type": "histogram",
//        "count": 1,
//        "min": 2,
//        "max": 2,
//        "mean": 2,
//        "std_dev": 0,
//        "median": 2,
//        "p75": 2,
//        "p95": 2,
//        "p98": 2,
//        "p99": 2,
//        "p999": 2
//    }
func collectHistogram(jsonMap map[string]interface{},
	metricName []string, metricType string) []metric.Metric {

	results := []metric.Metric{}

	if _, ok := jsonMap["count"]; ok {
		for key, value := range jsonMap {
			if key == "type" {
				continue
			}

			metricType := MetricTypeGauge
			if key == "count" {
				metricType = MetricTypeCounter
			}

			compositeMetricName := strings.Join(metricName, ".")
			m, ok := createMetricFromDatam(key, value, compositeMetricName, metricType)
			if ok {
				results = append(results, m)
			}
		}
	}
	return results
}

// collectCounter returns metric list for data that looks like:
//    "active-suspended-requests": {
//       "count": 0,
//       "type": "counter"
//    }
func collectCounter(jsonMap map[string]interface{}, metricName []string,
	metricType string) []metric.Metric {

	if _, ok := jsonMap["count"]; ok {
		compositeMetricName := strings.Join(metricName, ".")
		return metricFromMap(jsonMap, compositeMetricName, metricType, false)
	}
	return []metric.Metric{}
}

// collectRate returns metric list for data that looks like:
//    "rate": {
//      "m15": 0,
//      "m5": 0,
//      "m1": 0,
//      "mean": 0,
//      "count": 0,
//      "unit": "seconds"
//    }
func collectRate(jsonMap map[string]interface{}, metricName []string) []metric.Metric {
	results := []metric.Metric{}
	if unit, ok := jsonMap["unit"]; ok && (unit == "seconds" || unit == "milliseconds") {
		for key, value := range jsonMap {
			if key == "unit" {
				continue
			}
			metricType := MetricTypeGauge

			if key == "count" {
				metricType = MetricTypeCounter
			}

			compositeMetricName := strings.Join(metricName, ".")
			m, ok := createMetricFromDatam(key, value, compositeMetricName, metricType)
			if ok {
				results = append(results, m)
			}
		}

	}
	return results
}

// collectMeter returns metric list for data that looks like:
//    "suspends": {
//      "m15": 0,
//      "m5": 0,
//      "m1": 0,
//      "mean": 0,
//      "count": 0,
//      "unit": "seconds",
//      "event_type": "requests",
//      "type": "meter"
//    }
func collectMeter(jsonMap map[string]interface{}, metricName []string) []metric.Metric {
	results := []metric.Metric{}

	if checkForMeterUnits(jsonMap) {
		for key, value := range jsonMap {
			if key == "unit" || key == "event_type" || key == "type" {
				continue
			}

			metricType := MetricTypeGauge
			if key == "count" {
				metricType = MetricTypeCounter
			}

			compositeMetricName := strings.Join(metricName, ".")
			m, ok := createMetricFromDatam(key, value, compositeMetricName, metricType)
			if ok {
				results = append(results, m)
			}
		}
	}

	return results
}

func checkForMeterUnits(jsonMap map[string]interface{}) bool {
	if _, ok := jsonMap["event_type"]; ok {
		if unit, ok := jsonMap["unit"]; ok &&
			(unit == "seconds" || unit == "milliseconds" || unit == "minutes") {
			return true
		}
	}
	return false
}
