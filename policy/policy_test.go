package policy

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPolicyStopInterruptsPollingWait(t *testing.T) {
	stop := PolicyServicesStart()
	stopped := make(chan struct{})
	go func() {
		stop()
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("policy stop did not interrupt its polling wait")
	}
}

func TestPolicyMaintenanceDrain(t *testing.T) {
	var pending sync.WaitGroup
	var running atomic.Bool
	entered := make(chan struct{})
	release := make(chan struct{})
	runOnceAsync(&running, &pending, func() {
		close(entered)
		<-release
	})
	<-entered
	runOnceAsync(&running, &pending, func() {
		t.Error("maintenance task reentered before completion")
	})
	drained := make(chan struct{})
	go func() {
		pending.Wait()
		close(drained)
	}()
	select {
	case <-drained:
		t.Error("pending maintenance was not awaited")
	default:
	}
	close(release)
	<-drained
	if running.Load() {
		t.Fatal("maintenance running flag was not released")
	}
}
