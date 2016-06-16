package util

import (
	"encoding/json"
	"fmt"
	"fullerite/config"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// For dependency injection
var (
	ipGetter   = getIps
	httpRegexp = regexp.MustCompile(`http`)
)

// example configuration::
//
// {
//     	"heartbeat_path": "/var/run/nerve/heartbeat",
//		"instance_id": "srv1-devc",
//		"services": {
//	 		"<SERVICE_NAME>.<NAMESPACE>.<otherstuff>": {
//				"host": "<IPADDR>",
//      	    "port": ###,
//      	}
// 		}
//     "services": {
//
// Most imporantly is the port, host and service name. The service name is assumed to be formatted like this::
//
type nerveConfigData struct {
	Services map[string]map[string]interface{}
}

// NerveService is an exported struct containing services' info
type NerveService struct {
	Name      string
	Namespace string
}

// EndPoint defines a struct for endpoints
type EndPoint struct {
	Host string
	Port string
}

// ParseNerveConfig is responsible for taking the JSON string coming in into a map of service:port
// it will also filter based on only services runnign on this host.
// To deal with multi-tenancy we actually will return port:service
func ParseNerveConfig(raw *[]byte) (map[int]NerveService, error) {
	results := make(map[int]NerveService)
	ips, err := ipGetter()

	if err != nil {
		return results, err
	}
	parsed := new(nerveConfigData)

	// convert the ips into a map for membership tests
	ipMap := make(map[string]bool)
	for _, val := range ips {
		ipMap[val] = true
	}

	err = json.Unmarshal(*raw, parsed)
	if err != nil {
		return results, err
	}

	for rawServiceName, serviceConfig := range parsed.Services {
		host := strings.TrimSpace(serviceConfig["host"].(string))

		_, exists := ipMap[host]
		if exists {
			service := new(NerveService)
			service.Name = strings.Split(rawServiceName, ".")[0]
			service.Namespace = strings.Split(rawServiceName, ".")[1]
			port := extractPort(serviceConfig)
			if port != -1 {
				results[port] = *service
			}
		}
	}

	return results, nil
}

func extractPort(serviceConfig map[string]interface{}) int {
	checkConfig := make(map[string]string)

	if checkArray, ok := serviceConfig["checks"].([]interface{}); ok {
		checkConfig = config.GetAsMap(checkArray[0])
	}

	if uri, ok := checkConfig["uri"]; ok {
		uriArray := strings.Split(uri, "/")
		protocol := strings.TrimSpace(uriArray[1])
		port := uriArray[3]
		if !httpRegexp.MatchString(protocol) {
			return -1
		}
		if portInt, err := strconv.ParseInt(port, 10, 64); err == nil {
			return int(portInt)
		}
		return -1
	}
	return -1
}

// getIps is responsible for getting all the ips that are associated with this NIC
func getIps() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}, err
	}

	results := []string{}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return []string{}, err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			results = append(results, ip.String())
		}
	}

	return results, nil
}

// CreateMinimalNerveConfig creates a minimal nerve config
func CreateMinimalNerveConfig(config map[string]EndPoint) map[string]map[string]map[string]interface{} {
	minimalNerveConfig := make(map[string]map[string]map[string]interface{})
	serviceConfigs := make(map[string]map[string]interface{})
	for service, endpoint := range config {
		uriEndPoint := fmt.Sprintf("/http/%s/%s/status", service, endpoint.Port)
		serviceConfigs[service] = map[string]interface{}{
			"host": endpoint.Host,
			"port": endpoint.Port,
			"checks": []interface{}{
				map[string]interface{}{
					"uri": uriEndPoint,
				},
			},
		}
	}
	minimalNerveConfig["services"] = serviceConfigs
	return minimalNerveConfig
}
