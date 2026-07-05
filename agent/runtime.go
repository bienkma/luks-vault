package agent

import (
	"log"
	"sync"

	"github.com/bienkma/luks-vault/config"
)

type runtime struct {
	mu     sync.RWMutex
	cfg    *config.Configuration
	reload chan struct{}
}

func newRuntime(cfg *config.Configuration) *runtime {
	return &runtime{
		cfg:    cfg,
		reload: make(chan struct{}, 1),
	}
}

func (r *runtime) Config() *config.Configuration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

func (r *runtime) Reload() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.cfg = cfg
	r.mu.Unlock()

	select {
	case r.reload <- struct{}{}:
	default:
	}

	log.Println("configuration reloaded")
	return nil
}
