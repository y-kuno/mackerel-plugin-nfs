package mpnfs

import (
	"encoding/json"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mackerelio/golib/pluginutil"

	"bufio"
	"fmt"
	"io"
	"os"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"gopkg.in/alecthomas/kingpin.v2"
)

/*
1. ops              How many ops of this type have been requested
2. trans:           How many transmissions of this op type have been sent
3. timeouts:        How many timeouts of this op type have occurred
4. bytes_sent:      How many bytes have been sent for this op type
5. bytes_recv:      How many bytes have been received for this op type
6. queue:           How long ops of this type have waited in queue before being transmitted (microsecond)
7. rtt:             How long the client waited to receive replies of this op type from the server (microsecond)
8. execute:         How long ops of this type take to execute (from rpc_init_task to rpc_exit_task) (microsecond)
*/
var rpcOperationCounters = []string{
	"ops",
	"trans",
	"timeouts",
	"bytes_sent",
	"bytes_recv",
	"queue",
	"rtt",
	"execute",
}

var rpcOperations = []string{
	"read",
	"write",
}

// NFSPlugin mackerel plugin
type NFSPlugin struct {
	Prefix   string
	Tempfile string
}

// MetricKeyPrefix interface for PluginWithPrefix
func (np *NFSPlugin) MetricKeyPrefix() string {
	if np.Prefix == "" {
		np.Prefix = "nfs"
	}
	return np.Prefix
}

// GraphDefinition interface for mackerel plugin
func (np *NFSPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.ToTitle(strings.Replace(np.MetricKeyPrefix(), "nfs", "NFS", -1))
	return map[string]mp.Graphs{
		"ops.#": {
			Label: fmt.Sprintf("%s Operations", labelPrefix),
			Unit:  mp.UnitIOPS,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"throughput.#": {
			Label: fmt.Sprintf("%s Throughput", labelPrefix),
			Unit:  mp.UnitBytesPerSecond,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"bytes_ops.#": {
			Label: fmt.Sprintf("%s Byte Per Operations", labelPrefix),
			Unit:  mp.UnitBytes,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"retrans_num.#": {
			Label: fmt.Sprintf("%s Retrans Num", labelPrefix),
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"retrans_rate.#": {
			Label: fmt.Sprintf("%s Retrans Rate", labelPrefix),
			Unit:  mp.UnitPercentage,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"rtt_ave.#": {
			Label: fmt.Sprintf("%s RTT Average (ms)", labelPrefix),
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"execute_ave.#": {
			Label: fmt.Sprintf("%s Execute Average (ms)", labelPrefix),
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
		"queue_ave.#": {
			Label: fmt.Sprintf("%s Queue Average (ms)", labelPrefix),
			Unit:  mp.UnitInteger,
			Metrics: []mp.Metrics{
				{Name: "read", Label: "reads"},
				{Name: "write", Label: "writes"},
			},
		},
	}
}

// FetchMetrics interface for mackerel plugin
func (np *NFSPlugin) FetchMetrics() (map[string]float64, error) {
	now := time.Now()
	file, err := os.Open("/proc/self/mountstats")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	devices, stats, err := np.parseMountStats(file)
	if err != nil {
		return nil, err
	}

	lastStats, lastTime, err := np.fetchLastValues()
	if err != nil {
		log.Println("fetchLastValues (ignore):", err)
	}

	if err := np.saveValues(stats, now); err != nil {
		log.Fatalf("saveValues: %s", err)
	}
	return np.formatValues(devices, stats, now, lastStats, lastTime), nil
}

func (np *NFSPlugin) parseMountStats(out io.Reader) ([]string, map[string]float64, error) {
	mounts := make(map[string][]string)
	var key string
	var values []string

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		line := scanner.Text()
		record := strings.Fields(line)
		if strings.HasPrefix(line, "no device mounted") || len(record) == 0 {
			continue
		}

		if record[0] == "device" {
			if !(strings.Contains(record[7], "nfs") || strings.Contains(record[7], "nfs4")) {
				continue
			}

			if key != record[4] {
				values = []string{}
				key = record[4]
			}
		}
		values = append(values, strings.TrimSpace(line))
		mounts[key] = values
	}

	if len(mounts) == 0 {
		return nil, nil, fmt.Errorf("no nfs mount points were found")
	}

	stats := make(map[string]float64)
	var devices []string
	for k, v := range mounts {
		device := strings.Replace(strings.Trim(k, "/"), "/", "_", -1)
		devices = append(devices, device)
		rpcFlag := false
		for _, s := range v {
			record := strings.Fields(s)
			if len(record) == 0 {
				continue
			}

			if !rpcFlag {
				if record[0] != "RPC" {
					continue
				}
				rpcFlag = true
			}

			switch record[0] {
			case "RPC":
			case "xprt:":
			case "per-op":
				break
			default:
				np.parseRPCLine(stats, device, record)
			}
		}
	}
	return devices, stats, nil
}

func (np *NFSPlugin) parseRPCLine(stats map[string]float64, device string, record []string) error {
	op := string(record[0][:len(record[0])-1])
	if op == "READ" || op == "WRITE" {
		for i, record := range record[1:] {
			v, err := strconv.ParseFloat(record, 64)
			if err != nil {
				return err
			}
			stats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[i], device, strings.ToLower(op))] = v
		}
	}
	return nil
}

func (np *NFSPlugin) tempfileName() string {
	if np.Tempfile != "" {
		return filepath.Join(pluginutil.PluginWorkDir(), np.Tempfile)
	}
	return filepath.Join(pluginutil.PluginWorkDir(), fmt.Sprintf("mackerel-plugin-%s", np.MetricKeyPrefix()))
}

func (np *NFSPlugin) fetchLastValues() (map[string]float64, time.Time, error) {
	lastTime := time.Now()

	f, err := os.Open(np.tempfileName())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, lastTime, nil
		}
		return nil, lastTime, err
	}
	defer f.Close()

	stats := make(map[string]float64)
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&stats)
	lastTime = time.Unix(int64(stats["_lastTime"]), 0)
	if err != nil {
		return stats, lastTime, err
	}
	return stats, lastTime, nil
}

func (np *NFSPlugin) saveValues(values map[string]float64, now time.Time) error {
	f, err := os.Create(np.tempfileName())
	if err != nil {
		return err
	}
	defer f.Close()

	values["_lastTime"] = float64(now.Unix())
	encoder := json.NewEncoder(f)
	err = encoder.Encode(values)
	if err != nil {
		return err
	}
	return nil
}

func (np *NFSPlugin) formatValues(devices []string, stats map[string]float64, now time.Time,
	lastStat map[string]float64, lastTime time.Time) map[string]float64 {
	diffTime := now.Unix() - lastTime.Unix()
	if diffTime > 600 {
		log.Printf("too long duration")
		return nil
	}

	diffStats := make(map[string]float64)
	for k, v := range stats {
		var value float64
		lastValue, ok := lastStat[k]
		if ok {
			var err error
			value, err = np.calcDiff(v, lastValue, diffTime)
			if err != nil {
				log.Println("OutputValues: ", err)
			}
		} else {
			log.Printf("%s does not exist at last fetch\n", k)
			continue
		}
		diffStats[k] = value
	}

	metrics := make(map[string]float64)
	for _, device := range devices {
		for _, op := range rpcOperations {
			ops := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[0], device, op)]
			retrans := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[1], device, op)] - ops
			bytes := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[3], device, op)] +
				diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[4], device, op)]
			queueAve := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[5], device, op)] / ops
			rttAve := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[6], device, op)] / ops
			executeAve := diffStats[fmt.Sprintf("%s.%s.%s", rpcOperationCounters[7], device, op)] / ops

			metrics[fmt.Sprintf("ops.%s.%s", device, op)] = ops / float64(diffTime)
			metrics[fmt.Sprintf("throughput.%s.%s", device, op)] = bytes / float64(diffTime)
			bytesOps := bytes / ops
			if !math.IsNaN(bytesOps) {
				metrics[fmt.Sprintf("bytes_ops.%s.%s", device, op)] = bytesOps
			}
			metrics[fmt.Sprintf("retrans_num.%s.%s", device, op)] = retrans
			retransRate := retrans / ops
			if !math.IsNaN(retransRate) {
				metrics[fmt.Sprintf("retrans_rate.%s.%s", device, op)] = retrans / ops
			}
			if !math.IsNaN(rttAve) {
				metrics[fmt.Sprintf("rtt_ave.%s.%s", device, op)] = rttAve
			}
			if !math.IsNaN(executeAve) {
				metrics[fmt.Sprintf("execute_ave.%s.%s", device, op)] = executeAve
			}
			if !math.IsNaN(queueAve) {
				metrics[fmt.Sprintf("queue_ave.%s.%s", device, op)] = queueAve
			}
		}
	}
	return metrics
}

func (np *NFSPlugin) calcDiff(value float64, lastValue float64, diffTime int64) (float64, error) {
	diff := value - lastValue
	if diff < 0 {
		return 0, fmt.Errorf("counter seems to be reset")
	}
	return diff, nil
}

// Do the plugin
func Do() {
	optPrefix := kingpin.Flag("metric-key-prefix", "Metric key prefix").Default("nfs").String()
	optTempfile := kingpin.Flag("tempfile", "Temp file name").String()
	kingpin.Parse()

	plugin := mp.NewMackerelPlugin(&NFSPlugin{
		Prefix:   *optPrefix,
		Tempfile: *optTempfile,
	})
	plugin.Run()
}
