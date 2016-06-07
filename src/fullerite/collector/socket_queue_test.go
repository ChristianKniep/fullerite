package collector

import (
	"fullerite/metric"
	"os/exec"
	"testing"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getSocketQueueCollector() *SocketQueue {
	return newSocketQueue(make(chan metric.Metric), 10, l.WithField("testing", "socket_queue")).(*SocketQueue)
}

var socketStatsOut = `State      Recv-Q Send-Q        Local Address:Port          Peer Address:Port
LISTEN      10      128     *:9080    *:*
LISTEN      0      128     *:1224    *:*
LISTEN      0      128     *:1234    *:*`

func TestDefaultConfigSocketQueue(t *testing.T) {
	sscol := getSocketQueueCollector()
	sscol.Configure(make(map[string]interface{}))

	assert.Equal(t, 10, sscol.Interval())
}

func TestCustomConfigSocketQueue(t *testing.T) {
	sscol := getSocketQueueCollector()
	configMap := map[string]interface{}{
		"PortList": []string{"9080"},
	}
	sscol.Configure(configMap)
	assert.Equal(t, 10, sscol.Interval())
	assert.Equal(t, []string{"9080"}, sscol.portList)
}

func TestCollectSocketQueue(t *testing.T) {
	oldCommandOutput := cmdOutput

	defer func() {
		cmdOutput = oldCommandOutput
	}()

	cmdOutput = func(*exec.Cmd) ([]byte, error) {
		return []byte(socketStatsOut), nil
	}

	expected := []metric.Metric{
		metric.Metric{Name: "sq.listen", MetricType: "gauge", Value: 10, Dimensions: map[string]string{"port": "9080"}},
		metric.Metric{Name: "sq.listen", MetricType: "gauge", Value: 0, Dimensions: map[string]string{"port": "1234"}},
		metric.Metric{Name: "sq.listen", MetricType: "gauge", Value: 0, Dimensions: map[string]string{"port": "1224"}},
	}

	c := make(chan metric.Metric)
	sscol := newSocketQueue(c, 0, l.WithField("testing", "socket_queue")).(*SocketQueue)
	cfg := map[string]interface{}{
		"PortList": []string{"9080", "1234", "1224"},
	}
	sscol.Configure(cfg)
	go sscol.Collect()

	for range expected {
		actual := <-sscol.Channel()
		assert.Contains(t, expected, actual)
	}
}
