package main

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const showqSocket = "/var/spool/postfix/public/showq"

type PostfixCollector struct {
	queueLength  *prometheus.Desc
	queueBytes   *prometheus.Desc
	oldestMsgAge *prometheus.Desc
}

func NewPostfixCollector() *PostfixCollector {
	return &PostfixCollector{
		queueLength: prometheus.NewDesc(
			"postfix_queue_length",
			"Number of messages in the Postfix queue.",
			[]string{"queue"}, nil,
		),
		queueBytes: prometheus.NewDesc(
			"postfix_queue_bytes",
			"Total size of messages in the queue in bytes.",
			[]string{"queue"}, nil,
		),
		oldestMsgAge: prometheus.NewDesc(
			"postfix_queue_oldest_message_age_seconds",
			"Age of the oldest message in the queue in seconds.",
			[]string{"queue"}, nil,
		),
	}
}

func (c *PostfixCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.queueLength
	ch <- c.queueBytes
	ch <- c.oldestMsgAge
}

func (c *PostfixCollector) Collect(ch chan<- prometheus.Metric) {
	conn, err := net.Dial("unix", showqSocket)
	if err != nil {
		log.Printf("Could not connect to Postfix showq socket: %v", err)
		return
	}
	defer conn.Close()

	now := time.Now().Unix()
	
	// Aggregators per queue (active, deferred, hold, incoming)
	counts := make(map[string]float64)
	sizes := make(map[string]float64)
	oldest := make(map[string]int64)

	scanner := bufio.NewScanner(conn)
	scanner.Split(splitNull)

	var currentQueue string
	var currentSize float64
	var currentArrival int64

	for scanner.Scan() {
		key := scanner.Text()
		if !scanner.Scan() { break }
		val := scanner.Text()

		switch key {
		case "queue_name":
			currentQueue = val
		case "message_size":
			currentSize, _ = strconv.ParseFloat(val, 64)
		case "arrival_time":
			currentArrival, _ = strconv.ParseInt(val, 10, 64)
		case "queue_id":
			// A queue_id signifies a complete message record in the stream
			counts[currentQueue]++
			sizes[currentQueue] += currentSize
			
			age := now - currentArrival
			if age > oldest[currentQueue] {
				oldest[currentQueue] = age
			}
		}
	}

	for q, count := range counts {
		ch <- prometheus.MustNewConstMetric(c.queueLength, prometheus.GaugeValue, count, q)
		ch <- prometheus.MustNewConstMetric(c.queueBytes, prometheus.GaugeValue, sizes[q], q)
		ch <- prometheus.MustNewConstMetric(c.oldestMsgAge, prometheus.GaugeValue, float64(oldest[q]), q)
	}
}

// splitNull handles the Postfix null-terminated binary protocol
func splitNull(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 { return 0, nil, nil }
	if i := bytes.IndexByte(data, 0); i >= 0 { return i + 1, data[0:i], nil }
	return 0, nil, nil
}

func main() {
	prometheus.MustRegister(NewPostfixCollector())
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Postfix Exporter listening on :9154")
	log.Fatal(http.ListenAndServe(":9154", nil))
}
