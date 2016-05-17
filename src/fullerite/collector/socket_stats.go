package collector

import (
	"encoding/json"
	"fullerite/metric"
	"io/ioutil"
	"os/exec"
	"strings"

	l "github.com/Sirupsen/logrus"
)

const (
	cName = "SocketStats"
	mName = "socket_stats."
)

// SocketStats reports output of "ss" command and reports
// the socket RecvQ value as a metric.
type SocketStats struct {
	baseCollector
	configFilePath string
}

type configData struct {
	PortList []string
}

func init() {
	RegisterCollector("SocketStats", newSocketStats)
}

func newSocketStats(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	ss := new(SocketStats)
	ss.channel = channel
	ss.interval = initialInterval
	ss.log = log

	ss.name = cName
	ss.configFilePath = "/etc/socket_stats/socket_stats.conf.json"
	return ss
}

// Configure Override default parameters
func (ss *SocketStats) Configure(configMap map[string]interface{}) {
	if val, exists := configMap["configFilePath"]; exists {
		ss.configFilePath = val.(string)
	}
	ss.configureCommonParams(configMap)
}

// Collect the receive queue size (RecvQ)
func (ss SocketStats) Collect() {

	configFileContent, err := ioutil.ReadFile(ss.configFilePath)
	if err != nil {
		ss.log.Warn("Failed to read config file ", ss.configFilePath, " error = ", err)
		return
	}

	data := new(configData)
	err = json.Unmarshal(configFileContent, data)
	if err != nil {
		ss.log.Warn("Failed to parse the configuration at ", ss.configFilePath, ": ", err)
		return
	}
	for i := 0; i < len(data.PortList); i++ {
		sport := data.PortList[i]
		value, err := getSocketStats(sport)
		if err != nil {
			ss.log.Error("Error while collecting metrics: ", err, " for port ", sport)
			return
		}
		metric := metric.New(mName + sport)
		metric.Value = value
		ss.log.Debug(metric)
	}
}

func getSocketStats(sport string) (float64, error) {
	// Run the command 'ss -ntl sport = : <port_num>' to obtain the recvQ value
	args := "-ntl sport = :" + sport
	output, err := exec.Command("ss", args).CombinedOutput()
	if err != nil {
		return 0.0, err
	}
	val := getValueFromOutput(output)

	return val, err
}

func getValueFromOutput(output []byte) float64 {
	lines := strings.SplitN(string(output), "\n", 2)
	if len(lines) < 2 {
		return 0.0
	}
	// strVal := strings.Fields(lines[1])[1]
	// return strconv.ParseFloat(strVal, 64)
	return 0.0
}
