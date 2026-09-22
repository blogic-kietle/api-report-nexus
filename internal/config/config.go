package config

import (
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	AppEnv   string     `env:"APP_ENV" envDefault:"development"`
	Debug    bool       `env:"DEBUG"`
	Port     int        `env:"PORT" envDefault:"8000"`
	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`

	// 20MB, like express.json
	MaxJSONBody       int64         `env:"MAX_JSON_BODY" envDefault:"20971520"`
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"10s"`
	IdleTimeout       time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"120s"`
	ShutdownTimeout   time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	// BLogic View, where receipt links in the sales summary point.
	ClientURL string `env:"BLOGIC_VIEW_CLIENT_URL" envDefault:"https://blogicview.com"`

	LicenseAPIURL     string        `env:"LICENSE_API_URL"`
	LicenseAPITimeout time.Duration `env:"LICENSE_API_TIMEOUT" envDefault:"60s"`

	RabbitMQHost       string        `env:"RABBITMQ_HOST"`
	RabbitMQPort       int           `env:"RABBITMQ_PORT" envDefault:"5671"`
	RabbitMQUser       string        `env:"RABBITMQ_USER"`
	RabbitMQPass       string        `env:"RABBITMQ_PASS"`
	RabbitMQVHost      string        `env:"RABBITMQ_VHOST" envDefault:"/"`
	RabbitMQCertPath   string        `env:"RABBITMQ_CERT_PATH"`
	RabbitMQCertPass   string        `env:"RABBITMQ_CERT_PASS"`
	RabbitMQCAPath     string        `env:"RABBITMQ_CA_PATH"`
	RabbitMQServerName string        `env:"RABBITMQ_SERVER_NAME"`
	RabbitMQTimeout    time.Duration `env:"RABBITMQ_PUBLISH_TIMEOUT" envDefault:"10s"`

	// Gotenberg's --api-timeout must stay at or above PDFTimeout.
	GotenbergURL    string        `env:"GOTENBERG_URL" envDefault:"http://gotenberg:3000"`
	PDFTimeout      time.Duration `env:"PDF_RENDER_TIMEOUT" envDefault:"5m"`
	MaxChunkSize    int64         `env:"MAX_CHUNK_SIZE" envDefault:"5242880"`
	PDFChunkSize    int           `env:"PDF_CHUNK_SIZE" envDefault:"1000"`
	StreamTTL       time.Duration `env:"PDF_STREAM_SESSION_TTL" envDefault:"30m"`
	FinalizeTimeout time.Duration `env:"PDF_STREAM_FINALIZE_TIMEOUT" envDefault:"30m"`
}

func Load() (Config, error) {
	var c Config
	err := env.Parse(&c)
	return c, err
}
