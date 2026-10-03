package bootstrap

import (
	"go.uber.org/zap"
	"gorm.io/gorm"

	mcpadapter "github.com/TGlimmer/tg-emoji-hub/internal/adapter/mcp"
	mysqlrepo "github.com/TGlimmer/tg-emoji-hub/internal/adapter/repository/mysql"
	"github.com/TGlimmer/tg-emoji-hub/internal/app"

	mcpserver "github.com/mark3labs/mcp-go/server"
)

// MustStartMCPStdio 进 stdio 模式：阻塞读 os.Stdin/Stdout JSON-RPC 帧。
//
// 用于 Claude Desktop / Cursor 本地集成。stdio 模式下 main.go 不应启动 HTTP，
// 否则 stdio 抢 stdout + 日志混入 JSON 帧会污染客户端。
func MustStartMCPStdio(cfg *Config, db *gorm.DB, logger *zap.Logger) error {
	mcpSrv := buildMCPServer(cfg, db)
	logger.Info("MCP stdio 模式启动")
	return mcpserver.ServeStdio(mcpSrv)
}

// NewMCPHTTPHandler 返回挂到 Gin 的 http.Handler（Streamable HTTP）。
//
// HTTP 模式用；cfg.MCP.Enabled=false → 返回 nil。
func NewMCPHTTPHandler(cfg *Config, db *gorm.DB, logger *zap.Logger) *mcpserver.StreamableHTTPServer {
	if !cfg.MCP.Enabled {
		logger.Info("MCP 已禁用，HTTP handler 不挂载")
		return nil
	}
	mcpSrv := buildMCPServer(cfg, db)
	logger.Info("MCP Streamable HTTP 已构建", zap.String("path", cfg.MCP.Path))
	return mcpserver.NewStreamableHTTPServer(mcpSrv)
}

// buildMCPServer 共享给 stdio / HTTP 两种模式
//
// 注意：当前 stdio + HTTP 各自构建独立的 *MCPServer 实例 + 独立的 repo/usecase。
// 这是有意为之 — stdio 是单进程独占模式（main.go 不启 HTTP），不会双实例。
// 验证版本：github.com/mark3labs/mcp-go v0.54.0
func buildMCPServer(cfg *Config, db *gorm.DB) *mcpserver.MCPServer {
	searchRepo := mysqlrepo.NewSearchRepo(db)
	packRepo := mysqlrepo.NewPackRepo(db)

	searchUC := app.NewSearchUseCase(searchRepo)
	packUC := app.NewPackUseCase(packRepo, cfg.Paths.SignalsDir)

	return mcpadapter.NewServer(mcpadapter.Deps{
		SearchUC: searchUC,
		PackUC:   packUC,
		BaseURL:  cfg.Server.BaseURL,
		Name:     "tg-emoji-hub",
		Version:  Version,
	})
}
