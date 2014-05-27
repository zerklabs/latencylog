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
	"strconv"
	"strings"
	"time"
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
	snapshots []*Snapshot
)

func main() {
	var (
		tcpAddress  = flag.String("tcp-address", "", "Target IP:PORT to connect to")
		duration    = flag.Int("duration", 1, "How long to run for (minutes)")
		httpAddress = flag.String("http-address", "", "The target HTTP server")
		httpPath    = flag.String("http-path", "/put", "The URI path for the NSQ publisher")
		httpSSL     = flag.Bool("use-https", false, "Use HTTPS?")

		useHttp = false
		httpUri = ""
	)

	flag.Parse()

	if len(*tcpAddress) == 0 {
		fmt.Println("IP:PORT required")

		flag.PrintDefaults()
		return
	}

	if len(*httpAddress) > 0 {
		useHttp = true
		httpUri = fmt.Sprintf("%s%s", *httpAddress, *httpPath)

		if *httpSSL {
			httpUri = "https://" + httpUri
		} else {
			httpUri = "http://" + httpUri
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
			host := strings.Split(*tcpAddress, ":")[0]
			port, _ := strconv.Atoi(strings.Split(*tcpAddress, ":")[1])

			res, err := run(host, port)

			if err != nil {
				log.Fatal(err)
			}

			snapshots = append(snapshots, res)

			if useHttp {
				b, err := json.Marshal(res)

				if err != nil {
					log.Fatal(err)
				}

				buf := bytes.NewBufferString(string(b))
				_, err = client.Post(httpUri, "application/json", buf)

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

	aggregate(snapshots, *duration)
}

func aggregate(snapshots []*Snapshot, duration int) {
	if len(snapshots) > 0 {
		total := float64(0)
		log.Printf("Snapshots: %d", len(snapshots))

		for _, v := range snapshots {
			total += v.Duration
		}

		avg := total / float64(len(snapshots))
		log.Printf("Average after %d minute(s) and %d checks: %vms", duration, len(snapshots), avg)
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
