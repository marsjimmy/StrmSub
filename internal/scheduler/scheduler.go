// Package scheduler 定时触发 pipeline，也支持 Web 界面手动触发。
package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/marsjimmy/strmsub/internal/pipeline"
)

type Scheduler struct {
	mu       sync.Mutex
	interval time.Duration
	pipe     *pipeline.Pipeline
	trigger  chan struct{}
	reset    chan struct{} // 间隔变更时通知 Start 循环重建 ticker
}

func New(interval time.Duration, pipe *pipeline.Pipeline) *Scheduler {
	return &Scheduler{interval: interval, pipe: pipe, trigger: make(chan struct{}, 1), reset: make(chan struct{}, 1)}
}

// Trigger 手动触发一轮（非阻塞，合并连续触发）
func (s *Scheduler) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// SetInterval 热更新扫描间隔（设置页保存后调用）
func (s *Scheduler) SetInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	s.mu.Lock()
	s.interval = d
	s.mu.Unlock()
	select {
	case s.reset <- struct{}{}:
	default:
	}
}

func (s *Scheduler) getInterval() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.interval
}

func (s *Scheduler) Start(ctx context.Context) {
	// 启动先跑一轮
	go s.pipe.RunOnce(ctx)
	t := time.NewTicker(s.getInterval())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.pipe.RunOnce(ctx)
		case <-s.trigger:
			s.pipe.RunOnce(ctx)
		case <-s.reset:
			t.Reset(s.getInterval())
		}
	}
}
