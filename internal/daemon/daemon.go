package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kerren/op-ssh-manager/internal/log"
	"github.com/robfig/cron/v3"
)

// Daemon runs sync on a schedule
type Daemon struct {
	Schedule  string
	LockFile  string
	SyncFunc  func(ctx context.Context) error
	scheduler *cron.Cron
}

// New creates a new daemon
func New(schedule, lockFile string, syncFunc func(ctx context.Context) error) *Daemon {
	return &Daemon{
		Schedule: schedule,
		LockFile: lockFile,
		SyncFunc: syncFunc,
	}
}

// Run starts the daemon and blocks until interrupted
func (d *Daemon) Run(ctx context.Context) error {
	// Acquire lock
	lock, err := d.acquireLock()
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer d.releaseLock(lock)

	// Parse schedule
	d.scheduler = cron.New(cron.WithParser(cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)))

	_, err = d.scheduler.AddFunc(d.Schedule, func() {
		d.runSync(ctx)
	})
	if err != nil {
		return fmt.Errorf("invalid schedule: %w", err)
	}

	// Start scheduler
	d.scheduler.Start()
	log.Info("daemon started", "schedule", d.Schedule)

	// Run initial sync
	d.runSync(ctx)

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		log.Info("daemon stopping (context cancelled)")
	case sig := <-sigCh:
		log.Info("daemon stopping", "signal", sig)
	}

	// Stop scheduler
	stopCtx := d.scheduler.Stop()
	<-stopCtx.Done()

	return nil
}

// runSync runs a single sync operation
func (d *Daemon) runSync(ctx context.Context) {
	log.Info("running scheduled sync")
	start := time.Now()

	if err := d.SyncFunc(ctx); err != nil {
		log.Error("sync failed", "error", err, "duration", time.Since(start))
	} else {
		log.Info("sync completed", "duration", time.Since(start))
	}
}

// acquireLock acquires the daemon lock file
func (d *Daemon) acquireLock() (*os.File, error) {
	dir := filepath.Dir(d.LockFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(d.LockFile, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}

	// Try to lock the file
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("another daemon instance is running")
	}

	// Write PID
	file.Truncate(0)
	file.Seek(0, 0)
	fmt.Fprintf(file, "%d\n", os.Getpid())

	return file, nil
}

// releaseLock releases the daemon lock file
func (d *Daemon) releaseLock(file *os.File) {
	if file == nil {
		return
	}
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	file.Close()
	os.Remove(d.LockFile)
}

// RunOnce runs sync once without scheduling
func (d *Daemon) RunOnce(ctx context.Context) error {
	return d.SyncFunc(ctx)
}
