package ekausage

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type OnError func(err error, ctx map[string]any)

type Config struct {
	ServiceName  string
	KafkaBrokers string
	KafkaConfig  map[string]any
	OnError      OnError
	Debug        bool
	BufferSize   int
	Producer     Producer
}

type Option func(*Config)

func WithKafkaBrokers(b string) Option          { return func(c *Config) { c.KafkaBrokers = b } }
func WithKafkaConfig(kc map[string]any) Option   { return func(c *Config) { c.KafkaConfig = kc } }
func WithOnError(f OnError) Option               { return func(c *Config) { c.OnError = f } }
func WithDebug(b bool) Option                    { return func(c *Config) { c.Debug = b } }
func WithBufferSize(n int) Option                { return func(c *Config) { c.BufferSize = n } }
func WithProducer(p Producer) Option             { return func(c *Config) { c.Producer = p } }

type queuedMsg struct {
	topic string
	key   []byte
	value []byte
}

type Client struct {
	serviceName string
	hostname    string
	onError     OnError
	debug       bool

	producer Producer
	ownsProd bool

	buf     chan queuedMsg
	closed  atomic.Bool
	wg      sync.WaitGroup
	closeMu sync.Mutex
}

func New(serviceName string, opts ...Option) (*Client, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("serviceName required")
	}

	cfg := &Config{
		ServiceName: serviceName,
		BufferSize:  10000,
	}
	for _, o := range opts {
		o(cfg)
	}

	host, _ := os.Hostname()
	c := &Client{
		serviceName: cfg.ServiceName,
		hostname:    host,
		onError:     cfg.OnError,
		debug:       cfg.Debug,
		buf:         make(chan queuedMsg, cfg.BufferSize),
	}

	if cfg.Producer != nil {
		c.producer = cfg.Producer
	} else {
		brokers := cfg.KafkaBrokers
		if brokers == "" {
			brokers = os.Getenv(EnvKafkaBrokers)
		}
		if brokers == "" {
			return nil, fmt.Errorf("kafka brokers not configured (pass WithKafkaBrokers or set %s)", EnvKafkaBrokers)
		}
		kafkaCfg := kafkaConfigFromEnv()
		for k, v := range cfg.KafkaConfig {
			kafkaCfg[k] = v
		}
		clientID := fmt.Sprintf("eka-usage-sdk-go/%s", cfg.ServiceName)
		cp, err := newConfluentProducer(brokers, kafkaCfg, clientID, c.handleErr)
		if err != nil {
			return nil, err
		}
		c.producer = cp
		c.ownsProd = true
	}

	c.wg.Add(1)
	go c.dispatchLoop()
	return c, nil
}

func (c *Client) Record(workspaceID, product, metricType string, quantity float64, status string, unitCost *float64, metadata map[string]any) {
	if workspaceID == "" {
		c.handleErr(&ValidationError{Msg: "workspaceID required"}, map[string]any{"product": product})
		return
	}
	if status == "" {
		status = "ok"
	}
	if quantity == 0 {
		quantity = 1.0
	}
	if err := validateUsage(product, metricType, quantity, status); err != nil {
		c.handleErr(err, map[string]any{"workspace_id": workspaceID, "product": product, "metric_type": metricType})
		return
	}
	metaJSON, _ := safeMarshal(metadata)
	isBillable := 0
	if status == "ok" {
		isBillable = 1
	}
	evt := map[string]any{
		"workspace_id": workspaceID,
		"service_name": c.serviceName,
		"product":      product,
		"metric_type":  metricType,
		"quantity":     quantity,
		"unit_cost":    unitCost,
		"status":       status,
		"is_billable":  isBillable,
		"metadata":     metaJSON,
		"sdk_language": SDKLanguage,
		"sdk_version":  SDKVersion,
		"hostname":     c.hostname,
		"ts":           nowISO(),
	}
	c.enqueue(UsageTopic, workspaceID, evt)
}

func (c *Client) enqueue(topic, partitionKey string, evt map[string]any) {
	if c.closed.Load() {
		c.handleErr(ErrClosed, map[string]any{"topic": topic})
		return
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		c.handleErr(err, map[string]any{"phase": "marshal"})
		return
	}
	msg := queuedMsg{topic: topic, key: []byte(partitionKey), value: payload}
	select {
	case c.buf <- msg:
	default:
		c.handleErr(ErrBufferFull, map[string]any{"topic": topic, "reason": "local_queue_full"})
	}
}

func (c *Client) dispatchLoop() {
	defer c.wg.Done()
	for m := range c.buf {
		if err := c.producer.Produce(m.topic, m.key, m.value); err != nil {
			c.handleErr(err, map[string]any{"topic": m.topic, "phase": "produce"})
		}
	}
}

func (c *Client) handleErr(err error, ctx map[string]any) {
	if c.debug {
		fmt.Fprintf(os.Stderr, "[eka-usage-sdk] %v ctx=%v\n", err, ctx)
	}
	if c.onError != nil {
		defer func() { _ = recover() }()
		c.onError(err, ctx)
	}
}

func (c *Client) Shutdown() {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed.Swap(true) {
		return
	}
	close(c.buf)
	c.wg.Wait()
	c.producer.Flush(10000)
	if c.ownsProd {
		c.producer.Close()
	}
}

func kafkaConfigFromEnv() map[string]any {
	cfg := map[string]any{
		"linger.ms":        envInt(EnvKafkaLingerMs, DefaultLingerMs),
		"batch.size":       envInt(EnvKafkaBatchSize, DefaultBatchSize),
		"compression.type": envStr(EnvKafkaCompressionType, DefaultCompressionType),
		"acks":             envStr(EnvKafkaAcks, DefaultAcks),
		"retries":          envInt(EnvKafkaRetries, DefaultRetries),
	}
	return cfg
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func safeMarshal(v map[string]any) (string, error) {
	if v == nil {
		return "{}", nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}
