package chat

import (
	"testing"
	"time"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
)

// newChatServerForTest 构造测试专用 Server，并根治 Windows 下 t.TempDir()
// 清理竞态族（FIX-CHAT-TMPDIR-FLAKE-1）：测试函数返回后 t.TempDir() 的
// RemoveAll 会与 Server 后台 goroutine（continuation scheduler 的 250ms
// ticker、confirmed certification start 拉起的 job）对 .vit_agent / $HOME
// 状态目录的写盘交错，命中 "The directory is not empty"。本 helper 保证：
//
//  1. scheduler 熄火——构造后立即消耗 schedulerOnce 并 cancel 上下文：
//     goroutine 起来后第一轮 select 即从 Done 分支退出；此后
//     wakeContinuationScheduler 里的 startContinuationScheduler 因
//     sync.Once 已消耗不再拉起新 goroutine。这同时消灭两类症状：测试
//     返回后的周期写盘（清理竞态），以及测试中途 scheduler 抢先续跑
//     pending 链改变被测状态（续跑语义本身由
//     continuation_scheduler_test.go 以直接构造锁定，不受影响）。
//  2. certification job 收尾——t.Cleanup 轮询 runningProcessorCertificationJob
//     至终态，确保 job goroutine 的全部写盘发生在 TempDir RemoveAll 之前。
//  3. Close() 幂等收尾（schedulerDone 已 closed，等待立即返回）。
//
// t.Cleanup 按 LIFO 运行：helper 在 t.TempDir() 之后注册，故收尾先于
// RemoveAll 执行。需要真实 scheduler 行为的测试请直接用 New/Start。
func newChatServerForTest(t *testing.T, kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Server {
	t.Helper()
	server := New(kernelClient, shadowProject, logger)
	server.startContinuationScheduler()
	server.schedulerCancel()
	t.Cleanup(func() {
		_ = server.Close()
		waitProcessorCertificationJobsForTest(t, server)
	})
	return server
}

// waitProcessorCertificationJobsForTest 等待全部 certification job 离开
// queued/running。测试环境（kernel 为 nil、无真实 agent 端口）的 job 会因
// 回环 HTTP 立即失败而进入终态；deadline 只作兜底并留日志，不作为断言。
func waitProcessorCertificationJobsForTest(t *testing.T, server *Server) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if server.runningProcessorCertificationJob() == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job := server.runningProcessorCertificationJob(); job != nil {
		t.Logf("processor certification job %s still %s at wait deadline", job.JobID, job.Status)
	}
}
