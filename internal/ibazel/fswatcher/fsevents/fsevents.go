// Copyright 2017 The Bazel Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build darwin
// +build darwin

package fsevents

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsevents"

	"github.com/bazelbuild/bazel-watcher/internal/ibazel/fswatcher/common"
)

type realFSEventsWatcher struct {
	es  *fsevents.EventStream
	evs chan common.Event
}

var _ common.Watcher = &realFSEventsWatcher{}

// Close implements ibazel/fswatcher/common.Watcher
func (w *realFSEventsWatcher) Close() error {
	w.es.Stop()
	close(w.es.Events)
	close(w.evs)
	return nil
}

// UpdateAll implements ibazel/fswatcher/common.Watcher
func (w *realFSEventsWatcher) UpdateAll(names []string) error {
	w.es.Stop()
	commonRoot, err := findCommonRoot(names)
	if err != nil {
		return err
	}
	es := &fsevents.EventStream{
		Events: make(chan []fsevents.Event),
		Paths:  commonRoot,
		Flags:  w.es.Flags,
	}
	w.es = es
	go w.MapEvents()
	es.Start()

	return nil
}

// Events implements ibazel/fswatcher/common.Watcher
func (w *realFSEventsWatcher) Events() chan common.Event {
	return w.evs
}
func (s *realFSEventsWatcher) MapEvents() {
	for events := range s.es.Events {
		for _, event := range events {
			if evt, ok := newEvent(event.Path, event.Flags); ok {
				s.evs <- evt
			}
		}
	}
}

func newEvent(name string, mask fsevents.EventFlags) (common.Event, bool) {
	e := common.Event{}

	if mask&fsevents.ItemIsFile != fsevents.ItemIsFile {
		return e, false
	}

	if mask&fsevents.ItemRemoved == fsevents.ItemRemoved {
		e.Op |= common.Remove
	}
	if mask&fsevents.ItemCreated == fsevents.ItemCreated {
		e.Op |= common.Create
	}
	if mask&fsevents.ItemRenamed == fsevents.ItemRenamed {
		e.Op |= common.Rename
	}
	if mask&fsevents.ItemModified == fsevents.ItemModified ||
		mask&fsevents.ItemInodeMetaMod == fsevents.ItemInodeMetaMod {
		e.Op |= common.Write
	}
	if mask&fsevents.ItemChangeOwner == fsevents.ItemChangeOwner ||
		mask&fsevents.ItemXattrMod == fsevents.ItemXattrMod {
		e.Op |= common.Chmod
	}

	e.Name = name
	return e, true
}

// Find the longest common root path of all directories to watch.
func findCommonRoot(names []string) ([]string, error) {
	if len(names) == 0 {
		return []string{}, nil
	}

	rootSplit := strings.Split(strings.Trim(names[0], "/"), "/")
	rootLength := len(rootSplit)

	for _, dir := range names {
		split := strings.Split(strings.Trim(dir, "/"), "/")
		commonLength := 0
		for i := 0; i < rootLength && i < len(split); i++ {
			if rootSplit[i] != split[i] {
				break
			}
			commonLength = i + 1
		}
		rootLength = commonLength
	}

	if rootLength == 0 {
		return nil, errors.New("could not find common root of directories")
	}

	return []string{"/" + filepath.Join(rootSplit[:rootLength]...) + "/"}, nil
}

func NewWatcher() (common.Watcher, error) {
	es := &fsevents.EventStream{
		Events: make(chan []fsevents.Event),
		Paths:  []string{},
		Flags:  fsevents.FileEvents,
	}
	watcher := &realFSEventsWatcher{
		es:  es,
		evs: make(chan common.Event),
	}
	return watcher, nil
}
