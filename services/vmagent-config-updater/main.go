package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"encoding/binary"
	"hash/fnv"

	"gopkg.in/yaml.v3"
)

func parseFlagValue[T cmp.Ordered | bool](v string, defaultValue T) (any, error) {
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

type arrayFlag[T cmp.Ordered | bool] struct {
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

func newArrayFlag[T cmp.Ordered | bool](name string, defaultValue T, description string) *arrayFlag[T] {
	description += "\nSupports an `array` of values separated by comma or specified via multiple flags."
	a := &arrayFlag[T]{
		defaultValue: defaultValue,
	}
	flag.Var(a, name, description)
	return a
}

// fnvHash64 returns a 64-bit FNV-1a hash for the provided byte slices.
func fnvHash64(parts ...[]byte) uint64 {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write(p)
		// separator to avoid accidental collisions across boundaries
		h.Write([]byte{0})
	}
	return h.Sum64()
}

var (
	listenAddr                 = flag.String("httpListenAddr", ":8436", "TCP address for incoming HTTP requests")
	labelName                  = newArrayFlag("labelName", "instance", "Label name, which differs for all state copies")
	jobName                    = newArrayFlag("jobName", "node_exporter", "Scrape job name")
	targetRequiresK8sAuth      = newArrayFlag("targetRequiresK8sAuth", false, "Defines if target requires K8s auth token")
	targetsCount               = newArrayFlag("targetsCount", 100, "The number of scrape targets to return from -httpListenAddr. Each target has the same address defined by -targetAddr")
	targetAddr                 = newArrayFlag("targetAddr", "demo.robustperception.io:9090", "Address with port to use as target address the scrape config returned from -httpListenAddr")
	scrapeInterval             = newArrayFlag("scrapeInterval", time.Second*5, "The scrape_interval to set at the scrape config returned from -httpListenAddr")
	scrapeConfigUpdateInterval = newArrayFlag("scrapeConfigUpdateInterval", time.Minute*10, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr every -scrapeConfigUpdateInterval")
	scrapeConfigUpdatePercent  = newArrayFlag("scrapeConfigUpdatePercent", 1.0, "The -scrapeConfigUpdatePercent scrape targets are updated in the scrape config returned from -httpListenAddr ever -scrapeConfigUpdateInterval")
	scrapeConfigMetricRelabel  = newArrayFlag("scrapeConfigMetricRelabel", "", "Path to metric relabel configuration for scrape targets")
	revisionPoolSize           = newArrayFlag("revisionPoolSize", 0, "If >0, limit distinct revision values per target by cycling r0..r{n-1} to prevent unbounded new-series growth.")
	// Ramp-up controls (phase 1) and steady-state controls (phase 2)
	rampDuration       = newArrayFlag("rampDuration", time.Duration(0), "Optional ramp-up duration. During this time, rampUpdatePercent/Interval are used before switching to steady state.")
	rampUpdatePercent  = newArrayFlag("rampUpdatePercent", 0.0, "Percent of targets to update per ramp interval during ramp-up phase.")
	rampUpdateInterval = newArrayFlag("rampUpdateInterval", time.Duration(0), "Interval between updates during ramp-up phase.")

	// Determinism controls to avoid spikes on restarts
	epochAnchorRFC3339 = flag.String("epochAnchor", "1970-01-01T00:00:00Z", "RFC3339 anchor timestamp for deterministic schedule (prevents restart spikes).")
	seedString         = flag.String("seed", "vmbench", "Deterministic seed string used for hashing target update selections.")
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

	anchor, err := time.Parse(time.RFC3339, *epochAnchorRFC3339)
	if err != nil {
		log.Fatalf("failed to parse -epochAnchor: %v", err)
	}
	seedHash := fnvHash64([]byte(*seedString))

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
				targetRequiresK8sAuth.getArg(i),
			),
			// revision pool size limit
			revisionPoolSize: int64(revisionPoolSize.getArg(i)),
			// steady-state
			steadyUpdatePercent:  scrapeConfigUpdatePercent.getArg(i) / 100,
			steadyUpdateInterval: scrapeConfigUpdateInterval.getArg(i),
			// ramp (phase 1)
			rampDuration:       rampDuration.getArg(i),
			rampUpdatePercent:  rampUpdatePercent.getArg(i) / 100,
			rampUpdateInterval: rampUpdateInterval.getArg(i),
			// determinism
			epochAnchor: anchor.UTC(),
			seed:        seedHash,
		}
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

func newScrapeConfig(targetsCount int, scrapeInterval time.Duration, targetAddr, labelName, jobName, metricRelabel string, requiresK8sAuth bool) *scrapeConfig {
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
	var hc *httpConfig
	if requiresK8sAuth {
		hc = &httpConfig{
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
		}
	}
	return &scrapeConfig{
		JobName:              jobName,
		ScrapeInterval:       scrapeInterval,
		HTTPConfig:           hc,
		StaticConfigs:        scs,
		MetricRelabelConfigs: mrc,
	}
}

type target struct {
	config *scrapeConfig
	// revision pool size limit
	revisionPoolSize int64
	// steady state (phase 2)
	steadyUpdatePercent  float64 // 0..1
	steadyUpdateInterval time.Duration

	// ramp-up (phase 1)
	rampDuration       time.Duration
	rampUpdatePercent  float64 // 0..1
	rampUpdateInterval time.Duration

	// determinism
	epochAnchor time.Time
	seed        uint64

	mu sync.Mutex
}

// epochCounts computes how many intervals have elapsed in ramp and steady phases
// since the epochAnchor, based on the current wall-clock time.
func (t *target) epochCounts(now time.Time) (rampEpochs int64, steadyEpochs int64) {
	now = now.UTC()
	anchor := t.epochAnchor
	var rampEnd time.Time
	if t.rampDuration > 0 && t.rampUpdateInterval > 0 {
		rampEnd = anchor.Add(t.rampDuration)
		if now.After(anchor) {
			rampElapsed := now.Sub(anchor)
			// cap to ramp duration
			if now.After(rampEnd) {
				rampElapsed = t.rampDuration
			}
			rampEpochs = int64(rampElapsed / t.rampUpdateInterval)
		}
	}
	// steady phase begins after rampEnd (or immediately if no ramp)
	startSteady := anchor
	if !rampEnd.IsZero() {
		startSteady = rampEnd
	}
	if t.steadyUpdateInterval > 0 && now.After(startSteady) {
		steadyElapsed := now.Sub(startSteady)
		steadyEpochs = int64(steadyElapsed / t.steadyUpdateInterval)
	}
	return
}

// revisionForIndex deterministically computes the revision counter for a given staticConfig index.
// It sums "successful updates" over all elapsed epochs in ramp and steady phases using a stable hash.
func (t *target) revisionForIndex(idx int, now time.Time) int64 {
	rampEpochs, steadyEpochs := t.epochCounts(now)
	var rev int64 = 0

	// helper to turn a probability into a selection via hashing
	choose := func(phase byte, epoch int64, percent float64) bool {
		if percent <= 0 {
			return false
		}
		// build a stable key: seed | phase | epoch | index
		buf := make([]byte, 8+1+8+8)
		binary.LittleEndian.PutUint64(buf[0:8], t.seed)
		buf[8] = phase
		binary.LittleEndian.PutUint64(buf[9:17], uint64(epoch))
		binary.LittleEndian.PutUint64(buf[17:25], uint64(idx))
		h := fnvHash64(buf)
		// map to [0,1)
		const scale = 1.0 / (1 << 53)
		// use top 53 bits to preserve uniformity when converted to float64
		x := float64(h>>11) * scale
		return x < percent
	}

	// accumulate ramp epochs
	for e := int64(0); e < rampEpochs; e++ {
		if choose('r', e, t.rampUpdatePercent) {
			rev++
		}
	}
	// accumulate steady epochs
	for e := int64(0); e < steadyEpochs; e++ {
		if choose('s', e, t.steadyUpdatePercent) {
			rev++
		}
	}
	return rev
}

func (t *target) marshal() *yaml.Node {
	n := &yaml.Node{}
	t.mu.Lock()
	defer t.mu.Unlock()

	// Compute deterministic revision labels for each staticConfig target
	now := time.Now().UTC()
	for i, sc := range t.config.StaticConfigs {
		rev := t.revisionForIndex(i, now)
		if sc.Labels == nil {
			sc.Labels = make(map[string]string)
		}
		if t.revisionPoolSize > 0 {
			rev = rev % t.revisionPoolSize
		}
		sc.Labels["revision"] = fmt.Sprintf("r%d", rev)
	}

	if err := n.Encode(t.config); err != nil {
		log.Fatalf("BUG: unexpected error when marshaling scrape config: %s", err)
	}
	return n
}

// config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type config struct {
	ScrapeConfigs []*yaml.Node `yaml:"scrape_configs"`
}

// scrapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type scrapeConfig struct {
	JobName              string                 `yaml:"job_name"`
	ScrapeInterval       time.Duration          `yaml:"scrape_interval"`
	HTTPConfig           *httpConfig            `yaml:",inline"`
	StaticConfigs        []*staticConfig        `yaml:"static_configs"`
	MetricRelabelConfigs []*metricRelabelConfig `yaml:"metric_relabel_configs,omitempty"`
}

// httpConfig represents HTTP configuration for scrape, such as auth params
type httpConfig struct {
	BearerTokenFile string `yaml:"bearer_token_file"`
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
