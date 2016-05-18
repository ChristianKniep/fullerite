package collector

import (
	"fullerite/metric"
	"os/exec"
	"testing"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getSocketStatsCollector() *SocketStats {
	return newSocketStats(make(chan metric.Metric), 10, l.WithField("testing", "socket_stats")).(*SocketStats)
}

var socketStatsOut = `6 100 20 10
174 50 20 10
`

func TestDefaultConfigSocketStats(t *testing.T) {
	sscol := getSocketStatsCollector()
	sscol.Configure(make(map[string]interface{}))

	assert.Equal(t, 10, sscol.Interval())
}

func TestCustomConfigSocketStats(t *testing.T) {
	sscol := getSocketStatsCollector()
	configMap := map[string]interface{}{
		"PortList": []string{"9080"},
	}
	sscol.Configure(configMap)
	assert.Equal(t, 10, sscol.Interval())
	assert.Equal(t, []string{"9080"}, sscol.portList)
}

func TestCollectSocketStats(t *testing.T) {
	oldExecCommand := executeCommand
	oldCommandOutput := cmdOutput

	defer func() {
		executeCommand = oldExecCommand
		cmdOutput = oldCommandOutput
	}()

	executeCommand = func(string, ...string) *exec.Cmd {
		return &exec.Cmd{}
	}

	cmdOutput = func(*exec.Cmd) ([]byte, error) {
		return []byte(socketStatsOut), nil
	}

	expected := metric.Metric{Name: "ss.9080", MetricType: "gauge", Value: 50, Dimensions: map[string]string{}}

	c := make(chan metric.Metric)
	sscol := newSocketStats(c, 0, l.WithField("testing", "socket_stats")).(*SocketStats)
	cfg := map[string]interface{}{
		"PortList": []string{"9080"},
	}
	sscol.Configure(cfg)
	go sscol.Collect()

	actual := <-sscol.Channel()
	assert.Equal(t, expected, actual)
}
