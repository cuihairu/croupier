package analytics

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// groupCounter feeds hand-set group progress readings: miniredis's lag is not
// faithful to real Redis, so DATA-047 asserts through this fake.
type groupCounter struct {
	retained  int64
	groups    []redis.XInfoGroup
	groupsErr error
}

func (c *groupCounter) XLen(context.Context, string) *redis.IntCmd {
	return redis.NewIntResult(c.retained, nil)
}

func (c *groupCounter) XInfoGroups(_ context.Context, stream string) *redis.XInfoGroupsCmd {
	cmd := redis.NewXInfoGroupsCmd(context.Background(), stream)
	if c.groupsErr != nil {
		cmd.SetErr(c.groupsErr)
		return cmd
	}
	cmd.SetVal(c.groups)
	return cmd
}

func createdAlert(t *testing.T, sink *fakeAlertSinkV9, alertID string) *model.Alert {
	t.Helper()
	for _, a := range sink.created {
		if a.AlertID == alertID {
			return a
		}
	}
	return nil
}

func newBacklogMonitor(t *testing.T, dead DeadLetterCounter, limit int64) (*PipelineMonitor, *fakeAlertSinkV9) {
	t.Helper()
	sink := &fakeAlertSinkV9{}
	m := NewPipelineMonitor(sink, dead)
	m.cfg.WorkerGroup = "analytics-worker"
	m.cfg.MQBacklogLimit = limit
	return m, sink
}

// 验收：同一流全部消费并 ACK 后，即使物理长度（XLEN）很大也不再报积压。
func TestMonitorBacklogNotFiredAfterFullConsume(t *testing.T) {
	m, sink := newBacklogMonitor(t, &groupCounter{
		retained: 10000,
		groups:   []redis.XInfoGroup{{Name: "analytics-worker", Pending: 0, Lag: 0}},
	}, 10)
	m.checkMQBacklog(context.Background())
	assert.Nil(t, createdAlert(t, sink, "pipe:mq_backlog:events"), "fully consumed stream must not be reported as backlog")
}

func TestMonitorBacklogFiresOnProgressWithSplit(t *testing.T) {
	m, sink := newBacklogMonitor(t, &groupCounter{
		retained: 10000,
		groups:   []redis.XInfoGroup{{Name: "analytics-worker", Pending: 4, Lag: 8}},
	}, 10)
	m.checkMQBacklog(context.Background())
	alert := createdAlert(t, sink, "pipe:mq_backlog:events")
	require.NotNil(t, alert)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	// 拆分披露：未派发 8 + 已派发未 ACK 4（留存 10000 不是积压数）
	assert.Equal(t, int64(8), *alert.Details["undelivered"].(*int64))
	assert.Equal(t, int64(4), *alert.Details["pendingAck"].(*int64))
	assert.Equal(t, int64(10000), alert.Details["retained"])
}

// lag 未知（Redis < 7）但 pending 超阈 → pending 自身即停摆证据，仍触发；
// details 中未派发为 null（未知），不是 0。
func TestMonitorBacklogUnknownLagPendingEvidence(t *testing.T) {
	m, sink := newBacklogMonitor(t, &groupCounter{
		retained: 10000,
		groups:   []redis.XInfoGroup{{Name: "analytics-worker", Pending: 12, Lag: -1}},
	}, 10)
	m.checkMQBacklog(context.Background())
	alert := createdAlert(t, sink, "pipe:mq_backlog:events")
	require.NotNil(t, alert)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	assert.Nil(t, alert.Details["undelivered"])
	assert.Equal(t, int64(12), *alert.Details["pendingAck"].(*int64))
}

// lag 未知且 pending 低于阈值 → 无判决：既不触发也不解除既有告警
// （DATA-022：未知轮次不构成正向恢复证据）。
func TestMonitorBacklogUnknownRoundKeepsFiring(t *testing.T) {
	sink := &fakeAlertSinkV9{}
	m := NewPipelineMonitor(sink, &groupCounter{
		retained: 10000,
		groups:   []redis.XInfoGroup{{Name: "analytics-worker", Pending: 0, Lag: -1}},
	})
	m.cfg.WorkerGroup = "analytics-worker"
	m.cfg.MQBacklogLimit = 10
	sink.existing = &model.Alert{ID: 1, AlertID: "pipe:mq_backlog:events", Status: PipelineAlertStatusFiring}

	m.checkMQBacklog(context.Background())
	assert.Empty(t, sink.created)
	assert.Empty(t, sink.updated, "unknown round must neither fire nor resolve")
}

// 组信息读取失败 → 未知轮次：不触发、不解除。
func TestMonitorBacklogGroupErrorIsUnknown(t *testing.T) {
	sink := &fakeAlertSinkV9{}
	m := NewPipelineMonitor(sink, &groupCounter{retained: 10000, groupsErr: errors.New("x")})
	m.cfg.WorkerGroup = "analytics-worker"
	m.cfg.MQBacklogLimit = 10
	sink.existing = &model.Alert{ID: 2, AlertID: "pipe:mq_backlog:events", Status: PipelineAlertStatusFiring}

	m.checkMQBacklog(context.Background())
	assert.Empty(t, sink.created)
	assert.Empty(t, sink.updated)
}

// 无消费组 → 组尚未建立（DATA-023 建组从 0 开始），全部留存按未派发计。
func TestMonitorBacklogMissingGroupCountsAllRetained(t *testing.T) {
	m, sink := newBacklogMonitor(t, &groupCounter{retained: 50}, 10)
	m.checkMQBacklog(context.Background())
	alert := createdAlert(t, sink, "pipe:mq_backlog:events")
	require.NotNil(t, alert)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	assert.Equal(t, int64(50), *alert.Details["undelivered"].(*int64))
	assert.Equal(t, int64(0), *alert.Details["pendingAck"].(*int64))
}
