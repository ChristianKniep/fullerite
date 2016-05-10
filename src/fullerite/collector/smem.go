package collector

import (
	"fullerite/metric"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	l "github.com/Sirupsen/logrus"
)

var (
	execCommand   = exec.Command
	commandOutput = (*exec.Cmd).Output
)

type smemStatLine struct {
	proc string
	pss  float64
	rss  float64
	vss  float64
}

// SmemStats Collector to record smem stats
type SmemStats struct {
	baseCollector
	whitelistedProcs *regexp.Regexp
}

func init() {
	RegisterCollector("SmemStats", newSmemStats)
}

func newSmemStats(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	s := new(SmemStats)
	s.log = log
	s.channel = channel
	s.interval = initialInterval
	s.name = "SmemStats"

	return s
}

// Configure Override *baseCollector.Configure(); will fetch the whitelisted processes
func (s *SmemStats) Configure(configMap map[string]interface{}) {
	s.configureCommonParams(configMap)

	if pattern, exists := configMap["procsWhitelist"]; exists {
		if re, err := regexp.Compile(pattern.(string)); err == nil {
			s.whitelistedProcs = re
		} else {
			s.log.Warn("Failed to compile the procsWhitelist regex:", err)
		}

	}
}

// Collect periodically call smem periodically
func (s *SmemStats) Collect() {
	for _, stat := range s.getSmemStats() {
		s.publishMetric(stat.proc, "pss", stat.pss)
		s.publishMetric(stat.proc, "vss", stat.vss)
		s.publishMetric(stat.proc, "rss", stat.rss)
	}

}

func (s *SmemStats) getSmemStats() []smemStatLine {
	var out []byte
	var err error

	cmd := execCommand("/bin/ps", "ax", "-o", "rss=,vsz=,comm=")
	if out, err = commandOutput(cmd); err != nil {
		s.log.Error(err.Error())
		return nil
	}

	return s.parseSmemLines(string(out))
}

func (s *SmemStats) parseSmemLines(out string) []smemStatLine {
	raw := strings.Trim(out, "\n")
	lines := strings.Split(raw, "\n")
	stats := []smemStatLine{}

	for _, line := range lines {
		parts := strings.Fields(line)
		if s.whitelistedProcs.Match([]byte(parts[2])) {
			stats = append(stats, smemStatLine{
				proc: parts[2],
				pss:  0,
				rss:  strToFloat(parts[0]),
				vss:  strToFloat(parts[1]),
			})
		}
	}

	return stats
}

func (s *SmemStats) publishMetric(proc string, metricType string, val float64) {
	m := metric.New(proc + ".smem." + metricType)
	m.Value = val

	s.Channel() <- m
}

func strToFloat(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}

	return 0
}
