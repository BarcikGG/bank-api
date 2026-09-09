package config

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env         string `mapstructure:"env"`
	DatabaseURL string `mapstructure:"database_url"`

	KafkaBrokers       string `mapstructure:"kafka_brokers"`
	KafkaTopic         string `mapstructure:"kafka_topic"`
	KafkaConsumerGroup string `mapstructure:"kafka_consumer_group"`

	NotificationBatchSize    int           `mapstructure:"notification_batch_size"`
	NotificationPollInterval time.Duration `mapstructure:"notification_poll_interval"`
	NotificationLease        time.Duration `mapstructure:"notification_lease"`
	NotificationMaxAttempts  int           `mapstructure:"notification_max_attempts"`
	DeliveryTimeout          time.Duration `mapstructure:"delivery_timeout"`
}

func MustLoadConsumer() *Config {
	return mustLoad(Config.validateConsumer)
}

func MustLoadWorker() *Config {
	return mustLoad(Config.validateWorker)
}

func mustLoad(validate func(Config) error) *Config {
	loader := viper.New()
	loader.SetConfigFile(".env")
	loader.SetDefault("notification_batch_size", 10)
	loader.SetDefault("notification_poll_interval", 500*time.Millisecond)
	loader.SetDefault("notification_lease", 120*time.Second)
	loader.SetDefault("notification_max_attempts", 5)
	loader.SetDefault("delivery_timeout", 10*time.Second)
	loader.AutomaticEnv()

	if err := bindEnvironmentVariables(loader); err != nil {
		log.Fatalf("bind environment: %v", err)
	}

	if err := loader.ReadInConfig(); err != nil {
		log.Printf("Предупреждение: .env файл не найден (%v). Читаем системный env", err)
	}

	var cfg Config
	if err := loader.Unmarshal(&cfg); err != nil {
		log.Fatalf("decode config: %v", err)
	}

	if err := validate(cfg); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	return &cfg
}

func bindEnvironmentVariables(loader *viper.Viper) error {
	keys := []string{
		"env",
		"database_url",
		"kafka_brokers",
		"kafka_topic",
		"kafka_consumer_group",
		"notification_batch_size",
		"notification_poll_interval",
		"notification_lease",
		"notification_max_attempts",
		"delivery_timeout",
	}

	for _, key := range keys {
		if err := loader.BindEnv(key); err != nil {
			return fmt.Errorf("bind %s: %w", key, err)
		}
	}

	return nil
}

func (c Config) validateDatabase() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("database URL is empty")
	}
	return nil
}

func (c Config) validateConsumer() error {
	if err := c.validateDatabase(); err != nil {
		return err
	}
	if strings.TrimSpace(c.KafkaBrokers) == "" {
		return errors.New("Kafka brokers are empty")
	}
	if strings.TrimSpace(c.KafkaTopic) == "" {
		return errors.New("Kafka topic is empty")
	}
	if strings.TrimSpace(c.KafkaConsumerGroup) == "" {
		return errors.New("Kafka consumer group is empty")
	}
	return nil
}

func (c Config) validateWorker() error {
	if err := c.validateDatabase(); err != nil {
		return err
	}
	if c.NotificationBatchSize <= 0 {
		return errors.New("notification batch size must be positive")
	}
	if c.NotificationPollInterval <= 0 {
		return errors.New("notification poll interval must be positive")
	}
	if c.NotificationLease <= 0 {
		return errors.New("notification lease must be positive")
	}
	if c.NotificationMaxAttempts <= 0 {
		return errors.New("notification max attempts must be positive")
	}
	if c.DeliveryTimeout <= 0 {
		return errors.New("delivery timeout must be positive")
	}

	worstCaseBatchTime := time.Duration(c.NotificationBatchSize) * c.DeliveryTimeout
	if c.NotificationLease <= worstCaseBatchTime {
		return fmt.Errorf(
			"notification lease %s must exceed sequential batch delivery time %s",
			c.NotificationLease,
			worstCaseBatchTime,
		)
	}

	return nil
}
