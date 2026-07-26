package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type bridgeConfig struct {
	Port             int
	WeComToken       string
	WeComAESKey      string
	WeComReceiveID   string
	BridgeToken      string
	MessageBufferCap int
}

type sseEvent struct {
	ID      int64
	Payload []byte
}

type sseClient struct {
	ch chan sseEvent
}

type bridgeState struct {
	mu          sync.Mutex
	nextEventID int64
	buffer      []sseEvent
	bufferCap   int
	clients     map[*sseClient]struct{}
}

type wecomXML struct {
	XMLName      xml.Name `xml:"xml"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
	Content      string   `xml:"Content"`
	FromUserName string   `xml:"FromUserName"`
	ToUserName   string   `xml:"ToUserName"`
	AgentID      string   `xml:"AgentID"`
	AgentId      string   `xml:"AgentId"`
	MsgId        string   `xml:"MsgId"`
	MsgID        string   `xml:"MsgID"`
	MediaId      string   `xml:"MediaId"`
	PicUrl       string   `xml:"PicUrl"`
	Encrypt      string   `xml:"Encrypt"`
}

type wecomMessage struct {
	MsgType  string
	Event    string
	EventKey string
	Content  string
	FromUser string
	ToUser   string
	AgentID  string
	MsgID    string
	MediaID  string
	PicURL   string
}

const (
	defaultPort             = 8080
	defaultBufferSize       = 200
	maxBodyBytes      int64 = 10 * 1024 * 1024
)

func main() {
	cfg := loadConfig()
	state := &bridgeState{
		nextEventID: 1,
		bufferCap:   cfg.MessageBufferCap,
		clients:     make(map[*sseClient]struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		handleStream(w, r, cfg, state)
	})
	mux.HandleFunc("/wecom", func(w http.ResponseWriter, r *http.Request) {
		handleWeCom(w, r, cfg, state)
	})
	mux.HandleFunc("/proxy/gettoken", func(w http.ResponseWriter, r *http.Request) {
		handleProxyGetToken(w, r, cfg)
	})
	mux.HandleFunc("/proxy/send", func(w http.ResponseWriter, r *http.Request) {
		handleProxySend(w, r, cfg)
	})
	mux.HandleFunc("/proxy/appchat/send", func(w http.ResponseWriter, r *http.Request) {
		handleProxyAppChatSend(w, r, cfg)
	})
	mux.HandleFunc("/proxy/menu/get", func(w http.ResponseWriter, r *http.Request) {
		handleProxyMenuGet(w, r, cfg)
	})
	mux.HandleFunc("/proxy/menu/create", func(w http.ResponseWriter, r *http.Request) {
		handleProxyMenuCreate(w, r, cfg)
	})
	mux.HandleFunc("/proxy/menu/delete", func(w http.ResponseWriter, r *http.Request) {
		handleProxyMenuDelete(w, r, cfg)
	})
	mux.HandleFunc("/proxy/media/upload", func(w http.ResponseWriter, r *http.Request) {
		handleProxyUpload(w, r, cfg)
	})
	mux.HandleFunc("/proxy/media/get", func(w http.ResponseWriter, r *http.Request) {
		handleProxyMediaGet(w, r, cfg)
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           loggingMiddleware(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("wecom-bridge listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func loadConfig() bridgeConfig {
	port := getenvInt("PORT", defaultPort)
	bufferCap := getenvInt("BRIDGE_BUFFER_SIZE", defaultBufferSize)
	if bufferCap <= 0 {
		bufferCap = defaultBufferSize
	}
	return bridgeConfig{
		Port:             port,
		WeComToken:       strings.TrimSpace(os.Getenv("WECOM_TOKEN")),
		WeComAESKey:      strings.TrimSpace(os.Getenv("WECOM_AES_KEY")),
		WeComReceiveID:   strings.TrimSpace(os.Getenv("WECOM_RECEIVE_ID")),
		BridgeToken:      strings.TrimSpace(os.Getenv("WECOM_BRIDGE_TOKEN")),
		MessageBufferCap: bufferCap,
	}
}

func getenvInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func handleStream(w http.ResponseWriter, r *http.Request, cfg bridgeConfig, state *bridgeState) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if cfg.BridgeToken != "" {
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", cfg.BridgeToken) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("stream unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = w.Write([]byte("\n"))
	flusher.Flush()

	lastEventID := parseLastEventID(r)
	if lastEventID > 0 {
		missed := state.getMissed(lastEventID)
		for _, ev := range missed {
			if err := writeSSE(w, ev); err != nil {
				return
			}
			flusher.Flush()
		}
		log.Printf("wecom stream replay %d messages since %d", len(missed), lastEventID)
	}

	client := &sseClient{ch: make(chan sseEvent, 16)}
	state.addClient(client)
	defer state.removeClient(client)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-client.ch:
			if err := writeSSE(w, ev); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func parseLastEventID(r *http.Request) int64 {
	if v := strings.TrimSpace(r.Header.Get("Last-Event-ID")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			return id
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("lastEventId")); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			return id
		}
	}
	return 0
}

func writeSSE(w io.Writer, ev sseEvent) error {
	if _, err := fmt.Fprintf(w, "id: %d\n", ev.ID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: message\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", ev.Payload); err != nil {
		return err
	}
	return nil
}

func handleWeCom(w http.ResponseWriter, r *http.Request, cfg bridgeConfig, state *bridgeState) {
	switch r.Method {
	case http.MethodGet:
		handleWeComVerify(w, r, cfg)
	case http.MethodPost:
		handleWeComPost(w, r, cfg, state)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleWeComVerify(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	q := r.URL.Query()
	signature := firstNonEmpty(q.Get("msg_signature"), q.Get("signature"))
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if cfg.WeComToken == "" || cfg.WeComAESKey == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("missing token or aes key"))
		return
	}
	if echostr == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing echostr"))
		return
	}

	expected := sha1Hex(sortedJoin([]string{cfg.WeComToken, timestamp, nonce, echostr}))
	if signature == "" || signature != expected {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid signature"))
		return
	}

	plain, ok := decryptWeCom(echostr, cfg.WeComAESKey, cfg.WeComReceiveID)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("decrypt failed"))
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(plain))
}

func handleWeComPost(w http.ResponseWriter, r *http.Request, cfg bridgeConfig, state *bridgeState) {
	q := r.URL.Query()
	signature := firstNonEmpty(q.Get("msg_signature"), q.Get("signature"))
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")

	if cfg.WeComToken == "" || cfg.WeComAESKey == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("missing token or aes key"))
		return
	}

	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return
	}

	encrypted := extractEncrypted(body)
	if encrypted == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing encrypt"))
		return
	}

	expected := sha1Hex(sortedJoin([]string{cfg.WeComToken, timestamp, nonce, encrypted}))
	if signature == "" || signature != expected {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid signature"))
		return
	}

	plain, ok := decryptWeCom(encrypted, cfg.WeComAESKey, cfg.WeComReceiveID)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("decrypt failed"))
		return
	}

	msg := parseWeComMessage(plain)
	if msg == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
		return
	}

	payload := map[string]any{
		"messageId":  firstNonEmpty(msg.MsgID, fmt.Sprintf("%s-%d", msg.FromUser, time.Now().UnixMilli())),
		"sessionId":  msg.FromUser,
		"fromUser":   msg.FromUser,
		"toUser":     msg.ToUser,
		"text":       msg.Content,
		"msgType":    msg.MsgType,
		"event":      msg.Event,
		"eventKey":   msg.EventKey,
		"agentId":    msg.AgentID,
		"mediaId":    msg.MediaID,
		"picUrl":     msg.PicURL,
		"receivedAt": time.Now().UTC().Format(time.RFC3339),
	}

	state.broadcast(payload)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

func extractEncrypted(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			return ""
		}
		if v, ok := obj["encrypt"]; ok {
			return fmt.Sprintf("%v", v)
		}
		if v, ok := obj["Encrypt"]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	var doc wecomXML
	if err := xml.Unmarshal(body, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Encrypt)
}

func parseWeComMessage(xmlText string) *wecomMessage {
	var doc wecomXML
	if err := xml.Unmarshal([]byte(xmlText), &doc); err != nil {
		return nil
	}
	msgType := strings.TrimSpace(doc.MsgType)
	fromUser := strings.TrimSpace(doc.FromUserName)
	if msgType == "" || fromUser == "" {
		return nil
	}
	msgID := strings.TrimSpace(doc.MsgId)
	if msgID == "" {
		msgID = strings.TrimSpace(doc.MsgID)
	}
	return &wecomMessage{
		MsgType:  msgType,
		Event:    strings.TrimSpace(doc.Event),
		EventKey: strings.TrimSpace(doc.EventKey),
		Content:  strings.TrimSpace(doc.Content),
		FromUser: fromUser,
		ToUser:   strings.TrimSpace(doc.ToUserName),
		AgentID:  firstNonEmpty(strings.TrimSpace(doc.AgentID), strings.TrimSpace(doc.AgentId)),
		MsgID:    msgID,
		MediaID:  strings.TrimSpace(doc.MediaId),
		PicURL:   strings.TrimSpace(doc.PicUrl),
	}
}

func decryptWeCom(encrypted, aesKey, receiveID string) (string, bool) {
	key, err := base64.StdEncoding.DecodeString(aesKey + "=")
	if err != nil || len(key) != 32 {
		return "", false
	}
	cipherText, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", false
	}
	if len(cipherText)%aes.BlockSize != 0 {
		return "", false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false
	}
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherText))
	mode.CryptBlocks(plain, cipherText)
	plain = pkcs7Unpad(plain)
	if len(plain) < 20 {
		return "", false
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	msgStart := 20
	msgEnd := msgStart + int(msgLen)
	if msgEnd > len(plain) {
		return "", false
	}
	msg := string(plain[msgStart:msgEnd])
	rid := string(plain[msgEnd:])
	if receiveID != "" && rid != receiveID {
		return "", false
	}
	return msg, true
}

func pkcs7Unpad(buf []byte) []byte {
	if len(buf) == 0 {
		return buf
	}
	pad := int(buf[len(buf)-1])
	if pad < 1 || pad > 32 || pad > len(buf) {
		return buf
	}
	return buf[:len(buf)-pad]
}

func sha1Hex(input string) string {
	h := sha1.Sum([]byte(input))
	return fmt.Sprintf("%x", h)
}

func sortedJoin(parts []string) string {
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	sort.Strings(filtered)
	return strings.Join(filtered, "")
}
