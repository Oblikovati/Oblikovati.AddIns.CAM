//go:build js && wasm

// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wasmaddin"
	"oblikovati.org/cam/bridge"
)

const addInID = "com.oblikovati.cam"

type wasmCAM struct {
	mu     sync.Mutex
	engine *bridge.Engine
}

func (a *wasmCAM) ID() string       { return addInID }
func (a *wasmCAM) Manifest() string { return manifestJSON }

func (a *wasmCAM) Activate(caller client.Caller) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	eng := bridge.NewEngine(caller)
	a.engine = eng
	go func() { _ = eng.Setup() }()
	return nil
}

func (a *wasmCAM) Deactivate() error {
	a.mu.Lock()
	a.engine = nil
	a.mu.Unlock()
	return nil
}

func (a *wasmCAM) Notify(event []byte) {
	a.mu.Lock()
	eng := a.engine
	a.mu.Unlock()
	if eng != nil {
		eng.Notify(event)
	}
}

func main() {
	wasmaddin.Serve(&wasmCAM{})
}
