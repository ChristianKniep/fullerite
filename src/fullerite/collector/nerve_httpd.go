package collector

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"fullerite/config"
	"fullerite/metric"
	"fullerite/util"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	l "github.com/Sirupsen/logrus"
)

var (
	getNerveHTTPDMetrics = (*NerveHTTPD).getMetrics
	knownApacheMetrics   = map[string]bool{
		"ReqPerSec":        true,
		"BytesPerSec":      true,
		"BytesPerReq":      true,
		"BusyWorkers":      true,
		"Total Accesses":   true,
		"IdleWorkers":      true,
		"StartingWorkers":  true,
		"ReadingWorkers":   true,
		"WritingWorkers":   true,
		"KeepaliveWorkers": true,
		"DnsWorkers":       true,
		"ClosingWorkers":   true,
		"LoggingWorkers":   true,
		"FinishingWorkers": true,
		"CleanupWorkers":   true,
		"StandbyWorkers":   true,
		"CPULoad":          true,
	}
	metricRegexp = regexp.MustCompile(`^([A-Za-z ]+):\s+(.+)$`)
)

// NerveHTTPD discovers Apache servers via Nerve config
// and reports metric for them
type NerveHTTPD struct {
	baseCollector

	configFilePath    string
	queryPath         string
	host              string
	timeout           int
	statusTTL         time.Duration
	failedEndPoints   map[string]int64
	mu                *sync.RWMutex
	servicesWhitelist []string
}

type nerveHTTPDResponse struct {
	data   []byte
	err    error
	status int
}

func init() {
	RegisterCollector("NerveHTTPD", newNerveHTTPD)
}

func newNerveHTTPD(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	c := new(NerveHTTPD)
	c.channel = channel
	c.interval = initialInterval
	c.log = log
	c.mu = new(sync.RWMutex)

	c.name = "NerveHTTPD"
	c.configFilePath = "/etc/nerve/nerve.conf.json"
	c.queryPath = "server-status?auto"
	c.host = "localhost"
	c.timeout = 2
	c.statusTTL = time.Duration(60) * time.Minute
	c.failedEndPoints = map[string]int64{}
	return c
}

// Configure the collector
func (c *NerveHTTPD) Configure(configMap map[string]interface{}) {
	if val, exists := configMap["queryPath"]; exists {
		c.queryPath = val.(string)
	}

	if val, exists := configMap["configFilePath"]; exists {
		c.configFilePath = val.(string)
	}

	if val, exists := configMap["host"]; exists {
		c.host = val.(string)
	}

	if val, exists := configMap["status_ttl"]; exists {
		tmpStatusTTL := config.GetAsInt(val, 3600)
		c.statusTTL = time.Duration(tmpStatusTTL) * time.Second
	}

	if val, exists := configMap["servicesWhitelist"]; exists {
		c.servicesWhitelist = config.GetAsSlice(val)
	}

	c.configureCommonParams(configMap)
}

// Collect the metrics
func (c *NerveHTTPD) Collect() {
	rawFileContents, err := ioutil.ReadFile(c.configFilePath)
	if err != nil {
		c.log.Warn("Failed to read the contents of file ", c.configFilePath, " because ", err)
		return
	}
	servicePortMap, err := util.ParseNerveConfig(&rawFileContents)
	if err != nil {
		c.log.Warn("Failed to parse the nerve config at ", c.configFilePath, ": ", err)
		return
	}
	c.log.Debug("Finished parsing Nerve config into ", servicePortMap)

	for port, serviceName := range servicePortMap {
		if c.servicesWhitelist == nil || c.serviceInWhitelist(serviceName) {
			if !c.checkIfFailed(serviceName, port) {
				go c.emitHTTPDMetric(serviceName, port)
			}
		}
	}
}

func (c *NerveHTTPD) serviceInWhitelist(service string) bool {
	for _, s := range c.servicesWhitelist {
		if s == service {
			return true
		}
	}
	return false
}

func (c *NerveHTTPD) emitHTTPDMetric(serviceName string, port int) {
	metrics := getNerveHTTPDMetrics(c, serviceName, port)
	for _, metric := range metrics {
		c.Channel() <- metric
	}
}

func (c *NerveHTTPD) checkIfFailed(serviceName string, port int) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	endpoint := fmt.Sprintf("%s:%d", serviceName, port)
	if lastFailed, ok := c.failedEndPoints[endpoint]; ok {
		tm := time.Unix(lastFailed, 0)
		if time.Since(tm) < c.statusTTL {
			return true
		}
	}
	return false
}

func (c *NerveHTTPD) getMetrics(serviceName string, port int) []metric.Metric {
	results := []metric.Metric{}
	serviceLog := c.log.WithField("service", serviceName)

	endpoint := fmt.Sprintf("http://%s:%d/%s", c.host, port, c.queryPath)
	serviceLog.Debug("making GET request to ", endpoint)

	httpResponse := fetchApacheMetrics(endpoint, port)

	if httpResponse.status != 200 {
		c.updateFailedStatus(serviceName, port, httpResponse.status)
		serviceLog.Warn("Failed to query endpoint ", endpoint, ": ", httpResponse.err)
		return results
	}
	apacheMetrics := extractApacheMetrics(httpResponse.data)
	metric.AddToAll(&apacheMetrics, map[string]string{
		"service": serviceName,
		"port":    strconv.Itoa(port),
	})
	return apacheMetrics
}

func extractApacheMetrics(data []byte) []metric.Metric {
	results := []metric.Metric{}
	reader := bytes.NewReader(data)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		metricLine := scanner.Text()
		resultMatch := metricRegexp.FindStringSubmatch(metricLine)
		if len(resultMatch) > 0 {
			k := resultMatch[1]
			v := resultMatch[2]
			if k == "IdleWorkers" {
				continue
			}

			if k == "Scoreboard" {
				scoreBoardMetrics := extractScoreBoardMetrics(k, v)
				results = append(results, scoreBoardMetrics...)
			}

			metric, err := buildApacheMetric(k, v)
			if err == nil {
				results = append(results, metric)
			}
		}

	}
	return results
}

func buildApacheMetric(key, value string) (metric.Metric, error) {
	var tmpMetric metric.Metric
	if _, ok := knownApacheMetrics[key]; ok {
		whiteRegexp := regexp.MustCompile(`\s+`)
		metricName := whiteRegexp.ReplaceAllString(key, "")
		metricValue, err := strconv.ParseFloat(value, 64)

		if err != nil {
			return tmpMetric, err
		}
		return metric.WithValue(metricName, metricValue), nil
	}
	return tmpMetric, errors.New("invalid metric")
}

func extractScoreBoardMetrics(key, value string) []metric.Metric {
	results := []metric.Metric{}
	charCounter := func(str string, pattern string) float64 {
		return float64(strings.Count(str, pattern))
	}
	results = append(results, metric.WithValue("IdleWorkers", charCounter(value, "_")))
	results = append(results, metric.WithValue("StartingWorkers", charCounter(value, "S")))
	results = append(results, metric.WithValue("ReadingWorkers", charCounter(value, "R")))
	results = append(results, metric.WithValue("WritingWorkers", charCounter(value, "W")))
	results = append(results, metric.WithValue("KeepaliveWorkers", charCounter(value, "K")))
	results = append(results, metric.WithValue("DnsWorkers", charCounter(value, "D")))
	results = append(results, metric.WithValue("ClosingWorkers", charCounter(value, "C")))
	results = append(results, metric.WithValue("LoggingWorkers", charCounter(value, "L")))
	results = append(results, metric.WithValue("FinishingWorkers", charCounter(value, "G")))
	results = append(results, metric.WithValue("CleanupWorkers", charCounter(value, "I")))
	results = append(results, metric.WithValue("StandbyWorkers", charCounter(value, "_")))
	return results
}

func (c *NerveHTTPD) updateFailedStatus(serviceName string, port int, statusCode int) {
	if statusCode == 404 {
		c.mu.Lock()
		defer c.mu.Unlock()
		endpoint := fmt.Sprintf("%s:%d", serviceName, port)
		c.failedEndPoints[endpoint] = time.Now().Unix()
	}
}

func fetchApacheMetrics(endpoint string, timeout int) *nerveHTTPDResponse {
	response := new(nerveHTTPDResponse)
	client := http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	rsp, err := client.Get(endpoint)
	response.err = err
	if rsp != nil {
		response.status = rsp.StatusCode
	}

	if err != nil {
		return response
	}

	txt, err := ioutil.ReadAll(rsp.Body)
	defer rsp.Body.Close()
	if err != nil {
		response.err = err
		return response
	}
	response.data = txt
	return response
}
