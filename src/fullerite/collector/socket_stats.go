package collector

import (
	"fullerite/config"
	"fullerite/metric"
	"os/exec"
	"strconv"
	"strings"

	l "github.com/Sirupsen/logrus"
)

// SocketStats reports output of "ss" command and reports
// the socket RecvQ value as a metric.
type SocketStats struct {
	baseCollector
	portList []string
}

var (
	executeCommand = exec.Command
	cmdOutput      = (*exec.Cmd).CombinedOutput
)

func init() {
	RegisterCollector("SocketStats", newSocketStats)
}

func newSocketStats(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	ss := new(SocketStats)
	ss.channel = channel
	ss.interval = initialInterval
	ss.log = log
	ss.name = "SocketStats"

	return ss
}

// Configure Override default parameters
func (ss *SocketStats) Configure(configMap map[string]interface{}) {
	if asInterface, exists := configMap["PortList"]; exists {
		ss.portList = config.GetAsSlice(asInterface)
	}
}

// Collect the receive queue size (RecvQ)
func (ss SocketStats) Collect() {

	for i := 0; i < len(ss.portList); i++ {
		sport := ss.portList[i]
		value, err := getSocketStats(sport)
		if err != nil {
			ss.log.Error("Error while collecting metrics: ", err, " for port ", sport)
			return
		}
		metric := metric.New("ss." + sport)
		metric.Value = value
		ss.log.Debug(metric)
		ss.Channel() <- metric
	}
}

func getSocketStats(sport string) (float64, error) {
	// Run the command 'ss -ntl sport = : <port_num>' to obtain the recvQ value
	args := "-ntl sport = :" + sport
	cmd := execCommand("ss", args)
	output, err := cmdOutput(cmd)
	if err != nil {
		return 0.0, err
	}
	val := getValueFromOutput(output)

	return val, err
}

func getValueFromOutput(output []byte) float64 {
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0.0
	}
	parts := strings.Fields(lines[1]) // Second line of the output
	return strToFlt(parts[1])         // RecvQ - second column
}

func strToFlt(val string) float64 {
	if i, err := strconv.ParseFloat(val, 64); err == nil {
		return i
	}

	return 0
}
