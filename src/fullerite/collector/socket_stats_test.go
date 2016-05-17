package collector

import (
	"fullerite/metric"
	"io/ioutil"
	"encoding/json"
	"os"
	"testing"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getSocketStatsCollector() *SocketStats {
	return newSocketStats(make(chan metric.Metric), 10, l.WithField("testing", "socket_stats")).(*SocketStats)
}

func getRawSocketStats(name string) []byte {
	metric := []byte(`
socket_stats.9080: 50
socket_stats.1234: 174
	`)
	return metric
}

func setupTestConfig(name string) {
	testConfig := make(map[string]interface{}, 2)
	testConfig["PortList"] = []interface{} {9080, 1234}

	testFile, _ := ioutil.TempFile("", name)
	defer os.Remove(testFile.Name())

	marshalled, _ := json.Marshal(testConfig)
	testFile.Write(marshalled)
}

func TestDefaultConfigSocketStats(t *testing.T) {
	sscol := getSocketStatsCollector()
	sscol.Configure(make(map[string]interface{}))

	assert.Equal(t, 10, sscol.Interval())
	assert.Equal(t, "/etc/socket_stats/socket_stats.conf.json", sscol.configFilePath)
}


func TestCustomConfigSocketStats(t *testing.T) {
	sscol := getSocketStatsCollector()
	configMap := map[string]interface{}{
		"configFilePath": "/tmp/test.json",
	}
	sscol.Configure(configMap)
	assert.Equal(t, 10, sscol.Interval())
	assert.Equal(t, "/tmp/test.json", sscol.configFilePath)
}

func TestCollectSocketStats(t *testing.T) {
	setupTestConfig("socket_stats_testing")
	sscol := getSocketStatsCollector()
	cfg := map[string]interface{}{
		"configFilePath": "socket_stats_testing",
	}
	sscol.Configure(cfg)
	sscol.Collect()
}
