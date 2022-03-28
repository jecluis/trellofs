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
	"log"
	"time"
	"trellofs/trello"

	"github.com/jacobsa/fuse/fuseops"
)

type inode struct {
	attrs fuseops.InodeAttributes

	isRoot     bool
	closed     bool
	name       string
	entityType trello.EntityType

	lastUpdate time.Time
	isDirty    bool
}

func newRootInode(attrs fuseops.InodeAttributes) *inode {
	now := time.Now()
	attrs.Mtime = now
	attrs.Crtime = now
	attrs.Ctime = now

	return &inode{
		isRoot: true,
		attrs:  attrs,
		name:   "/",
	}
}

func (in *inode) isDir() bool {
	return (in.isRoot ||
		in.entityType == trello.TYPE_WORKSPACE ||
		in.entityType == trello.TYPE_BOARD ||
		in.entityType == trello.TYPE_LIST ||
		in.entityType == trello.TYPE_CARD)
}

func (in *inode) isSymlink() bool {
	return false
}

func (in *inode) isFile() bool {
	return false
}

func (in *inode) shouldUpdate() bool {
	delta := time.Since(in.lastUpdate)
	secs := delta.Seconds()
	doUpdate := in.isDirty || (secs > 60.0)
	if doUpdate {
		log.Printf(
			"should update inode for '%s', last update %f seconds ago",
			in.name, secs,
		)
	}
	return doUpdate
}

func (in *inode) update() {
	in.lastUpdate = time.Now()
}

func (in *inode) ReadDir(dst []byte, offset int) int {
	if !in.isDir() {
		log.Fatalf("read dir called on non-dir inode '%s'", in.name)
	}

	if in.shouldUpdate() {
		in.update()
	}

	return 0
}
