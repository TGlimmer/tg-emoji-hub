package bootstrap

import (
	stderrors "errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config 启动期配置（只放基础设施连接 + 启动参数）
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	MCP      MCPConfig      `mapstructure:"mcp"`
	Paths    PathsConfig    `mapstructure:"paths"`
}

type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	BaseURL string `mapstructure:"base_url"`
	Mode    string `mapstructure:"mode"` // dev / prod，dev 启用 AutoMigrate
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Name     string `mapstructure:"name"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"` // console / json
}

type MCPConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"` // 实际为 Streamable HTTP 挂载路径，名字保留通用
}

type PathsConfig struct {
	SignalsDir   string `mapstructure:"signals_dir"`
	ThumbnailDir string `mapstructure:"thumbnail_dir"`
	MediaDir     string `mapstructure:"media_dir"`
}

// DSN 拼 MySQL 连接串
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		d.User, d.Password, d.Host, d.Port, d.Name)
}

// MustLoadConfig 启动期读配置，失败直接 Fatal
func MustLoadConfig() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")
	v.AutomaticEnv()

	// 默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.base_url", "http://localhost:8080")
	v.SetDefault("server.mode", "dev")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")
	v.SetDefault("mcp.enabled", true)
	v.SetDefault("mcp.path", "/mcp")
	v.SetDefault("paths.signals_dir", "./var/signals")
	v.SetDefault("paths.thumbnail_dir", "./web/public/thumbs")
	v.SetDefault("paths.media_dir", "./web/public/emoji")

	if path := os.Getenv("CONFIG_PATH"); path != "" {
		v.SetConfigFile(path)
	}

	if err := v.ReadInConfig(); err != nil {
		// 允许配置文件不存在时仍按默认值跑（开发场景方便）
		var nfe viper.ConfigFileNotFoundError
		if !stderrors.As(err, &nfe) {
			panic(fmt.Sprintf("加载配置文件失败: %v", err))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Sprintf("解析配置失败: %v", err))
	}
	return &cfg
}

// MustInitLogger 启动期日志，dev 用 console 彩色 / prod 用 json
//
// stdioMode=true 时强制走 stderr：MCP stdio 协议要求 stdout 仅承载 JSON-RPC，
// 日志混入会污染客户端解析。
func MustInitLogger(cfg *Config, stdioMode bool) *zap.Logger {
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	var zapCfg zap.Config
	if cfg.Log.Format == "json" {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)
	if stdioMode {
		zapCfg.OutputPaths = []string{"stderr"}
		zapCfg.ErrorOutputPaths = []string{"stderr"}
	}

	logger, err := zapCfg.Build()
	if err != nil {
		panic(fmt.Sprintf("初始化 logger 失败: %v", err))
	}
	return logger
}
