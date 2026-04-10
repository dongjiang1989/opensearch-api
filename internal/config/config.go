package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server     ServerConfig
	OpenSearch OpenSearchConfig
	Storage    StorageConfig
	JWT        JWTConfig
	Log        LogConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Host         string        `mapstructure:"host"`
	Mode         string        `mapstructure:"mode"` // debug, release, test
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// OpenSearchConfig OpenSearch 配置
type OpenSearchConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Secure   bool   `mapstructure:"secure"`
	IndexPrefix string `mapstructure:"index_prefix"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type       string `mapstructure:"type"` // local, s3
	LocalPath  string `mapstructure:"local_path"`
	S3Bucket   string `mapstructure:"s3_bucket"`
	S3Region   string `mapstructure:"s3_region"`
	S3Endpoint string `mapstructure:"s3_endpoint"` // MinIO 等兼容服务
	S3KeyID    string `mapstructure:"s3_key_id"`
	S3Secret   string `mapstructure:"s3_secret"`
}

// JWTConfig JWT 配置
type JWTConfig struct {
	Secret     string        `mapstructure:"secret"`
	Issuer     string        `mapstructure:"issuer"`
	ExpireTime time.Duration `mapstructure:"expire_time"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // json, console
}

// Load 加载配置
func Load() (*Config, error) {
	// 设置配置名和路径
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 添加配置搜索路径
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/opensearch-file-api/")

	// 设置环境变量前缀
	viper.SetEnvPrefix("OPENSEARCH")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	// 读取配置文件（如果存在）
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// 配置文件不存在时使用默认值和环境变量
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置默认值
func setDefaults() {
	// Server
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.mode", "release")
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 60*time.Second)

	// OpenSearch
	viper.SetDefault("opensearch.host", "localhost")
	viper.SetDefault("opensearch.port", 9200)
	viper.SetDefault("opensearch.username", "admin")
	viper.SetDefault("opensearch.password", "admin")
	viper.SetDefault("opensearch.secure", true)
	viper.SetDefault("opensearch.index_prefix", "tenant")

	// Storage
	viper.SetDefault("storage.type", "local")
	viper.SetDefault("storage.local_path", "./data/files")

	// JWT
	viper.SetDefault("jwt.secret", "change-this-secret-key")
	viper.SetDefault("jwt.issuer", "opensearch-file-api")
	viper.SetDefault("jwt.expire_time", 24*time.Hour)

	// Log
	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
}

// Address 返回服务器监听地址
func (c *ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// Address 返回 OpenSearch 地址
func (c *OpenSearchConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// URL 返回 OpenSearch URL
func (c *OpenSearchConfig) URL() string {
	protocol := "https"
	if !c.Secure {
		protocol = "http"
	}
	return fmt.Sprintf("%s://%s:%d", protocol, c.Host, c.Port)
}

// IsS3 判断是否使用 S3 存储
func (c *StorageConfig) IsS3() bool {
	return c.Type == "s3"
}
