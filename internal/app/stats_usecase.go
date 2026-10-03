package app

import (
	"context"

	"github.com/TGlimmer/tg-emoji-hub/internal/domain/port"
)

// StatsUseCase 首页统计用例
type StatsUseCase struct {
	repo port.StatsRepository
}

func NewStatsUseCase(repo port.StatsRepository) *StatsUseCase {
	return &StatsUseCase{repo: repo}
}

// Overview 直通 repo，无业务规则
func (u *StatsUseCase) Overview(ctx context.Context) (*port.StatsOverview, error) {
	return u.repo.Overview(ctx)
}
