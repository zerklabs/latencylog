package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	// "github.com/bitly/nsq/util"
)

type Snapshot struct {
	Duration         float64   `json:"duration"`
	DurationFormat   string    `json:"durationformat"`
	DistanceEstimate string    `json:"distanceestimate"`
	End              time.Time `json:"end"`
	Host             string    `json:"host"`
	Port             int       `json:"port"`
	Start            time.Time `json:"start"`
	LocalIP          string    `json:"localip"`
	LocalPort        string    `json:"localport"`
}

var (
	host     = flag.String("host", "", "Target IP to connect to. Required")
	port     = flag.Int("port", 0, "Target port to connect to. Required")
	duration = flag.Int("duration", 1, "How long to run for (minutes)")
	nsqHost  = flag.String("nsq-http-address", "", "The NSQ target HTTP server")
	nsqPort  = flag.Int("nsq-http-port", 4151, "The NSQ target HTTP server port")
	nsqPath  = flag.String("nsq-http-path", "/put", "The URI path for the NSQ publisher")
	nsqSSL   = flag.Bool("nsq-https", false, "Use HTTPS for NSQ connection? (Default: true)")
	nsqTopic = flag.String("nsq-topic", "latencylog", "Specify the topic used in NSQ")

	// nsqdTCPAddrs = util.StringArray{}

	useNsq = false
	nsqUri = ""

	snapshots []*Snapshot
)

// func init() {
// 	flag.Var(&nsqdTCPAddrs, "nsqd-tcp-address", "nsqd TCP address (may be given multiple times)")
// }

func main() {
	flag.Parse()

	if len(*host) == 0 {
		fmt.Println("Host required")

		flag.PrintDefaults()
		return
	}

	if *port == 0 {
		fmt.Println("Port required")

		flag.PrintDefaults()
		return
	}

	if len(*nsqHost) > 0 {
		useNsq = true
		nsqUri = fmt.Sprintf("%s:%d%s?topic=%s", *nsqHost, *nsqPort, *nsqPath, *nsqTopic)

		if *nsqSSL {
			nsqUri = "https://" + nsqUri
		} else {
			nsqUri = "http://" + nsqUri
		}
	}

	snapshots = make([]*Snapshot, 0)

	tickChan := time.NewTicker(time.Second * 1)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr}

	go func() {
		for _ = range tickChan.C {
			res, err := run(*host, *port)

			if err != nil {
				log.Fatal(err)
			}

			snapshots = append(snapshots, res)

			if useNsq {
				b, err := json.Marshal(res)

				if err != nil {
					log.Fatal(err)
				}

				buf := bytes.NewBufferString(string(b))
				_, err = client.Post(nsqUri, "application/json", buf)

				if err != nil {
					log.Printf("HTTP Error: %s", err)
				}
			}

			log.Printf("%v, %s:%d", res.Duration, res.Host, res.Port)
		}
	}()

	sleepFor, _ := time.ParseDuration(fmt.Sprintf("%dm", *duration))
	log.Printf("Running for: %s", sleepFor)
	time.Sleep(sleepFor)
	tickChan.Stop()

	if len(snapshots) > 0 {
		total := float64(0)
		log.Printf("Snapshots: %d", len(snapshots))

		for _, v := range snapshots {
			total += v.Duration
		}

		avg := total / float64(len(snapshots))
		log.Printf("Average after %d minute(s) and %d checks: %vms", *duration, len(snapshots), avg)
	}
}

func run(host string, port int) (*Snapshot, error) {
	start := time.Now()
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))

	if err != nil {
		return &Snapshot{}, err
	}

	c.Write(make([]byte, 8))

	end := time.Now()
	dur := end.Sub(start)

	snap := &Snapshot{
		Start:          start,
		End:            end,
		Duration:       dur.Seconds() * 1e3,
		DurationFormat: "ms",
		Host:           host,
		Port:           port,
		LocalIP:        strings.Split(c.LocalAddr().String(), ":")[0],
		LocalPort:      strings.Split(c.LocalAddr().String(), ":")[1],
	}

	if snap.Duration < float64(10) {
		snap.DistanceEstimate = "near"
	} else if snap.Duration > float64(40) {
		snap.DistanceEstimate = "far"
	} else {
		snap.DistanceEstimate = "close"
	}

	if err := c.Close(); err != nil {
		return snap, err
	}

	return snap, nil
}
