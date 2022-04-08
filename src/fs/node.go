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

import "github.com/jacobsa/fuse/fuseops"

type FSNode interface {
	Lock()
	Unlock()

	ShouldUpdate() bool
	Update() ([]FSNode, []FSNode, error) // (new, removed, error)
	GetName() string
	GetTrelloID() string
	GetNodeID() fuseops.InodeID
	GetNodeAttrs() fuseops.InodeAttributes
	SetNodeID(fuseops.InodeID)

	LookupChild(string) (FSNode, error)

	ReadDir([]byte, int) int
	ReadAt([]byte, int64) (int, error)
}
