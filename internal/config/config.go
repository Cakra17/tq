package config

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/joeshaw/envdecode"
	"github.com/joho/godotenv"
)

type DatabaseConfig struct {
	Name              string `env:"DB_NAME"`
	Host              string `env:"DB_HOST"`
	Port              string `env:"DB_PORT"`
	Username          string `env:"DB_USERNAME"`
	Password          string `env:"DB_PASSWORD"`
	MaxOpenConnection int    `env:"DB_MAX_OPEN_CONNECTION,default=100"`
	MaxIdleConnection int    `env:"DB_MAX_IDLE_CONNECTION,default=50"`
	MaxConnLifetime   int    `env:"DB_MAX_CONN_LIFETIME,default=30"`
	MaxConnIdleTime   int    `env:"DB_MAX_CONN_IDLE_TIME,default=1"`
}

type RedisConfig struct {
	Host string `env:"REDIS_HOST"`
	Password string `env:"REDIS_PASSWORD"`
	DB int `env:"REDIS_ENV"`
}

type Config struct {
	Database DatabaseConfig
	Redis RedisConfig
	AppPort  string `env:"APP_PORT,default=6969"`
}

func LoadEnv() Config {
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	var cfg Config
	if err := envdecode.Decode(&cfg); err != nil {
		log.Fatalf("Failed to decode env: %v", err)
	}
	log.Println("Env setup finished")
	return cfg
}

func ConnectDB(cdf DatabaseConfig) *sql.DB {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cdf.Username, cdf.Password, cdf.Host, cdf.Port, cdf.Name,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Error Opening Database: %v", err)
	}

	db.SetMaxOpenConns(cdf.MaxConnLifetime)
	db.SetMaxIdleConns(cdf.MaxConnIdleTime)
	db.SetConnMaxIdleTime(time.Duration(cdf.MaxIdleConnection) * time.Second)
	db.SetConnMaxLifetime(time.Duration(cdf.MaxConnLifetime) * time.Second)

	if err := db.Ping(); err != nil {
		log.Fatalf("Erorr Opening Database: %v", err)
	}
	log.Println("Connected to Postgres successfully")
	return db
}