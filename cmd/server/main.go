package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/TGlimmer/tg-emoji-hub/cmd/server/bootstrap"
)

func main() {
	mcpStdio := flag.Bool("mcp-stdio", false,
		"以 MCP stdio 模式启动（Claude Desktop / Cursor 本地集成）")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg := bootstrap.MustLoadConfig()
	logger := bootstrap.MustInitLogger(cfg, *mcpStdio)
	defer func() { _ = logger.Sync() }()

	db := bootstrap.MustInitDB(cfg, logger)
	defer bootstrap.MustClose(db, logger)

	if *mcpStdio {
		if err := bootstrap.MustStartMCPStdio(cfg, db, logger); err != nil {
			logger.Fatal("MCP stdio 异常", zap.Error(err))
		}
		return
	}

	httpSrv := bootstrap.MustStartHTTP(cfg, db, logger)
	bootstrap.WaitAndShutdown(ctx, httpSrv, logger)
}
