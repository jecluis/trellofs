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
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"trellofs/trello"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type InodeRef struct {
	ID    string
	Inode *inode
}

type inode struct {
	attrs   fuseops.InodeAttributes
	InodeID fuseops.InodeID

	isRoot     bool
	closed     bool
	name       string
	entityType trello.EntityType
	trelloID   string

	lastUpdate time.Time
	isDirty    bool

	children     []*inode
	childrenByID map[string]*inode
	dentries     []fuseutil.Dirent

	lock sync.Mutex
}

func newRootInode(attrs fuseops.InodeAttributes) *inode {
	now := time.Now()
	attrs.Mtime = now
	attrs.Crtime = now
	attrs.Ctime = now

	return &inode{
		isRoot:       true,
		attrs:        attrs,
		name:         "/",
		childrenByID: map[string]*inode{},
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

		var tmpChildren []*inode
		var newChildren []InodeRef
		for _, ws := range workspaces {

			if in.childrenByID[ws.ID] != nil {
				log.Printf(
					"update > workspace %s (%s) already exists, skip",
					ws.DisplayName,
					ws.ID,
				)
				continue
			}

			newInode := &inode{
				isRoot: false,
				attrs: fuseops.InodeAttributes{
					Nlink:  1,
					Mode:   0700 | os.ModeDir,
					Uid:    in.attrs.Uid,
					Gid:    in.attrs.Gid,
					Mtime:  in.lastUpdate,
					Crtime: in.lastUpdate,
					Ctime:  in.lastUpdate,
				},
				name:         ws.Name,
				entityType:   trello.TYPE_WORKSPACE,
				trelloID:     ws.ID,
				childrenByID: map[string]*inode{},
			}
			newChildren = append(newChildren, InodeRef{
				ID:    ws.ID,
				Inode: newInode,
			})
			in.childrenByID[ws.ID] = newInode
			tmpChildren = append(tmpChildren, newInode)
		}
		in.children = tmpChildren
		return newChildren, nil, nil
	}
	return nil, nil, nil
}

func (in *inode) ReadDir(
	dst []byte,
	offset int,
	inodeByID map[string]fuseops.InodeID,
) int {
	if !in.isDir() {
		log.Fatalf("read dir called on non-dir inode '%s'", in.name)
	}

	fmt.Printf("read dir > offset %d, num children: %d\n",
		offset, len(in.children))
	var size int
	for i := offset; i < len(in.children); i++ {
		child := in.children[i]
		entryType := fuseutil.DT_Unknown
		if child.entityType == trello.TYPE_WORKSPACE {
			entryType = fuseutil.DT_Directory
		}
		entry := fuseutil.Dirent{
			Name:   child.name,
			Inode:  inodeByID[child.trelloID],
			Type:   entryType,
			Offset: fuseops.DirOffset(i + 1),
		}

		tmp := fuseutil.WriteDirent(dst[size:], entry)
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s", child.name,
			)
			break
		}
		size += tmp
		log.Printf("read dir > wrote direntry for %s (%s)", child.name, child.trelloID)
	}
	return size
}

func (in *inode) LookupChild(name string) (*inode, error) {
	in.lock.Lock()
	defer in.lock.Unlock()

	for _, child := range in.children {
		if child.name == name {
			return child, nil
		}
	}
	return nil, fuse.ENOENT
}
