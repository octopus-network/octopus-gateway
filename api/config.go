package main

import (
	"time"

	"github.com/spf13/viper"
)

type Api struct {
	Host            string        `default:"0.0.0.0"`
	Port            string        `default:"8080"`
	GracefulTimeout time.Duration `default:"30s"`
}

type Database struct {
	Driver            string `default:"postgres"`
	Host              string
	Port              string
	Database          string
	User              string
	Password          string
	SslMode           string `default:"disable"`
	MaxConnectionPool int    `default:"10"`
}

type Config struct {
	Api      Api
	Database Database
}

func NewConfig() *Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	// viper.OnConfigChange(func(e fsnotify.Event) {
	// 	fmt.Println("Config file changed:", e.Name)
	// })
	// viper.WatchConfig()

	config := Config{}
	err = viper.Unmarshal(&config)
	if err != nil {
		panic(err)
	}
	return &config
}
