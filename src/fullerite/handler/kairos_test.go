package handler

import (
	"fullerite/metric"

	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	l "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func getTestKairosHandler(interval, buffsize, timeoutsec int) *Kairos {
	testChannel := make(chan metric.Metric)
	testLog := l.WithField("testing", "kairos_handler")
	timeout := time.Duration(timeoutsec) * time.Second

	return newKairos(testChannel, interval, buffsize, timeout, testLog).(*Kairos)
}

func TestKairosConfigureEmptyConfig(t *testing.T) {
	config := make(map[string]interface{})
	k := getTestKairosHandler(12, 13, 14)
	k.Configure(config)

	assert.Equal(t, 12, k.Interval())
}

func TestKairosConfigure(t *testing.T) {
	config := map[string]interface{}{
		"interval":        "10",
		"timeout":         "10",
		"max_buffer_size": "100",
		"server":          "kairos.server",
		"port":            "10101",
	}

	k := getTestKairosHandler(12, 13, 14)
	k.Configure(config)

	assert.Equal(t, 10, k.Interval())
	assert.Equal(t, 100, k.MaxBufferSize())
	assert.Equal(t, "kairos.server", k.Server())
	assert.Equal(t, "10101", k.Port())
}

func TestKairosConfigureIntPort(t *testing.T) {
	config := map[string]interface{}{
		"interval":        "10",
		"timeout":         "10",
		"max_buffer_size": "100",
		"server":          "kairos.server",
		"port":            10101,
	}

	k := getTestKairosHandler(12, 13, 14)
	k.Configure(config)

	assert.Equal(t, 10, k.Interval())
	assert.Equal(t, 100, k.MaxBufferSize())
	assert.Equal(t, "kairos.server", k.Server())
	assert.Equal(t, "10101", k.Port())
}

func TestKairosRun(t *testing.T) {
	wait := make(chan bool)
	// Mock Kairos server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		assert.Nil(t, err)

		kairosMetrics := make([]KairosMetric, 1)
		err = json.Unmarshal(body, &kairosMetrics)
		assert.Nil(t, err)

		assert.Equal(t, kairosMetrics[0].Name, "Test")
		assert.Equal(t, r.Header["Content-Type"], []string{"application/json"})

		wait <- true
	}))
	defer ts.Close()

	url, _ := url.Parse(ts.URL)
	urlParts := strings.Split(url.Host, ":")

	config := map[string]interface{}{
		"interval":        "1",
		"timeout":         "1",
		"max_buffer_size": "1",
		"server":          urlParts[0],
		"port":            urlParts[1],
	}

	k := getTestKairosHandler(12, 13, 14)
	k.Configure(config)

	go k.Run()

	m := metric.New("Test")
	k.Channel() <- m

	select {
	case <-wait:
		// noop
	case <-time.After(2 * time.Second):
		t.Fatal("Failed to post and handle after 2 seconds")
	}
}

func TestKairosServerErrorParse(t *testing.T) {
	k := getTestKairosHandler(12, 13, 14)

	metrics := make([]metric.Metric, 0, 3)

	metrics = append(metrics, metric.New("Test1"))
	metrics = append(metrics, metric.New("Test2"))
	metrics = append(metrics, metric.New("test3"))

	metrics[0].AddDimension("somedim", "")
	metrics[1].AddDimension("somedim", "working")
	metrics[2].AddDimension("somedime", "")

	series := make([]KairosMetric, 0, len(metrics))
	for i := range metrics {
		series = append(series, k.convertToKairos(metrics[i]))
	}

	expectMetrics := make([]KairosMetric, 0, 2)
	expectMetrics = append(expectMetrics, k.convertToKairos(metrics[0]))
	expectMetrics = append(expectMetrics, k.convertToKairos(metrics[2]))

	expectByt, _ := json.Marshal(expectMetrics)

	errByt := []byte(`{\"errors\":[\"metric[0](name=Test1).tag[somedim].value may not be empty.\",` +
		`\"metric[2](name=Test3).tag[somedim].value may not be empty.\"]}`)

	assert.Equal(t, k.parseServerError(string(errByt), series), string(expectByt))
}

func TestSanitizeMetricName(t *testing.T) {
	k := getTestKairosHandler(12, 13, 14)

	m1 := metric.New("Test==:")
	m1.AddDimension("somedim", "value")
	s1 := k.convertToKairos(m1)

	m2 := metric.New("Test---")
	m2.AddDimension("somedim", "value")
	s2 := k.convertToKairos(m2)

	assert.Equal(t, s1, s2, "metric name should be sanitazed")
}

func TestSanitizeMetricDimensionName(t *testing.T) {
	k := getTestKairosHandler(12, 13, 14)

	m1 := metric.New("Test")
	m1.AddDimension("some=dim", "valu=")
	s1 := k.convertToKairos(m1)

	m2 := metric.New("Test")
	m2.AddDimension("some-dim", "valu-")
	s2 := k.convertToKairos(m2)

	assert.Equal(t, s1, s2, "metric dimension should be sanitazed")
}

func TestSanitizeMetricDimensionValue(t *testing.T) {
	k := getTestKairosHandler(12, 13, 14)

	m1 := metric.New("Test")
	m1.AddDimension("some=dim", "valu=")
	s1 := k.convertToKairos(m1)

	m2 := metric.New("Test")
	m2.AddDimension("some-dim", "valu-")
	s2 := k.convertToKairos(m2)

	assert.Equal(t, s1, s2, "metric dimension should be sanitazed")
}

func TestSanitationMetrics(t *testing.T) {
	s := getTestKairosHandler(12, 13, 14)

	m1 := metric.New(" Test= .me$tric ")
	m1.AddDimension("simple string", "simple string")
	m1.AddDimension("dot.string", "dot.string")
	m1.AddDimension("3.3", "3.3")
	m1.AddDimension("slash/string", "slash/string")
	m1.AddDimension("colon:string", "colon:string")
	m1.AddDimension("equal=string", "equal=string")
	datapoint1 := s.convertToKairos(m1)

	m2 := metric.New("Test-_.metric")
	m2.AddDimension("simple_string", "simple_string")
	m2.AddDimension("dot.string", "dot.string")
	m2.AddDimension("3.3", "3.3")
	m2.AddDimension("slash/string", "slash/string")
	m2.AddDimension("colon-string", "colon-string")
	m2.AddDimension("equal-string", "equal-string")
	datapoint2 := s.convertToKairos(m2)

	assert.Equal(t, datapoint1, datapoint2, "the two metrics should be the same")
}

func TestKairosDimensionsOverwriting(t *testing.T) {
	s := getTestKairosHandler(12, 12, 12)

	m1 := metric.New("Test")
	m1.AddDimension("some=dim", "first value")
	m1.AddDimension("some-dim", "second value")
	datapoint := s.convertToKairos(m1)

	assert.Equal(t, len(datapoint.Tags), 1, "the two metrics should be the same")
}
