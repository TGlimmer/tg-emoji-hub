package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/TGlimmer/tg-emoji-hub/internal/adapter/handler"
	mysqlrepo "github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"
)

// Version 主服务版本
const Version = "0.1.0"

// HTTPServer 包装 gin engine + http server，便于 graceful shutdown
type HTTPServer struct {
	srv    *http.Server
	logger *zap.Logger
}

// MustStartHTTP 装配 HTTP 路由与 server，监听端口
func MustStartHTTP(cfg *Config, db *gorm.DB, logger *zap.Logger) *HTTPServer {
	if cfg.Server.Mode != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(zapAccessLog(logger))

	// repo / usecase / handler 装配
	searchRepo := mysqlrepo.NewSearchRepo(db)
	packRepo := mysqlrepo.NewPackRepo(db)
	statsRepo := mysqlrepo.NewStatsRepo(db)

	searchUC := app.NewSearchUseCase(searchRepo)
	packUC := app.NewPackUseCase(packRepo, cfg.Paths.SignalsDir)
	statsUC := app.NewStatsUseCase(statsRepo)

	searchH := handler.NewSearchHandler(searchUC, cfg.Server.BaseURL)
	packH := handler.NewPackHandler(packUC, cfg.Server.BaseURL)
	statsH := handler.NewStatsHandler(statsUC)
	healthH := handler.NewHealthHandler(func() error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return sqlDB.PingContext(ctx)
	}, Version)

	handler.Register(r, handler.Deps{
		SearchHandler: searchH,
		PackHandler:   packH,
		StatsHandler:  statsH,
		HealthHandler: healthH,
		ThumbnailDir:  cfg.Paths.ThumbnailDir,
		MediaDir:      cfg.Paths.MediaDir,
	})

	// 挂 MCP Streamable HTTP 到 cfg.MCP.Path
	if mcpHandler := NewMCPHTTPHandler(cfg, db, logger); mcpHandler != nil {
		path := cfg.MCP.Path
		if path == "" {
			path = "/mcp"
		}
		// gin.WrapH 适配标准 http.Handler，Any 匹配所有 method（POST 主流 + GET 兼容）
		r.Any(path, gin.WrapH(mcpHandler))
		r.Any(path+"/*any", gin.WrapH(mcpHandler))
		logger.Info("MCP HTTP 路由已挂载", zap.String("path", path))
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("HTTP 监听", zap.Int("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 启动失败", zap.Error(err))
		}
	}()

	return &HTTPServer{srv: srv, logger: logger}
}

// Shutdown 优雅停服
func (h *HTTPServer) Shutdown(ctx context.Context) {
	if h == nil || h.srv == nil {
		return
	}
	if err := h.srv.Shutdown(ctx); err != nil {
		h.logger.Warn("HTTP shutdown 异常", zap.Error(err))
	}
}

// zapAccessLog 简单访问日志中间件
func zapAccessLog(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http_access",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("cost", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
