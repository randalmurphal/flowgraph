package event

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Bus provides pub/sub event distribution with fan-out support.
type Bus interface {
	// Publish sends an event to all subscribers.
	Publish(ctx context.Context, evt Event) error

	// Subscribe creates a subscription for specific event types.
	Subscribe(types []string, handler Handler) Subscription

	// SubscribeAll subscribes to all events.
	SubscribeAll(handler Handler) Subscription

	// Close shuts down the bus and all subscriptions.
	Close() error
}

// Subscription represents an active subscription.
type Subscription interface {
	// Unsubscribe removes the subscription.
	Unsubscribe()

	// Pause temporarily stops delivery.
	Pause()

	// Resume continues delivery after pause.
	Resume()

	// IsPaused returns true if the subscription is paused.
	IsPaused() bool
}

// BusConfig configures bus behavior.
type BusConfig struct {
	// BufferSize is the channel buffer size per subscription.
	// Default: 256
	BufferSize int

	// MaxSubscribers limits total subscriptions.
	// Default: 0 (unlimited)
	MaxSubscribers int

	// NonBlocking makes Publish non-blocking (drops events if buffer full).
	// Default: false (blocking)
	NonBlocking bool

	// DeduplicateTTL enables deduplication with the given TTL.
	// Default: 0 (disabled)
	DeduplicateTTL time.Duration

	// OnDrop is called when an event is dropped (non-blocking mode).
	OnDrop func(evt Event, subscriberID string)

	// OnError is called when a handler returns an error.
	OnError func(evt Event, subscriberID string, err error)
}

// DefaultBusConfig provides reasonable defaults.
var DefaultBusConfig = BusConfig{
	BufferSize: 256,
}

// LocalBus is an in-memory event bus implementation.
type LocalBus struct {
	config BusConfig

	mu            sync.RWMutex
	subscriptions map[string]*subscription
	byType        map[string]map[string]*subscription // event type -> subscription ID -> subscription
	wildcards     map[string]*subscription            // subscriptions for all events

	// Deduplication cache
	dedupeMu    sync.RWMutex
	dedupeCache map[string]time.Time

	nextID  atomic.Int64
	closed  atomic.Bool
	closeCh chan struct{}
}

// NewBus creates a new local event bus.
func NewBus(config BusConfig) *LocalBus {
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultBusConfig.BufferSize
	}

	bus := &LocalBus{
		config:        config,
		subscriptions: make(map[string]*subscription),
		byType:        make(map[string]map[string]*subscription),
		wildcards:     make(map[string]*subscription),
		closeCh:       make(chan struct{}),
	}

	if config.DeduplicateTTL > 0 {
		bus.dedupeCache = make(map[string]time.Time)
		go bus.cleanupDedupe()
	}

	return bus
}

// subscription is an internal subscription implementation.
type subscription struct {
	id      string
	types   []string // empty = all types
	handler Handler
	events  chan Event
	paused  atomic.Bool
	done    chan struct{}
	bus     *LocalBus
}

// Publish sends an event to all matching subscribers.
func (b *LocalBus) Publish(ctx context.Context, evt Event) error {
	if b.closed.Load() {
		return &EventError{
			Event:   evt,
			Message: "bus is closed",
		}
	}

	// Check deduplication
	if b.config.DeduplicateTTL > 0 {
		if b.isDuplicate(evt) {
			return nil // Silently skip duplicates
		}
		b.recordEvent(evt)
	}

	// Get matching subscriptions
	b.mu.RLock()
	subs := b.getMatchingSubscriptions(evt.Type())
	b.mu.RUnlock()

	// Deliver to each subscription
	for _, sub := range subs {
		if sub.paused.Load() {
			continue
		}

		if b.config.NonBlocking {
			select {
			case sub.events <- evt:
			default:
				// Buffer full - drop event
				if b.config.OnDrop != nil {
					b.config.OnDrop(evt, sub.id)
				}
			}
		} else {
			select {
			case sub.events <- evt:
			case <-ctx.Done():
				return ctx.Err()
			case <-b.closeCh:
				return &EventError{
					Event:   evt,
					Message: "bus closed during publish",
				}
			}
		}
	}

	return nil
}

// Subscribe creates a subscription for specific event types.
func (b *LocalBus) Subscribe(types []string, handler Handler) Subscription {
	return b.subscribe(types, handler)
}

// SubscribeAll subscribes to all events.
func (b *LocalBus) SubscribeAll(handler Handler) Subscription {
	return b.subscribe(nil, handler)
}

func (b *LocalBus) subscribe(types []string, handler Handler) *subscription {
	if b.closed.Load() {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check subscriber limit
	if b.config.MaxSubscribers > 0 && len(b.subscriptions) >= b.config.MaxSubscribers {
		return nil
	}

	id := b.nextID.Add(1)
	sub := &subscription{
		id:      string(rune(id)),
		types:   types,
		handler: handler,
		events:  make(chan Event, b.config.BufferSize),
		done:    make(chan struct{}),
		bus:     b,
	}

	b.subscriptions[sub.id] = sub

	if len(types) == 0 {
		b.wildcards[sub.id] = sub
	} else {
		for _, t := range types {
			if b.byType[t] == nil {
				b.byType[t] = make(map[string]*subscription)
			}
			b.byType[t][sub.id] = sub
		}
	}

	// Start processing goroutine
	go sub.process()

	return sub
}

// getMatchingSubscriptions returns all subscriptions matching an event type.
func (b *LocalBus) getMatchingSubscriptions(eventType string) []*subscription {
	subs := make([]*subscription, 0)

	// Add type-specific subscriptions
	if typeSubs, ok := b.byType[eventType]; ok {
		for _, sub := range typeSubs {
			subs = append(subs, sub)
		}
	}

	// Add wildcard subscriptions
	for _, sub := range b.wildcards {
		subs = append(subs, sub)
	}

	return subs
}

// Close shuts down the bus.
func (b *LocalBus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil // Already closed
	}

	close(b.closeCh)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Close all subscriptions
	for _, sub := range b.subscriptions {
		close(sub.done)
	}

	return nil
}

// process handles events for a subscription.
func (s *subscription) process() {
	for {
		select {
		case evt := <-s.events:
			if s.paused.Load() {
				continue
			}

			_, err := s.handler.Handle(context.Background(), evt)
			if err != nil && s.bus.config.OnError != nil {
				s.bus.config.OnError(evt, s.id, err)
			}

		case <-s.done:
			return
		}
	}
}

// Unsubscribe removes the subscription.
func (s *subscription) Unsubscribe() {
	s.bus.mu.Lock()
	defer s.bus.mu.Unlock()

	delete(s.bus.subscriptions, s.id)
	delete(s.bus.wildcards, s.id)

	for _, t := range s.types {
		if typeSubs, ok := s.bus.byType[t]; ok {
			delete(typeSubs, s.id)
		}
	}

	close(s.done)
}

// Pause temporarily stops delivery.
func (s *subscription) Pause() {
	s.paused.Store(true)
}

// Resume continues delivery after pause.
func (s *subscription) Resume() {
	s.paused.Store(false)
}

// IsPaused returns true if the subscription is paused.
func (s *subscription) IsPaused() bool {
	return s.paused.Load()
}

// Deduplication helpers

func (b *LocalBus) isDuplicate(evt Event) bool {
	b.dedupeMu.RLock()
	defer b.dedupeMu.RUnlock()

	_, exists := b.dedupeCache[evt.ID()]
	return exists
}

func (b *LocalBus) recordEvent(evt Event) {
	b.dedupeMu.Lock()
	defer b.dedupeMu.Unlock()

	b.dedupeCache[evt.ID()] = time.Now()
}

func (b *LocalBus) cleanupDedupe() {
	ticker := time.NewTicker(b.config.DeduplicateTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.dedupeMu.Lock()
			cutoff := time.Now().Add(-b.config.DeduplicateTTL)
			for id, ts := range b.dedupeCache {
				if ts.Before(cutoff) {
					delete(b.dedupeCache, id)
				}
			}
			b.dedupeMu.Unlock()

		case <-b.closeCh:
			return
		}
	}
}
