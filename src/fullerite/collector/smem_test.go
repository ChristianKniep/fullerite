package collector

import (
	"fullerite/metric"
	"os/exec"
	"testing"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var smemOutput = `   5   864  2442180 apache2
`

func TestNewSmemStats(t *testing.T) {
	c := make(chan metric.Metric)
	i := 10
	l := defaultLog.WithFields(l.Fields{"collector": "Mesos"})

	actual := newSmemStats(c, i, l).(*SmemStats)

	assert.Equal(t, "SmemStats", actual.Name())
	assert.Equal(t, c, actual.Channel())
	assert.Equal(t, i, actual.Interval())
	assert.Equal(t, l, actual.log)
}

func TestSmemStatsConfigure(t *testing.T) {
	tests := []struct {
		config            map[string]interface{}
		expectedWhitelist string
		expectedUser      string
		msg               string
	}{
		{
			config: map[string]interface{}{
				"user":           "fullerite",
				"procsWhitelist": "apache2|tmux",
			},
			expectedWhitelist: "apache2|tmux",
			expectedUser:      "fullerite",
			msg:               "All configs are valid, so no errors",
		},
		{
			config:            map[string]interface{}{},
			expectedWhitelist: "",
			expectedUser:      "",
			msg:               "Required configs missing",
		},
	}

	l := defaultLog.WithFields(l.Fields{"collector": "SmemStats"})

	for _, test := range tests {
		sut := newSmemStats(nil, 0, l).(*SmemStats)
		sut.Configure(test.config)
		assert.Equal(t, test.expectedWhitelist, sut.whitelistedProcs, test.msg)
	}
}

func TestSmemStatsCollect(t *testing.T) {
	oldExecCommand := execCommand
	oldCommandOutput := commandOutput

	defer func() {
		execCommand = oldExecCommand
		commandOutput = oldCommandOutput
	}()

	execCommand = func(string, ...string) *exec.Cmd {
		return &exec.Cmd{}
	}

	commandOutput = func(*exec.Cmd) ([]byte, error) {
		return []byte(smemOutput), nil
	}

	actual := []metric.Metric{}
	expected := []metric.Metric{
		metric.Metric{Name: "apache2.smem.pss", MetricType: "gauge", Value: 5, Dimensions: map[string]string{}},
		metric.Metric{Name: "apache2.smem.vss", MetricType: "gauge", Value: 2.44218e+06, Dimensions: map[string]string{}},
		metric.Metric{Name: "apache2.smem.rss", MetricType: "gauge", Value: 864, Dimensions: map[string]string{}},
	}

	c := make(chan metric.Metric)
	sut := newSmemStats(c, 0, defaultLog).(*SmemStats)
	sut.user = "user"
	sut.whitelistedProcs = "some|whitelist"
	go sut.Collect()

	for i := 0; i < len(expected); i++ {
		actual = append(actual, <-c)
	}

	assert.Equal(t, expected, actual)
}

func TestSmemStatsInvalidRegex(t *testing.T) {
	oldGetSmemStats := getSmemStats
	defer func() { getSmemStats = oldGetSmemStats }()

	getSmemStatsCalled := false
	getSmemStats = func(*SmemStats) []smemStatLine {
		getSmemStatsCalled = true
		return nil
	}

	sut := newSmemStats(nil, 0, nil).(*SmemStats)
	sut.Collect()

	assert.False(t, getSmemStatsCalled)
}
