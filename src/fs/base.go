/*
 * trellofs - A Trello POSIX filesystem
 * Copyright (C) 2022  Joao Eduardo Luis <joao@wipwd.dev>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 */
package fs

import (
	"sync"
	"time"

	"github.com/jecluis/trellofs/src/trello"

	"github.com/jacobsa/fuse/fuseops"
)

type BaseFSNode struct {
	lock sync.Mutex

	name string

	uid uint32
	gid uint32

	NodeID    fuseops.InodeID
	NodeAttrs fuseops.InodeAttributes

	isDir    bool
	TrelloID string

	lastUpdate time.Time

	Ctx *trello.TrelloCtx
}

func (base *BaseFSNode) Lock() {
	base.lock.Lock()
}

func (base *BaseFSNode) Unlock() {
	base.lock.Unlock()
}

func (base *BaseFSNode) GetName() string {
	return base.name
}

func (base *BaseFSNode) GetNodeID() fuseops.InodeID {
	return base.NodeID
}

func (base *BaseFSNode) GetNodeAttrs() fuseops.InodeAttributes {
	return base.NodeAttrs
}

func (base *BaseFSNode) GetTrelloID() string {
	return base.TrelloID
}

func (base *BaseFSNode) SetNodeID(id fuseops.InodeID) {
	base.NodeID = id
}

func (base *BaseFSNode) getLastUpdated() time.Time {
	return base.lastUpdate
}

func (base *BaseFSNode) markUpdated() {
	base.lastUpdate = time.Now()
}

func (base *BaseFSNode) shouldUpdate(interval float64) bool {
	base.Lock()
	defer base.Unlock()
	delta := time.Since(base.lastUpdate)
	secs := delta.Seconds()
	return secs >= interval
}

func (base *BaseFSNode) ReadAt(dst []byte, offset int64) (int, error) {
	return 0, nil
}
