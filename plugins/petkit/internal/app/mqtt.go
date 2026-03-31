package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttKeepAlive      = 60 * time.Second
	mqttReconnectDelay = 30 * time.Second
	mqttConnectTimeout = 15 * time.Second
)

var schemeRe = regexp.MustCompile(`(?i)^(?:tcp|ssl|mqtt|mqtts)://`)
var hostPortRe = regexp.MustCompile(`^(?P<host>.+?)(?::(?P<port>\d+))?$`)

// iotMQTTConfig holds the credentials returned by the Petkit IoT endpoint.
type iotMQTTConfig struct {
	MQTTHost     string
	ProductKey   string
	DeviceName   string
	DeviceSecret string
}

// mqttListener manages the Petkit Aliyun IoT MQTT connection for one account.
type mqttListener struct {
	mu          sync.Mutex
	cfg         iotMQTTConfig
	client      mqtt.Client
	onMessage   func()
	started     bool
	connected   bool
	stopCh      chan struct{}
	accountName string
}

func newMQTTListener(cfg iotMQTTConfig, accountName string, onMessage func()) *mqttListener {
	return &mqttListener{
		cfg:         cfg,
		onMessage:   onMessage,
		stopCh:      make(chan struct{}),
		accountName: accountName,
	}
}

// aliyunSign computes Aliyun IoT MQTT credentials.
// clientId: {deviceName}|securemode=3,signmethod=hmacsha256|
// username:  {deviceName}&{productKey}
// password:  HMAC-SHA256(deviceSecret, "clientId{cid}deviceName{dn}productKey{pk}")
func aliyunSign(productKey, deviceName, deviceSecret string) (clientID, username, password string) {
	cid := deviceName
	content := fmt.Sprintf("clientId%sdeviceName%sproductKey%s", cid, deviceName, productKey)
	mac := hmac.New(sha256.New, []byte(deviceSecret))
	mac.Write([]byte(content))
	sig := hex.EncodeToString(mac.Sum(nil))
	clientID = fmt.Sprintf("%s|securemode=3,signmethod=hmacsha256|", cid)
	username = fmt.Sprintf("%s&%s", deviceName, productKey)
	password = sig
	return
}

func parseMQTTHost(raw string) (host string, port int, err error) {
	raw = strings.TrimSpace(raw)
	raw = schemeRe.ReplaceAllString(raw, "")
	m := hostPortRe.FindStringSubmatch(raw)
	if m == nil {
		return "", 0, fmt.Errorf("invalid mqtt host: %q", raw)
	}
	host = m[1]
	port = 1883
	if m[2] != "" {
		port, err = strconv.Atoi(m[2])
		if err != nil {
			return "", 0, fmt.Errorf("invalid mqtt port: %q", m[2])
		}
	}
	return host, port, nil
}

func (l *mqttListener) start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		return nil
	}

	host, port, err := parseMQTTHost(l.cfg.MQTTHost)
	if err != nil {
		return err
	}

	clientID, username, password := aliyunSign(l.cfg.ProductKey, l.cfg.DeviceName, l.cfg.DeviceSecret)
	broker := fmt.Sprintf("tcp://%s:%d", host, port)

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetUsername(username).
		SetPassword(password).
		SetCleanSession(false).
		SetKeepAlive(mqttKeepAlive).
		SetConnectTimeout(mqttConnectTimeout).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(mqttReconnectDelay).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			l.mu.Lock()
			l.connected = false
			l.mu.Unlock()
		}).
		SetOnConnectHandler(func(c mqtt.Client) {
			l.mu.Lock()
			l.connected = true
			topic := fmt.Sprintf("/%s/%s/user/get", l.cfg.ProductKey, l.cfg.DeviceName)
			l.mu.Unlock()
			c.Subscribe(topic, 0, func(_ mqtt.Client, msg mqtt.Message) {
				l.handleMessage(msg.Payload())
			})
		})

	// Will message signals offline status
	willTopic := fmt.Sprintf("/%s/%s/user/update", l.cfg.ProductKey, l.cfg.DeviceName)
	opts.SetWill(willTopic, `{"status":"offline"}`, 0, false)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.WaitTimeout(mqttConnectTimeout)
	if err := token.Error(); err != nil {
		return fmt.Errorf("petkit mqtt connect: %w", err)
	}

	l.client = client
	l.started = true
	return nil
}

func (l *mqttListener) stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.started {
		return
	}
	if l.client != nil {
		l.client.Disconnect(500)
		l.client = nil
	}
	l.started = false
	l.connected = false
}

func (l *mqttListener) isConnected() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.connected
}

func (l *mqttListener) handleMessage(payload []byte) {
	// The MQTT message is a trigger signal; we don't need to parse the full
	// payload — just fire the refresh callback so the REST API is queried.
	// Log the inner type for diagnostics if parseable.
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err == nil {
		_ = msg // available for future structured handling
	}
	if l.onMessage != nil {
		l.onMessage()
	}
}

// fetchIoTMQTTConfig calls the Petkit REST API to retrieve MQTT broker credentials.
func (c *Client) fetchIoTMQTTConfig(ctx context.Context) (iotMQTTConfig, error) {
	resp, err := c.getSessionJSON(ctx, "iot/getIotMqttConfig", url.Values{"clientType": []string{"2"}})
	if err != nil {
		return iotMQTTConfig{}, fmt.Errorf("petkit iot mqtt config: %w", err)
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return iotMQTTConfig{}, fmt.Errorf("petkit iot mqtt config: unexpected response type")
	}
	cfg := iotMQTTConfig{
		MQTTHost:     stringFromAny(m["mqttHost"], ""),
		ProductKey:   stringFromAny(m["productKey"], ""),
		DeviceName:   stringFromAny(m["deviceName"], ""),
		DeviceSecret: stringFromAny(m["deviceSecret"], ""),
	}
	if cfg.MQTTHost == "" || cfg.ProductKey == "" || cfg.DeviceName == "" || cfg.DeviceSecret == "" {
		return iotMQTTConfig{}, fmt.Errorf("petkit iot mqtt config: missing required fields in response")
	}
	return cfg, nil
}
