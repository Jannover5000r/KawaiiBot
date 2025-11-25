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
	timeUntilMidnight := s.getTimeUntilMidnight()
	
	log.Printf("First daily webhook will be sent in %v", timeUntilMidnight)
	
	// Create timer for first execution at midnight
	timer := time.NewTimer(timeUntilMidnight)
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
			timer.Reset(24 * time.Hour)
		}
	}
}

// getTimeUntilMidnight calculates the time until the next midnight
func (s *Scheduler) getTimeUntilMidnight() time.Duration {
	now := time.Now()
	
	// Calculate next midnight
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	
	// If it's exactly midnight, send immediately
	if now.Hour() == 0 && now.Minute() == 0 && now.Second() == 0 {
		return 0
	}
	
	return nextMidnight.Sub(now)
}

// sendDailyWebhook sends the daily webhook
func (s *Scheduler) sendDailyWebhook() {
	log.Println("Sending daily webhook...")
	
	// Check if webhook is still enabled
	if !s.dailyWebhook.IsEnabled() {
		log.Println("Daily webhook is disabled, skipping")
		return
	}
	
	// Send the webhook with retry logic
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		err := s.dailyWebhook.SendDailyWebhook()
		if err == nil {
			log.Println("Daily webhook sent successfully")
			return
		}
		
		log.Printf("Failed to send daily webhook (attempt %d/%d): %v", i+1, maxRetries, err)
		
		if i < maxRetries-1 {
			// Wait before retrying (exponential backoff)
			time.Sleep(time.Duration(i+1) * 5 * time.Minute)
		}
	}
	
	log.Printf("Failed to send daily webhook after %d attempts", maxRetries)
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