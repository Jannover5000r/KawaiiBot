// Package scheduler handles scheduled tasks like daily webhooks
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"KawaiiBot/webhook"
)

// Scheduler handles scheduled tasks
type Scheduler struct {
	dailyWebhook *webhook.DailyWebhook
	ticker       *time.Ticker
	mutex        sync.Mutex
	running      bool
	stopChan     chan struct{}
}

// New creates a new Scheduler instance
func New(dailyWebhook *webhook.DailyWebhook) *Scheduler {
	return &Scheduler{
		dailyWebhook: dailyWebhook,
		stopChan:     make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	// Check if daily webhook is configured
	if !s.dailyWebhook.IsEnabled() {
		log.Println("Daily webhook is not configured or disabled, scheduler will not start")
		return nil
	}

	s.running = true

	// Start the scheduling routine
	go s.schedulingRoutine(ctx)

	log.Println("Scheduler started successfully")
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	close(s.stopChan)
	s.running = false

	if s.ticker != nil {
		s.ticker.Stop()
	}

	log.Println("Scheduler stopped")
	return nil
}

// schedulingRoutine runs the main scheduling loop
func (s *Scheduler) schedulingRoutine(ctx context.Context) {
	// Calculate time until next midnight
	timeUntilNextSend := s.getTimeUntilNextSend()

	log.Printf("First daily webhook will be sent in %v", timeUntilNextSend)

	// Create timer for first execution at midnight
	timer := time.NewTimer(timeUntilNextSend)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Scheduler context cancelled")
			return
		case <-s.stopChan:
			log.Println("Scheduler stopped by request")
			return
		case <-timer.C:
			// It's midnight! Send the daily webhook
			s.sendDailyWebhook()

			// Reset timer for next midnight (24 hours from now)
			// Calculate time until next midnight and reset timer
			timeUntilNextSend := s.getTimeUntilNextSend()
			log.Printf("Next daily webhook will be sent in %v", timeUntilNextSend)
			timer.Reset(timeUntilNextSend)
		}
	}
}

func (s *Scheduler) getTimeUntilNextSend() time.Duration {
	now := time.Now()

	// Create target time: today at 5:00:00 AM in local timezone
	target := time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, now.Location())

	// If 5 AM today has already passed, schedule for tomorrow
	if now.After(target) || now.Equal(target) {
		target = target.Add(24 * time.Hour)
	}

	timeUntil := target.Sub(now)
	log.Printf("[SCHEDULER] Current time: %s, Next 5AM: %s, Time until: %v",
		now.Format("2006-01-02 15:04:05"),
		target.Format("2006-01-02 15:04:05"),
		timeUntil)
	return timeUntil
}

// sendDailyWebhook sends the daily webhook
func (s *Scheduler) sendDailyWebhook() {
	log.Println("[SCHEDULER] Attempting to send daily webhook...")

	// Check if webhook is still enabled
	if !s.dailyWebhook.IsEnabled() {
		log.Println("[SCHEDULER] Daily webhook is disabled, skipping")
		return
	}

	// Get webhook status for logging
	enabled, url := s.dailyWebhook.GetStatus()
	log.Printf("[SCHEDULER] Webhook status - Enabled: %v, URL configured: %v", enabled, url != "")

	// Send the webhook with retry logic
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		log.Printf("[SCHEDULER] Sending webhook (attempt %d/%d)...", i+1, maxRetries)
		err := s.dailyWebhook.SendDailyWebhook()
		if err == nil {
			log.Println("[SCHEDULER] Daily webhook sent successfully")
			return
		}

		log.Printf("[SCHEDULER] Failed to send daily webhook (attempt %d/%d): %v", i+1, maxRetries, err)

		if i < maxRetries-1 {
			// Wait before retrying (exponential backoff)
			waitTime := time.Duration(i+1) * 5 * time.Minute
			log.Printf("[SCHEDULER] Waiting %v before retry...", waitTime)
			time.Sleep(waitTime)
		}
	}

	log.Printf("[SCHEDULER] Failed to send daily webhook after %d attempts", maxRetries)
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.running
}

// ForceSend forces sending a daily webhook immediately
func (s *Scheduler) ForceSend() error {
	if !s.dailyWebhook.IsEnabled() {
		return fmt.Errorf("daily webhook is disabled")
	}

	go s.sendDailyWebhook()
	return nil
}

