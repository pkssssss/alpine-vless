package singbox

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"strings"
)

const (
	defaultSNI         = "dash.cloudflare.com"
	defaultHandshake   = "dash.cloudflare.com"
	defaultHandshakePt = 443
	defaultFlow        = "xtls-rprx-vision"
	defaultFP          = "chrome"
)

type Node struct {
	Port             int
	UUID             string
	SNI              string
	HandshakeHost    string
	HandshakePort    int
	Flow             string
	Fingerprint      string
	RealityPrivateKey string
	RealityShortID   string
}

type Config struct {
	Node Node
	Raw  []byte
}

func NewDefaultNode(_ context.Context) (Node, error) {
	port, err := randomFreePort()
	if err != nil {
		return Node{}, err
	}
	uuid, err := newUUIDv4()
	if err != nil {
		return Node{}, err
	}
	sid, err := newShortID()
	if err != nil {
		return Node{}, err
	}
	priv, _, err := newRealityKeyPair()
	if err != nil {
		return Node{}, err
	}

	return Node{
		Port:              port,
		UUID:              uuid,
		SNI:               defaultSNI,
		HandshakeHost:     defaultHandshake,
		HandshakePort:     defaultHandshakePt,
		Flow:              defaultFlow,
		Fingerprint:       defaultFP,
		RealityPrivateKey: priv,
		RealityShortID:    sid,
	}, nil
}

func (n Node) URL(ip, publicKey string) string {
	host := ip
	if host == "" {
		host = "your_ip"
	}

	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("security", "reality")
	q.Set("flow", n.Flow)
	q.Set("type", "tcp")
	q.Set("sni", n.SNI)
	q.Set("pbk", publicKey)
	q.Set("sid", n.RealityShortID)
	q.Set("fp", n.Fingerprint)

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(n.UUID),
		Host:     fmt.Sprintf("%s:%d", host, n.Port),
		RawQuery: q.Encode(),
		Fragment: fmt.Sprintf("alpine-reality-%s-%d", host, n.Port),
	}
	return u.String()
}

func WriteConfig(path, logPath string, node Node) error {
	cfg := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
			"output":    logPath,
		},
		"inbounds": []any{
			map[string]any{
				"type":        "vless",
				"tag":         "vless-reality",
				"listen":      "::",
				"listen_port": node.Port,
				"users": []any{
					map[string]any{
						"uuid": node.UUID,
						"flow": node.Flow,
					},
				},
				"tls": map[string]any{
					"enabled":     true,
					"server_name": node.SNI,
					"reality": map[string]any{
						"enabled": true,
						"handshake": map[string]any{
							"server":      node.HandshakeHost,
							"server_port": node.HandshakePort,
						},
						"private_key": node.RealityPrivateKey,
						"short_id":    []string{node.RealityShortID},
					},
				},
			},
		},
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "block"},
		},
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	if err := os.WriteFile(path, b, 0600); err != nil {
		return err
	}
	return nil
}

func ReadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	type rawCfg struct {
		Inbounds []struct {
			Type       string `json:"type"`
			ListenPort int    `json:"listen_port"`
			Users      []struct {
				UUID string `json:"uuid"`
				Flow string `json:"flow"`
			} `json:"users"`
			TLS struct {
				ServerName string `json:"server_name"`
				Reality    struct {
					PrivateKey string   `json:"private_key"`
					ShortID    []string `json:"short_id"`
				} `json:"reality"`
			} `json:"tls"`
		} `json:"inbounds"`
	}

	var rc rawCfg
	if err := json.Unmarshal(b, &rc); err != nil {
		return Config{}, err
	}
	if len(rc.Inbounds) < 1 {
		return Config{}, errors.New("配置文件缺少 inbounds")
	}
	inb := rc.Inbounds[0]
	if len(inb.Users) < 1 {
		return Config{}, errors.New("配置文件缺少 users")
	}
	if len(inb.TLS.Reality.ShortID) < 1 {
		return Config{}, errors.New("配置文件缺少 reality.short_id")
	}

	node := Node{
		Port:              inb.ListenPort,
		UUID:              inb.Users[0].UUID,
		SNI:               inb.TLS.ServerName,
		HandshakeHost:     inb.TLS.ServerName,
		HandshakePort:     defaultHandshakePt,
		Flow:              inb.Users[0].Flow,
		Fingerprint:       defaultFP,
		RealityPrivateKey: inb.TLS.Reality.PrivateKey,
		RealityShortID:    inb.TLS.Reality.ShortID[0],
	}

	return Config{Node: node, Raw: b}, nil
}

func RealityPublicKeyFromPrivateKey(privateKey string) (string, error) {
	raw, err := decodeBase64Flexible(strings.TrimSpace(privateKey))
	if err != nil {
		return "", err
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("Reality private key 长度异常: %d", len(raw))
	}

	curve := ecdh.X25519()
	priv, err := curve.NewPrivateKey(raw)
	if err != nil {
		return "", err
	}
	pub := priv.PublicKey().Bytes()
	return base64.RawURLEncoding.EncodeToString(pub), nil
}

func decodeBase64Flexible(s string) ([]byte, error) {
	decs := []*base64.Encoding{
		base64.RawURLEncoding,
		base64.URLEncoding,
		base64.RawStdEncoding,
		base64.StdEncoding,
	}
	var lastErr error
	for _, enc := range decs {
		b, err := enc.DecodeString(s)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("无法解析 base64: %w", lastErr)
}

func newRealityKeyPair() (privateKey, publicKey string, err error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	pub := priv.PublicKey()
	return base64.RawURLEncoding.EncodeToString(priv.Bytes()),
		base64.RawURLEncoding.EncodeToString(pub.Bytes()),
		nil
}

func newShortID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]),
	), nil
}

func randomFreePort() (int, error) {
	const (
		min = 20000
		max = 60000
	)
	for i := 0; i < 100; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
		if err != nil {
			return 0, err
		}
		p := min + int(n.Int64())

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return p, nil
	}
	return 0, errors.New("无法找到空闲端口")
}
