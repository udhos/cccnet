package main

import (
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"gopkg.in/v2/yaml"
)

type config struct {
	Postgres     string
	CcmEndpoint  string
	CcmList      []ccm
	RegionList   []region
	LogCollector string
}

type ccm struct {
	Name string
	Host string
}

type region struct {
	Name                  string
	CcoEndpoint           string
	CcoList               []cco
	RabbitEndpointPublic  string
	RabbitEndpointPrivate string
	RabbitList            []rabbit
}

type cco struct {
	Name string
	Host string
}

type rabbit struct {
	Name string
	Host string
}

var verbose bool
var dump bool

func main() {

	me := path.Base(os.Args[0])
	if len(os.Args) < 2 {
		log.Printf("usage: %s location", me)
		log.Print("location is one of: browser worker ccoName ccmName rabbitName", me)
		os.Exit(1)
	}
	location := os.Args[1]

	verbose = os.Getenv("VERBOSE") != ""
	dump = os.Getenv("DUMP") != ""
	log.Printf("runtime %s VERBOSE=%v DUMP=%v", runtime.Version(), verbose, dump)

	cfg := config{}

	dec := yaml.NewDecoder(os.Stdin)
	log.Printf("reading config from stdin...")
	if errDec := dec.Decode(&cfg); errDec != nil {
		log.Printf("error decoding config from stdin: %v", errDec)
	}
	log.Printf("reading config from stdin...done")

	result := run(&cfg, location)

	if dump {
		enc := yaml.NewEncoder(os.Stdout)
		log.Printf("writing config to stdout...")
		if errEnc := enc.Encode(&cfg); errEnc != nil {
			log.Printf("error encoding config to stdout: %v", errEnc)
		}
		log.Printf("writing config to stdout...done")
	}

	if !result {
		log.Printf("FAILURE")
		os.Exit(2)
	}
	if verbose {
		log.Printf("SUCCESS")
	}
}

func run(cfg *config, location string) bool {

	switch {
	case location == "browser":
		return runBrowser(cfg, location)
	case strings.HasPrefix(location, "ccm"):
		return runCcm(cfg, location)
	case strings.HasPrefix(location, "cco"):
		return runCco(cfg, location)
	case strings.HasPrefix(location, "rabbit"):
		return runRabbit(cfg, location)
	case location == "worker":
		return runWorker(cfg, location)
	default:
		log.Printf("bad location: %s", location)
	}

	return false
}

func runBrowser(cfg *config, location string) bool {
	result := true
	if !test(location, "ccm", cfg.CcmEndpoint, ":443") {
		result = false
	}
	if !test(location, "log-collector", cfg.LogCollector, ":8882") {
		result = false
	}
	for _, r := range cfg.RegionList {
		if !test(location, r.Name+",rabbit-lb-public", r.RabbitEndpointPublic, ":443") {
			result = false
		}
	}
	return result
}

func runRabbit(cfg *config, location string) bool {
	result := true
	if !test(location, "ccm", cfg.CcmEndpoint, ":443") {
		result = false
	}
	for _, reg := range cfg.RegionList {
		for _, rab := range reg.RabbitList {
			if rab.Name == location {
				// found this rabbit

				// 1. test CCO LB
				if !test(location, reg.Name+",cco-lb", reg.CcoEndpoint, ":8443") {
					result = false
				}
				// 2. test rabbit LB
				if !test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":7788") {
					result = false
				}
				// 3. test other rabbits
				for _, other := range reg.RabbitList {
					if !test(location, reg.Name+","+other.Name, other.Host, ":7788") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":4369") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":25672") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":22") {
						result = false
					}
				}
				return result
			}
		}
	}
	if verbose {
		log.Printf("could not find this rabbit: %s", location)
	}
	return false
}

func runCcm(cfg *config, location string) bool {
	result := true
	if !test(location, "postgres", cfg.Postgres, ":5432") {
		result = false
	}
	if !test(location, "log-collector", cfg.LogCollector, ":4560") {
		result = false
	}
	if !test(location, "log-collector", cfg.LogCollector, ":8881") {
		result = false
	}
	for _, reg := range cfg.RegionList {
		if !test(location, reg.Name+",cco-lb", reg.CcoEndpoint, ":8443") {
			result = false
		}
	}
	for _, ccm := range cfg.CcmList {
		if !test(location, ccm.Name, ccm.Host, ":22") {
			result = false
		}
	}
	return result
}

func runCco(cfg *config, location string) bool {
	result := true
	if !test(location, "log-collector", cfg.LogCollector, ":4560") {
		result = false
	}
	if !test(location, "log-collector", cfg.LogCollector, ":8881") {
		result = false
	}
	if !test(location, "ccm", cfg.CcmEndpoint, ":8443") {
		result = false
	}
	for _, reg := range cfg.RegionList {
		for _, o := range reg.CcoList {
			if o.Name == location {
				// found this cco

				// 1. test rabbit LB
				if !test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":5671") {
					result = false
				}

				// 3. test other ccos
				for _, other := range reg.CcoList {
					if !test(location, reg.Name+","+other.Name, other.Host, ":22") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":5701") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":27017") {
						result = false
					}
					if !test(location, reg.Name+","+other.Name, other.Host, ":8443") {
						result = false
					}
				}

				return result
			}
		}
	}
	if verbose {
		log.Printf("could not find this cco: %s", location)
	}
	return false
}

func runWorker(cfg *config, location string) bool {
	result := true
	for _, reg := range cfg.RegionList {
		if !test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":5671") {
			result = false
		}
		if !test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":7789") {
			result = false
		}
	}
	return result
}

func test(location, target, host, port string) bool {
	endp := host + port
	if verbose {
		log.Printf("%s: target=%s: opening: %s", location, target, endp)
	}
	connected := open(endp)
	if !connected {
		log.Printf("%s: target=%s: failure: %s", location, target, endp)
		return false
	}
	if verbose {
		log.Printf("%s: target=%s: connected: %s", location, target, endp)
	}
	return true
}

func open(addr string) bool {

	timeout := 3 * time.Second

	c, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		c.Close()
		return true
	}

	log.Printf("connect failure: %s: %v", addr, err)

	return false
}
