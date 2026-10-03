package bootstrap

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// shutdownTimeout 优雅关闭最长等待时间
const shutdownTimeout = 5 * time.Second

// WaitAndShutdown 阻塞等待 ctx 取消信号 → 优雅停 HTTP server
// 设计：把 main() 末尾的 lifecycle 块抽出来，让 main() 保持 ≤ 30 行硬上限。
func WaitAndShutdown(ctx context.Context, httpSrv *HTTPServer, logger *zap.Logger) {
	<-ctx.Done()
	logger.Info("收到退出信号，开始优雅关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	httpSrv.Shutdown(shutdownCtx)

	logger.Info("退出完成")
}
