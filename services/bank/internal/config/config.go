package config

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env string `mapstructure:"env"`

	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	Timeout     time.Duration `mapstructure:"timeout"`
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`

	DatabaseURL string `mapstructure:"database_url"`

	JWTSecret  string        `mapstructure:"jwt_secret"`
	JWTTTL     time.Duration `mapstructure:"jwt_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`

	KafkaBrokers       string        `mapstructure:"kafka_brokers"`
	OutboxBatchSize    int           `mapstructure:"outbox_batch_size"`
	OutboxPollInterval time.Duration `mapstructure:"outbox_poll_interval"`
	OutboxLease        time.Duration `mapstructure:"outbox_lease"`
}

func MustLoad() *Config {
	return mustLoad(Config.validateAPI)
}

func MustLoadRelay() *Config {
	return mustLoad(Config.validateRelay)
}

func mustLoad(validate func(Config) error) *Config {
	loader := viper.New()
	loader.SetConfigFile(".env")

	if err := loader.ReadInConfig(); err != nil {
		log.Printf("Предупреждение: .env файл не найден (%v). Читаем системный env.", err)
	}

	loader.AutomaticEnv()
	bindEnvironmentVariables(loader)

	var cfg Config
	if err := loader.Unmarshal(&cfg); err != nil {
		log.Fatalf("Не удалось прочитать конфиг: %v", err)
	}

	if err := validate(cfg); err != nil {
		log.Fatalf("Некорректная конфигурация: %v", err)
	}

	return &cfg
}

func bindEnvironmentVariables(loader *viper.Viper) {
	keys := []string{
		"env",
		"host",
		"port",
		"timeout",
		"idle_timeout",
		"database_url",
		"jwt_secret",
		"jwt_ttl",
		"refresh_ttl",
		"kafka_brokers",
		"outbox_batch_size",
		"outbox_poll_interval",
		"outbox_lease",
	}

	for _, key := range keys {
		if err := loader.BindEnv(key); err != nil {
			log.Fatalf("Не удалось привязать env %s: %v", key, err)
		}
	}
}

func (c Config) validateAPI() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("database URL is empty")
	}

	if strings.TrimSpace(c.JWTSecret) == "" {
		return errors.New("jwt is empty!")
	}

	if len([]byte(c.JWTSecret)) < 32 {
		return errors.New("jwt secret must be longer 32 bytes")
	}

	if c.JWTTTL <= 0 {
		return errors.New("jwt ttl must be positive")
	}

	if c.RefreshTTL <= 0 {
		return errors.New("refresh ttl must be positive")
	}

	if c.RefreshTTL <= c.JWTTTL {
		return errors.New("refresh ttl must be longer than jwt ttl")
	}

	return nil
}

func (c Config) validateRelay() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("database URL is empty")
	}

	if strings.TrimSpace(c.KafkaBrokers) == "" {
		return errors.New("Kafka brokers are empty")
	}

	if c.OutboxBatchSize <= 0 {
		return errors.New("outbox batch size must be positive")
	}

	if c.OutboxPollInterval <= 0 {
		return errors.New("outbox poll interval must be positive")
	}

	if c.OutboxLease <= 0 {
		return errors.New("outbox lease must be positive")
	}

	return nil
}
