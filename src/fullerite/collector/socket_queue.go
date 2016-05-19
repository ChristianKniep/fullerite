package collector

import (
	"fullerite/config"
	"fullerite/metric"
	"fullerite/util"
	"os/exec"
	"regexp"
	"strings"

	l "github.com/Sirupsen/logrus"
)

// SocketQueue reports output of "ss" command and reports
// the socket RecvQ value as a metric.
type SocketQueue struct {
	baseCollector
	portList []string
}

var (
	cmdOutput      = (*exec.Cmd).CombinedOutput
)

func init() {
	RegisterCollector("SocketQueue", newSocketQueue)
}

func newSocketQueue(channel chan metric.Metric, initialInterval int, log *l.Entry) Collector {
	ss := new(SocketQueue)
	ss.channel = channel
	ss.interval = initialInterval
	ss.log = log
	ss.name = "SocketQueue"

	return ss
}

// Configure Override default parameters
func (ss *SocketQueue) Configure(configMap map[string]interface{}) {
	if asInterface, exists := configMap["PortList"]; exists {
		ss.portList = config.GetAsSlice(asInterface)
	}
	ss.configureCommonParams(configMap)
}

// Collect the receive queue size (RecvQ)
func (ss SocketQueue) Collect() {
	/** Run the command 'ss -ntl sport = :<port_num> | sport = :<port_num> ...'
	    to obtain the recvQ value
	*/
        cmdArgs := "-ntl sport = :" +  strings.Join(ss.portList, "| sport = :" )

	cmd := exec.Command("ss", cmdArgs)
	output, err := cmdOutput(cmd)
	if err != nil {
		ss.log.Error("Error while collecting metrics: ", err)
		return
	}
	ss.emitSocketQueueMetrics(output)	
}

func (ss SocketQueue) emitSocketQueueMetrics(output []byte) {
	// Capture the receive queue size and the corres. port number from the output.
	re := regexp.MustCompile("\\w+\\s+(\\w+)\\s+\\w+\\s+\\S+:(\\S+).*")
	res := re.FindAllStringSubmatch(string(output), -1)
	pmap := make(map[string]float64)
	for _, v := range res {
		sport, qsize := v[2], v[1]
		pmap[sport] = util.StrToFloat(qsize)
	}

	for sport, qsize := range pmap {
		m := metric.WithValue("sq.listen", qsize)
		m.AddDimension("port", sport)
		ss.Channel() <- m
	}
}
