package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

func parseFlagValue[T cmp.Ordered](v string, defaultValue T) (any, error) {
	var err error
	value := defaultValue
	if len(v) > 0 {
		if _, ok := any(value).(time.Duration); ok {
			return time.ParseDuration(v)
		}
		_, err = fmt.Sscanf(v, "%v", &value)
	}
	return value, err
}

type arrayFlag[T cmp.Ordered] struct {
	values       []T
	defaultValue T
}

func (af *arrayFlag[T]) String() string {
	if len(af.values) > 0 {
		strVals := make([]string, len(af.values))
		for i, v := range af.values {
			strVals[i] = fmt.Sprintf("%v", v)
		}
		return strings.Join(strVals, ",")
	}
	return fmt.Sprintf("%v", af.defaultValue)
}

func (af *arrayFlag[T]) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		if val, err := parseFlagValue(v, af.defaultValue); err != nil {
			return fmt.Errorf("failed to parse value %v for type %t", val)
		} else {
			af.values = append(af.values, val.(T))
		}
	}
	return nil
}

func (af *arrayFlag[T]) total() []T {
	if len(af.values) == 0 {
		return []T{af.defaultValue}
	}
	return af.values
}

func (af *arrayFlag[T]) getArg(idx int) T {
	if len(af.values) == 0 || idx >= len(af.values) {
		return af.defaultValue
	}
	return af.values[idx]
}

func newArrayFlag[T cmp.Ordered](name string, defaultValue T, description string) *arrayFlag[T] {
	description += "\nSupports an `array` of values separated by comma or specified via multiple flags."
	a := &arrayFlag[T]{
		defaultValue: defaultValue,
	}
	flag.Var(a, name, description)
	return a
}

var (
	listenAddr                 = flag.String("httpListenAddr", ":8436", "TCP address for incoming HTTP requests")
	labelName                  = newArrayFlag("labelName", "instance", "Label name, which differs for all state copies")
	jobName                    = newArrayFlag("jobName", "node_exporter", "Scrape job name")
	targetsCount               = newArrayFlag("targetsCount", 100, "The number of scrape targets to return from -httpListenAddr. Each target has the same address defined by -targetAddr")
	targetAddr                 = newArrayFlag("targetAddr", "demo.robustperception.io:9090", "Address with port to use as target address the scrape config returned from -httpListenAddr")
	scrapeInterval             = newArrayFlag("scrapeInterval", time.Second*5, "The scrape_interval to set at the scrape config returned from -httpListenAddr")
	scrapeConfigUpdateInterval = newArrayFlag("scrapeConfigUpdateInterval", time.Minute*10, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr every -scrapeConfigUpdateInterval")
	scrapeConfigUpdatePercent  = newArrayFlag("scrapeConfigUpdatePercent", 1.0, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr ever -scrapeConfigUpdateInterval")
	scrapeConfigMetricRelabel  = newArrayFlag("scrapeConfigMetricRelabel", "", "Path to metric relabel configuration for scrape targets")
)

func main() {
	flag.Parse()
	flag.VisitAll(func(f *flag.Flag) {
		log.Printf("-%s=%s", f.Name, f.Value.String())
	})
	uniqueJobs := make(map[string]struct{})
	for _, job := range jobName.total() {
		uniqueJobs[job] = struct{}{}
	}

	log.Printf("creating %d jobs", len(uniqueJobs))
	targets := make([]*target, len(uniqueJobs))
	c := &config{
		ScrapeConfigs: make([]*yaml.Node, len(targets)),
	}
	for i := range targets {
		targets[i] = &target{
			config: newScrapeConfig(
				targetsCount.getArg(i),
				scrapeInterval.getArg(i),
				targetAddr.getArg(i),
				labelName.getArg(i),
				jobName.getArg(i),
				scrapeConfigMetricRelabel.getArg(i),
			),
			updateInterval: scrapeConfigUpdateInterval.getArg(i),
			updatePercent:  scrapeConfigUpdatePercent.getArg(i) / 100,
		}
		go targets[i].run()
	}
	rh := func(w http.ResponseWriter, r *http.Request) {
		for i := range targets {
			c.ScrapeConfigs[i] = targets[i].marshal()
		}
		data := c.marshalYAML()
		w.Header().Set("Content-Type", "text/yaml")
		w.Write(data)
	}
	hf := http.HandlerFunc(rh)
	log.Printf("starting scrape config updater at http://%s/", *listenAddr)
	if err := http.ListenAndServe(*listenAddr, hf); err != nil {
		log.Fatalf("unexpected error when running the http server: %s", err)
	}
}

func (c *config) marshalYAML() []byte {
	data, err := yaml.Marshal(c)
	if err != nil {
		log.Fatalf("BUG: unexpected error when marshaling config: %s", err)
	}
	return data
}

func newScrapeConfig(targetsCount int, scrapeInterval time.Duration, targetAddr, labelName, jobName, metricRelabel string) *scrapeConfig {
	scs := make([]*staticConfig, 0, targetsCount)
	for i := 0; i < targetsCount; i++ {
		scs = append(scs, &staticConfig{
			Targets: []string{targetAddr},
			Labels: map[string]string{
				labelName:  fmt.Sprintf("%s-%d", labelName, i),
				"revision": "r0",
			},
		})
	}
	var mrc []*metricRelabelConfig
	if len(metricRelabel) > 0 {
		if data, err := os.ReadFile(metricRelabel); err != nil {
			log.Fatalf("failed to open %q metric relabel config file: %v", metricRelabel, err)
		} else if err := yaml.Unmarshal(data, &mrc); err != nil {
			log.Fatalf("failed to parse %q metric relabel config: %v", metricRelabel, err)
		}
	}
	return &scrapeConfig{
		JobName:              jobName,
		ScrapeInterval:       scrapeInterval,
		StaticConfigs:        scs,
		MetricRelabelConfigs: mrc,
	}
}

type target struct {
	config         *scrapeConfig
	updatePercent  float64
	updateInterval time.Duration
	mu             sync.Mutex
}

func (t *target) run() {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	rev := 0
	for range time.Tick(t.updateInterval) {
		rev++
		revStr := fmt.Sprintf("r%d", rev)
		t.mu.Lock()
		for _, sc := range t.config.StaticConfigs {
			if r.Float64() >= t.updatePercent {
				continue
			}
			sc.Labels["revision"] = revStr
		}
		t.mu.Unlock()
	}
}

func (t *target) marshal() *yaml.Node {
	n := &yaml.Node{}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := n.Encode(t.config); err != nil {
		log.Fatalf("BUG: unexpected error when marshaling scrape config: %s", err)
	}
	return n
}

// config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type config struct {
	ScrapeConfigs []*yaml.Node `yaml:"scrape_configs"`
}

// rapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type scrapeConfig struct {
	JobName              string                 `yaml:"job_name"`
	ScrapeInterval       time.Duration          `yaml:"scrape_interval"`
	StaticConfigs        []*staticConfig        `yaml:"static_configs"`
	MetricRelabelConfigs []*metricRelabelConfig `yaml:"metric_relabel_configs,omitempty"`
}

// staticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type staticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

// metricRelabelConfig represents `metric_relabel_configs` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#metric_relabel_configs
type metricRelabelConfig struct {
	If          string `yaml:"if,omitempty"`
	Action      string `yaml:"action,omitempty"`
	Regex       string `yaml:"regex,omitempty"`
	Replacement string `yaml:"replacement,omitempty"`
	TargetLabel string `yaml:"target_label,omitempty"`
	SourceLabel string `yaml:"source_label,omitempty"`
}
