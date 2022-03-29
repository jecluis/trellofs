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
	"os"
	"sync"
	"time"
	"trellofs/trello"

	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type InodeRef struct {
	ID    string
	Inode *inode
}

type inode struct {
	attrs fuseops.InodeAttributes

	isRoot     bool
	closed     bool
	name       string
	entityType trello.EntityType
	trelloID   string

	lastUpdate time.Time
	isDirty    bool

	children     []*inode
	childrenByID map[string]*inode

	lock sync.Mutex
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

func (in *inode) ShouldUpdate() bool {
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

func (in *inode) Update(
	ctx *trello.TrelloCtx,
) ([]InodeRef, []InodeRef, error) {

	in.lock.Lock()
	defer in.lock.Unlock()

	in.lastUpdate = time.Now()

	if in.isRoot {
		workspaces, err := trello.GetWorkspaces(ctx)
		if err != nil {
			log.Printf("error updating workspaces for root dir: %s", err)
			return nil, nil, err
		}

		childrenByID := map[string]*inode{}
		var children []*inode
		for _, ws := range workspaces {
			newInode := &inode{
				isRoot: false,
				attrs: fuseops.InodeAttributes{
					Mode:   0700 | os.ModeDir,
					Uid:    in.attrs.Uid,
					Gid:    in.attrs.Gid,
					Mtime:  in.lastUpdate,
					Crtime: in.lastUpdate,
					Ctime:  in.lastUpdate,
				},
				name:       ws.Name,
				entityType: trello.TYPE_WORKSPACE,
				trelloID:   ws.ID,
			}
			childrenByID[ws.ID] = newInode
			children = append(children, newInode)
		}

		/*
			var exists []trello.Workspace
			var toCreate []trello.Workspace
			for _, ws := range workspaces {
				if in.childrenByID[ws.ID] == nil {
					toCreate = append(toCreate, ws)
				} else {
					exists = append(exists, ws)
				}
			}

			contains := func(arr []trello.Workspace, id string) bool {
				found := false
				for _, ws := range arr {
					if ws.ID == id {
						found := true
						break
					}
				}
				return found
			}

			children := map[string]*inode{}
			var newChildren []InodeRef
			var removedChildren []InodeRef

			for id, entry := range in.childrenByID {

			}
		*/

	}
}

func (in *inode) ReadDir(
	dst []byte,
	offset int,
	ctx *trello.TrelloCtx,
) int {
	if !in.isDir() {
		log.Fatalf("read dir called on non-dir inode '%s'", in.name)
	}

	var sz int
	for i := offset; i < len(in.entries); i++ {
		e := in.entries[i]
		if e.Type == fuseutil.DT_Unknown {
			continue
		}
		tmp := fuseutil.WriteDirent(dst[sz:], in.entries[i])
		if tmp == 0 {
			continue
		}
		sz += tmp
	}

	return sz
}
