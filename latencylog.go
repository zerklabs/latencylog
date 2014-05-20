package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type Snapshot struct {
	Duration       float64   `json:"duration"`
	DurationFormat string    `json:"durationformat"`
	End            time.Time `json:"end"`
	Host           string    `json:"host"`
	Port           int       `json:"port"`
	Start          time.Time `json:"start"`
	LocalIP        string    `json:"localip"`
	LocalPort      string    `json:"localport"`
}

var (
	host      = flag.String("host", "", "Target IP to connect to. Required")
	port      = flag.Int("port", 0, "Target port to connect to. Required")
	duration  = flag.Int("duration", 1, "How long to run for (minutes)")
	snapshots []*Snapshot
)

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

	snapshots = make([]*Snapshot, 0)

	tickChan := time.NewTicker(time.Second * 1)
	client := &http.Client{}

	go func() {
		for _ = range tickChan.C {
			res, err := run(*host, *port)

			if err != nil {
				log.Fatal(err)
			}

			snapshots = append(snapshots, res)

			b, err := json.Marshal(res)

			if err != nil {
				log.Fatal(err)
			}

			buf := bytes.NewBufferString(string(b))
			client.Post("http://10.127.144.40:4151/put?topic=latencylog", "application/json", buf)

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

	if err := c.Close(); err != nil {
		return snap, err
	}

	return snap, nil
}
