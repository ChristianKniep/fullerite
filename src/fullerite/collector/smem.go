// Requirements: smem (https://www.selenic.com/smem/).
//
// Config file: SmemStats.conf
// Example: {
//   "user": "some-user", <-- This user should be able to access the /proc/<pid>/smaps files listed in the whitelist below
//   "procsWhitelist": "apache2|tmux",
//   "smemPath": "/usr/bin/smem" <-- Path to the smem executable
// }

package collector

import (
	"fullerite/config"
	"fullerite/metric"
	"fullerite/util"
	"os/exec"
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
	user               string
	whitelistedProcs   string
	smemPath           string
	whitelistedMetrics []string
}

var (
	requiredConfigs = []string{"user", "procsWhitelist"}

	execCommand   = exec.Command
	commandOutput = (*exec.Cmd).Output
	getSmemStats  = (*SmemStats).getSmemStats
	allMetrics    = []string{"rss", "vss", "pss"}
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

	if user, exists := configMap["user"]; exists {
		s.user = user.(string)
	} else {
		s.log.Warn("Required config does not exist for SmemStats: user")
	}

	if whitelist, exists := configMap["procsWhitelist"]; exists {
		s.whitelistedProcs = whitelist.(string)
	} else {
		s.log.Warn("Required config does not exist for SmemStats: procsWhitelist")
	}

	if smemPath, exists := configMap["smemPath"]; exists {
		s.smemPath = smemPath.(string)
	} else {
		s.log.Warn("Required config does not exist for SmemStats: smemPath")
	}

	if blacklist, exists := configMap["metricsBlacklist"]; exists {
		s.whitelistedMetrics = getWhitelistedMetrics(config.GetAsSlice(blacklist))
	} else {
		s.whitelistedMetrics = allMetrics
	}
}

// Collect periodically call smem periodically
func (s *SmemStats) Collect() {
	if s.whitelistedProcs == "" || s.user == "" || s.smemPath == "" {
		return
	}

	for _, stat := range getSmemStats(s) {
		for _, element := range s.whitelistedMetrics {
			switch element {
			case "pss":
				s.Channel() <- metric.WithValue(stat.proc+".smem.pss", stat.pss)
			case "vss":
				s.Channel() <- metric.WithValue(stat.proc+".smem.vss", stat.vss)
			case "rss":
				s.Channel() <- metric.WithValue(stat.proc+".smem.rss", stat.rss)
			}
		}
	}
}

func (s *SmemStats) getSmemStats() []smemStatLine {
	var out []byte
	var err error

	cmd := execCommand(
		"/usr/bin/sudo",
		"-u", s.user,
		s.smemPath,
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
			pss:  util.StrToFloat(parts[0]),
			rss:  util.StrToFloat(parts[1]),
			vss:  util.StrToFloat(parts[2]),
		})
	}

	return stats
}

func getWhitelistedMetrics(blacklist []string) []string {
	var diff []string
	for _, s1 := range allMetrics {
		found := false
		for _, s2 := range blacklist {
			if s1 == s2 {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, s1)
		}
	}
	return diff
}
