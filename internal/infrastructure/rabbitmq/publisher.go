// Package rabbitmq publishes the platform events the Node service emitted, over
// AMQPS with a client certificate.
package rabbitmq

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"software.sslmate.com/src/go-pkcs12"
)

// Topology as Node declared it; a durable exchange of the same name is refused with 406.
const (
	exchange   = "blogic_license"
	queue      = "stores-delivery-report-update"
	routingKey = "stores.delivery-report.update"
)

// Config is the broker address and the TLS material.
type Config struct {
	Host       string
	Port       int
	User, Pass string
	VHost      string
	// CertPath is the .p12 client identity; empty means plain AMQP, local brokers only.
	CertPath string
	CertPass string
	// CAPath is PEM; empty means the system roots.
	CAPath string
	// ServerName is the SNI when it differs from Host.
	ServerName string
	Timeout    time.Duration
	Log        *slog.Logger
}

// Publisher keeps one connection and channel, opened on first use and reopened after a failure.
type Publisher struct {
	cfg  Config
	url  string
	amqp amqp.Config

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

// New loads the certificates now, so a bad path fails at boot rather than on the first statement.
func New(cfg Config) (*Publisher, error) {
	p := &Publisher{cfg: cfg}
	p.amqp = amqp.Config{
		SASL:       []amqp.Authentication{&amqp.PlainAuth{Username: cfg.User, Password: cfg.Pass}},
		Vhost:      cfg.VHost,
		Heartbeat:  10 * time.Second,
		Locale:     "en_US",
		Dial:       amqp.DefaultDial(cfg.Timeout),
		Properties: amqp.NewConnectionProperties(),
	}
	scheme := "amqp"
	if cfg.CertPath != "" {
		tlsCfg, err := tlsConfig(cfg.CertPath, cfg.CertPass, cfg.CAPath, cfg.ServerName)
		if err != nil {
			return nil, err
		}
		p.amqp.TLSClientConfig = tlsCfg
		scheme = "amqps"
	}
	p.url = scheme + "://" + net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)) + "/"
	return p, nil
}

// Mutual TLS from the .p12 identity and the broker CA: Node's pfx/passphrase/ca/servername.
func tlsConfig(certPath, certPass, caPath, serverName string) (*tls.Config, error) {
	pfx, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: %w", err)
	}
	key, leaf, chain, err := pkcs12.DecodeChain(pfx, certPass)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: %s: %w", certPath, err)
	}
	cert := tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
	for _, c := range chain {
		cert.Certificate = append(cert.Certificate, c.Raw)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, ServerName: serverName, MinVersion: tls.VersionTLS12}
	if caPath != "" {
		pem, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("rabbitmq: %w", err)
		}
		cfg.RootCAs = x509.NewCertPool()
		if !cfg.RootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("rabbitmq: no PEM certificate in %s", caPath)
		}
	}
	return cfg, nil
}

// DeliveryReportUpdated tells the platform that a store's statement went out.
func (p *Publisher) DeliveryReportUpdated(ctx context.Context, storeID string) error {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()
	return p.publish(ctx, routingKey, deliveryReportMessage(storeID, time.Now()))
}

func deliveryReportMessage(storeID string, now time.Time) []byte {
	body, _ := json.Marshal(struct {
		StoreToken      string `json:"storeToken"`
		LastTimeTrigger string `json:"lastTimeTrigger"`
		// millisecond ISO-8601 in UTC, the shape JavaScript dates serialise to
	}{storeID, now.UTC().Format("2006-01-02T15:04:05.000Z07:00")})
	return body
}

// A connection the broker dropped since the last message fails at once: retried on a fresh one, once.
func (p *Publisher) publish(ctx context.Context, key string, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch, err := p.channel()
	if err != nil {
		return err
	}
	if err = confirmPublish(ctx, ch, key, body); err == nil {
		return nil
	}
	p.reset()
	if ctx.Err() != nil {
		return err
	}
	if ch, err = p.channel(); err != nil {
		return err
	}
	if err = confirmPublish(ctx, ch, key, body); err != nil {
		p.reset()
	}
	return err
}

func (p *Publisher) channel() (*amqp.Channel, error) {
	if p.ch != nil && !p.ch.IsClosed() && !p.conn.IsClosed() {
		return p.ch, nil
	}
	p.reset()
	conn, err := amqp.DialConfig(p.url, p.amqp)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: dial %s: %w", p.cfg.Host, err)
	}
	ch, err := conn.Channel()
	if err == nil {
		err = ch.ExchangeDeclare(exchange, amqp.ExchangeDirect, false, false, false, false, nil)
	}
	if err == nil {
		_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	}
	if err == nil {
		err = ch.QueueBind(queue, routingKey, exchange, false, nil)
	}
	if err == nil {
		err = ch.Confirm(false)
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq: setup: %w", err)
	}
	p.conn, p.ch = conn, ch
	if p.cfg.Log != nil {
		p.cfg.Log.Info("rabbitmq connected", "host", p.cfg.Host)
	}
	return ch, nil
}

func confirmPublish(ctx context.Context, ch *amqp.Channel, key string, body []byte) error {
	dc, err := ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		// ponytail: Node sent Date.now() (ms) in this seconds field; kept so consumers see no change.
		Timestamp: time.Unix(time.Now().UnixMilli(), 0),
		Body:      body,
	})
	if err != nil {
		return err
	}
	acked, err := dc.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return errors.New("rabbitmq: message not acknowledged")
	}
	return nil
}

func (p *Publisher) reset() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn, p.ch = nil, nil
}

// Close ends the connection at shutdown.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reset()
	return nil
}
