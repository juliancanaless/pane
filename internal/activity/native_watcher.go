package activity

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/rjeczalik/notify"
)

// NativeWatcher uses platform-native file system events (FSEvents/inotify)
// with recursive watching, ignore filtering, and debouncing.
type NativeWatcher struct {
	Root     string
	OnEvent  WatchFunc
	Debounce time.Duration // default 100ms
}

func (w NativeWatcher) Run(ctx context.Context) error {
	debounce := w.Debounce
	if debounce == 0 {
		debounce = 100 * time.Millisecond
	}

	filter := NewIgnoreFilter(w.Root)

	// Buffer size for the event channel
	c := make(chan notify.EventInfo, 256)

	// Recursive watch: "..." suffix means all subdirectories
	watchPath := filepath.Join(w.Root, "...")
	if err := notify.Watch(watchPath, c, notify.All); err != nil {
		return err
	}
	defer notify.Stop(c)

	// Debounce: collect events over the debounce window, emit unique paths
	var mu sync.Mutex
	pending := make(map[string]WatchEvent)

	flush := func() {
		mu.Lock()
		batch := pending
		pending = make(map[string]WatchEvent)
		mu.Unlock()

		for _, event := range batch {
			w.emit(event)
		}
	}

	timer := time.NewTimer(debounce)
	timer.Stop()

	for {
		select {
		case <-ctx.Done():
			flush()
			return nil
		case ei := <-c:
			path := ei.Path()

			// Filter out ignored paths
			if filter.ShouldIgnore(path) {
				continue
			}

			eventType := mapEventType(ei.Event())

			mu.Lock()
			// Last event for a path wins within the debounce window
			pending[path] = WatchEvent{
				Path:      path,
				EventType: eventType,
				Time:      time.Now(),
			}
			mu.Unlock()

			// Reset the debounce timer
			timer.Reset(debounce)

		case <-timer.C:
			flush()
		}
	}
}

func (w NativeWatcher) emit(event WatchEvent) {
	if w.OnEvent != nil {
		w.OnEvent(event)
	}
}

func mapEventType(e notify.Event) EventType {
	switch {
	case e&notify.Create != 0:
		return EventCreated
	case e&notify.Write != 0:
		return EventModified
	case e&notify.Remove != 0:
		return EventDeleted
	case e&notify.Rename != 0:
		return EventRenamed
	default:
		return EventModified
	}
}
