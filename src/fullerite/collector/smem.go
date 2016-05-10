// Requirements: smem (https://www.selenic.com/smem/).
// Permissions: The user running fullerite should be able to access /proc/<pid>/smaps files for the process being monitored
//
// Config file: SmemStats.conf
// Example: {
//   "procsWhitelist": "apache2|tmux"
// }

package collector

import (
	"fullerite/config"
	"fullerite/metric"
	"os/exec"
	"strconv"
	"strings"

	l "github.com/Sirupsen/logrus"
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
	user             string
	whitelistedProcs string
}

var (
	requiredConfigs = []string{"user", "procsWhitelist"}

	execCommand   = exec.Command
	commandOutput = (*exec.Cmd).Output
	getSmemStats  = (*SmemStats).getSmemStats
)

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
	cfg := config.GetAsMap(configMap)

	if user, exists := cfg["user"]; exists {
		s.user = user
	} else {
		s.log.Warn("Required config does not exist for SmemStats: user")
	}

	if whitelist, exists := cfg["procsWhitelist"]; exists {
		s.whitelistedProcs = whitelist
	} else {
		s.log.Warn("Required config does not exist for SmemStats: procsWhitelist")
	}
}

// Collect periodically call smem periodically
func (s *SmemStats) Collect() {
	if s.whitelistedProcs == "" || s.user == "" {
		return
	}

	for _, stat := range getSmemStats(s) {
		s.Channel() <- metric.WithValue(stat.proc+".smem.pss", stat.pss)
		s.Channel() <- metric.WithValue(stat.proc+".smem.vss", stat.vss)
		s.Channel() <- metric.WithValue(stat.proc+".smem.rss", stat.rss)
	}
}

func (s *SmemStats) getSmemStats() []smemStatLine {
	var out []byte
	var err error

	cmd := execCommand(
		"/usr/bin/sudo",
		"-u", s.user,
		"/usr/bin/smem",
		"-P", s.whitelistedProcs,
		"-c", "pss rss vss name")
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
		stats = append(stats, smemStatLine{
			proc: parts[3],
			pss:  strToFloat(parts[0]),
			rss:  strToFloat(parts[1]),
			vss:  strToFloat(parts[2]),
		})
	}

	return stats
}

func strToFloat(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}

	return 0
}
