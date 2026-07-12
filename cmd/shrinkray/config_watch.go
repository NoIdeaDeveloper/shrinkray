package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gwlsn/shrinkray/internal/api"
	"github.com/gwlsn/shrinkray/internal/config"
)

type configReloadOptions struct {
	mediaOverride string
	queueFile     string
}

func startConfigWatcher(ctx context.Context, cfgPath string, handler *api.Handler, cfg *config.Config, opts configReloadOptions) {
	if cfgPath == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Warning: Failed to start config watcher: %v", err)
		return
	}

	watchDir := filepath.Dir(cfgPath)
	if err := watcher.Add(watchDir); err != nil {
		log.Printf("Warning: Failed to watch config directory %s: %v", watchDir, err)
		_ = watcher.Close()
		return
	}

	cfgPathAbs, err := filepath.Abs(cfgPath)
	if err != nil {
		cfgPathAbs = cfgPath
	}

	// Track in-flight reloads so shutdown can wait for them to complete
	var reloadWG sync.WaitGroup
	reloadDone := make(chan struct{})

	reloadCh := make(chan struct{}, 1)
	go func() {
		defer watcher.Close()

		var timer *time.Timer
		triggerReload := func() {
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(250*time.Millisecond, func() {
				reloadWG.Add(1)
				defer reloadWG.Done()

				newCfg, err := config.Load(cfgPath)
				if err != nil {
					log.Printf("Warning: Failed to reload config from %s: %v", cfgPath, err)
					return
				}
				if opts.mediaOverride != "" {
					newCfg.MediaPath = opts.mediaOverride
				}
				if opts.queueFile != "" {
					newCfg.QueueFile = opts.queueFile
				}

				if newCfg.MediaPath == "" {
					newCfg.MediaPath = cfg.MediaPath
				}
				if _, err := os.Stat(newCfg.MediaPath); err != nil {
					log.Printf("Warning: Ignoring config reload, media path unavailable: %s (%v)", newCfg.MediaPath, err)
					return
				}

				handler.ApplyConfig(newCfg)
				log.Printf("Config reloaded from %s", cfgPath)
			})
		}

		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				// Wait for any in-flight reload to finish before signaling done
				go func() {
					reloadWG.Wait()
					close(reloadDone)
				}()
				return
			case <-reloadCh:
				triggerReload()
			case event, ok := <-watcher.Events:
				if !ok {
					go func() {
						reloadWG.Wait()
						close(reloadDone)
					}()
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				eventPath, err := filepath.Abs(event.Name)
				if err != nil {
					eventPath = event.Name
				}
				if eventPath != cfgPathAbs {
					continue
				}
				select {
				case reloadCh <- struct{}{}:
				default:
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					go func() {
						reloadWG.Wait()
						close(reloadDone)
					}()
					return
				}
				log.Printf("Warning: Config watcher error: %v", err)
			}
		}
	}()

	// Store reloadDone for the caller to wait on during shutdown
	// The caller (main.go) should call WaitConfigReload() after ctx cancel
	configWatcherDone = reloadDone
}

// configWatcherDone is set by startConfigWatcher; main calls WaitConfigReload during shutdown
var configWatcherDone chan struct{}

// WaitConfigReload blocks until any in-flight config reload has completed.
// Call after cancelling the watcher context to prevent reloads from racing shutdown.
func WaitConfigReload() {
	if configWatcherDone != nil {
		<-configWatcherDone
	}
}
