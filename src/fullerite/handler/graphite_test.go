package handler

import (
	"fullerite/metric"

	"strings"
	"testing"
	"time"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getTestGraphiteHandler(interval, buffsize, timeoutsec int) *Graphite {
	testChannel := make(chan metric.Metric)
	testLog := l.WithField("testing", "graphite_handler")
	timeout := time.Duration(timeoutsec) * time.Second

	return newGraphite(testChannel, interval, buffsize, timeout, testLog).(*Graphite)
}

func TestGraphiteConfigureEmptyConfig(t *testing.T) {
	config := make(map[string]interface{})
	g := getTestGraphiteHandler(12, 13, 14)
	g.Configure(config)

	assert.Equal(t, 12, g.Interval())
}

func TestGraphiteConfigure(t *testing.T) {
	config := map[string]interface{}{
		"interval":        "10",
		"timeout":         "10",
		"max_buffer_size": "100",
		"server":          "test_server",
		"port":            "10101",
	}

	g := getTestGraphiteHandler(12, 13, 14)
	g.Configure(config)

	assert.Equal(t, 10, g.Interval())
	assert.Equal(t, 100, g.MaxBufferSize())
	assert.Equal(t, "test_server", g.Server())
	assert.Equal(t, "10101", g.Port())
}

func TestGraphiteConfigureIntPort(t *testing.T) {
	config := map[string]interface{}{
		"interval":        "10",
		"timeout":         "10",
		"max_buffer_size": "100",
		"server":          "test_server",
		"port":            10101,
	}

	g := getTestGraphiteHandler(12, 13, 14)
	g.Configure(config)

	assert.Equal(t, 10, g.Interval())
	assert.Equal(t, 100, g.MaxBufferSize())
	assert.Equal(t, "test_server", g.Server())
	assert.Equal(t, "10101", g.Port())
}

func TestGraphiteDimensionsOverwriting(t *testing.T) {
	s := getTestGraphiteHandler(12, 12, 12)

	m1 := metric.New("Test")
	m1.AddDimension("some=dim", "first value")
	m1.AddDimension("some-dim", "second value")
	datapoint := s.convertToGraphite(m1)

	assert.Equal(t, strings.Count(datapoint, "some-dim"), 1, "there should be only one dimension")
}

func TestGraphiteSanitation(t *testing.T) {
	s := getTestGraphiteHandler(12, 12, 12)

	m1 := metric.New(" Test= .me$tric ")
	m1.AddDimension("simple string", "simple string")
	m1.AddDimension("dot.string", "dot.string")
	m1.AddDimension("3.3", "3.3")
	m1.AddDimension("slash/string", "slash/string")
	m1.AddDimension("colon:string", "colon:string")
	m1.AddDimension("equal=string", "equal=string")
	datapoint1 := s.convertToGraphite(m1)

	datapoint2 := "Test-__metric.3_3.3_3.colon-string.colon-string.dot_string.dot_string.equal-string.equal-string.simple_string.simple_string.slash_string.slash_string"

	assert.Equal(t, strings.Split(datapoint1, " ")[0], datapoint2, "the two metrics should be the same")
}
