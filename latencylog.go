package main

import (
	// "bytes"
	"crypto/tls"
	// "encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/influxdb/influxdb-go"
)

type Snapshot struct {
	Duration         float64   `json:"duration"`
	DurationFormat   string    `json:"durationformat"`
	DistanceEstimate string    `json:"distanceestimate"`
	Status           int       `json:"status"`
	End              time.Time `json:"end"`
	Host             string    `json:"host"`
	Port             int       `json:"port"`
	Start            time.Time `json:"start"`
	LocalIP          string    `json:"localip"`
	LocalPort        string    `json:"localport"`
}

var (
	snapshots []*Snapshot

	httpUri = ""
	useHttp = false

	client *http.Client
	db     *influxdb.Client
)

func main() {
	var (
		tcpAddress = flag.String("tcp-address", "", "Target IP:PORT to connect to")
		duration   = flag.Int("duration", 1, "How long to run for (minutes)")
		// httpAddress = flag.String("http-address", "", "The target HTTP server")
		// httpPath    = flag.String("http-path", "/put", "The URI path for the NSQ publisher")
		// httpSSL     = flag.Bool("use-https", false, "Use HTTPS?")

		err error
	)

	flag.Parse()

	if len(*tcpAddress) == 0 {
		fmt.Println("IP:PORT required")

		flag.PrintDefaults()
		return
	}

	influxConfig := &influxdb.ClientConfig{
		Host:     "api.cobhamna.com/influxdb",
		Database: "latencylog-metrics",
		Username: "latencylog",
		Password: "haej0Zoh",
		IsSecure: true,
	}

	db, err = influxdb.NewClient(influxConfig)

	if err != nil {
		panic(err)
	}

	// if len(*httpAddress) > 0 {
	// 	useHttp = true
	// 	httpUri = fmt.Sprintf("%s%s", *httpAddress, *httpPath)

	// 	if *httpSSL {
	// 		httpUri = "https://" + httpUri
	// 	} else {
	// 		httpUri = "http://" + httpUri
	// 	}
	// }

	snapshots = make([]*Snapshot, 0)

	tickChan := time.NewTicker(time.Second * 1)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client = &http.Client{Transport: tr}

	go func() {
		for _ = range tickChan.C {
			host := strings.Split(*tcpAddress, ":")[0]
			port, _ := strconv.Atoi(strings.Split(*tcpAddress, ":")[1])

			res, err := run(host, port)

			if err != nil {
				log.Fatal(err)
			}

			log.Printf("%v, %s:%d", res.Duration, res.Host, res.Port)

			snapshots = append(snapshots, res)
		}
	}()

	sleepFor, _ := time.ParseDuration(fmt.Sprintf("%dm", *duration))
	log.Printf("Running for: %s", sleepFor)
	time.Sleep(sleepFor)
	tickChan.Stop()

	aggregate(snapshots, *duration)
}

// func record(snapshot *Snapshot) {
// 	// if useHttp {
// 	// 	b, err := json.Marshal(snapshot)

// 	// 	if err != nil {
// 	// 		log.Fatal(err)
// 	// 	}

// 	// 	buf := bytes.NewBufferString(string(b))
// 	// 	_, err = client.Post(httpUri, "application/json", buf)

// 	// 	if err != nil {
// 	// 		log.Printf("HTTP Error: %s", err)
// 	// 	}
// 	// }

// 	series := []*influxdb.Series{
// 		&influxdb.Series{
// 			Name:    "measurements",
// 			Columns: []string{"source", "destination", "duration", "status"},
// 			Points: [][]interface{}{
// 				{fmt.Sprintf("%s:%s", snapshot.LocalIP, snapshot.LocalPort), fmt.Sprintf("%s:%d", snapshot.Host, snapshot.Port), snapshot.Duration, snapshot.Status},
// 			},
// 		},
// 	}

// 	err := db.WriteSeries(series)
// 	if err != nil {
// 		panic(err)
// 	}
// }

func aggregate(snapshots []*Snapshot, duration int) {
	if len(snapshots) > 0 {
		series := make([]*influxdb.Series, 0)
		total := float64(0)
		log.Printf("Snapshots: %d", len(snapshots))

		for _, v := range snapshots {
			total += v.Duration
			series = append(series, &influxdb.Series{
				Name:    "measurements",
				Columns: []string{"source", "destination", "duration", "status", "start", "end"},
				Points: [][]interface{}{
					{fmt.Sprintf("%s:%s", v.LocalIP, v.LocalPort), fmt.Sprintf("%s:%d", v.Host, v.Port), v.Duration, v.Status, v.Start, v.End},
				},
			})
		}

		avg := total / float64(len(snapshots))
		log.Printf("Average after %d minute(s) and %d checks: %vms", duration, len(snapshots), avg)

		err := db.WriteSeries(series)
		if err != nil {
			panic(err)
		}
	}
}

func run(host string, port int) (*Snapshot, error) {
	snap := &Snapshot{}

	snap.DurationFormat = "ms"
	snap.Host = host
	snap.Port = port

	snap.Start = time.Now()
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))

	if err != nil {
		snap.Status = 0
		snap.End = time.Now()
		return snap, err
	}

	c.Write(make([]byte, 8))

	snap.End = time.Now()
	dur := snap.End.Sub(snap.Start)

	snap.Duration = dur.Seconds() * 1e3
	snap.LocalIP = strings.Split(c.LocalAddr().String(), ":")[0]
	snap.LocalPort = strings.Split(c.LocalAddr().String(), ":")[1]
	snap.Status = 1

	if snap.Duration <= float64(10) {
		snap.DistanceEstimate = "sub10ms"
	} else if snap.Duration <= float64(20) {
		snap.DistanceEstimate = "sub20ms"
	} else if snap.Duration <= float64(30) {
		snap.DistanceEstimate = "sub30ms"
	} else if snap.Duration <= float64(40) {
		snap.DistanceEstimate = "sub40ms"
	} else if snap.Duration <= float64(50) {
		snap.DistanceEstimate = "sub50ms"
	} else if snap.Duration <= float64(60) {
		snap.DistanceEstimate = "sub60ms"
	} else if snap.Duration <= float64(70) {
		snap.DistanceEstimate = "sub70ms"
	} else if snap.Duration <= float64(80) {
		snap.DistanceEstimate = "sub80ms"
	} else if snap.Duration <= float64(90) {
		snap.DistanceEstimate = "sub90ms"
	} else {
		snap.DistanceEstimate = "above90ms"
	}

	if err := c.Close(); err != nil {
		return snap, err
	}

	return snap, nil
}
