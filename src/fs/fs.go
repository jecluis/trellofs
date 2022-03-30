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
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
	"trellofs/trello"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
	"github.com/jacobsa/timeutil"
)

type trelloFS struct {
	fuseutil.NotImplementedFileSystem

	uid uint32
	gid uint32

	lock sync.Mutex

	inodes     []*inode
	freeInodes []fuseops.InodeID
	byID       map[string]fuseops.InodeID

	Clock timeutil.Clock

	ctx *trello.TrelloCtx
}

func NewTrelloFS(
	uid uint32,
	gid uint32,
	ctx *trello.TrelloCtx,
) (fuse.Server, error) {
	fs := &trelloFS{
		uid:    uid,
		gid:    gid,
		inodes: make([]*inode, fuseops.RootInodeID+1),
		byID:   make(map[string]fuseops.InodeID),
		Clock:  timeutil.RealClock(),
		ctx:    ctx,
	}

	rootAttrs := fuseops.InodeAttributes{
		Mode: 0700 | os.ModeDir,
		Uid:  uid,
		Gid:  gid,
	}
	fs.inodes[fuseops.RootInodeID] = newRootInode(rootAttrs)

	return fuseutil.NewFileSystemServer(fs), nil
}

func (fs *trelloFS) insertInode(new InodeRef) fuseops.InodeID {
	numFree := len(fs.freeInodes)
	id := fuseops.InodeID(len(fs.inodes))
	if numFree > 0 {
		id = fs.freeInodes[numFree-1]
		fs.freeInodes = fs.freeInodes[:numFree-1]
		fs.inodes[id] = new.Inode
	} else {
		fs.inodes = append(fs.inodes, new.Inode)
	}
	fs.byID[new.ID] = id
	log.Printf(
		"added new inode %s (%s) id %d\n",
		new.Inode.name,
		new.Inode.trelloID,
		id,
	)
	return id
}

func (fs *trelloFS) StatFS(
	ctx context.Context,
	op *fuseops.StatFSOp,
) error {
	log.Println("statfs not implemented")
	return nil
}

func (fs *trelloFS) LookUpInode(
	ctx context.Context,
	op *fuseops.LookUpInodeOp,
) error {
	log.Printf("lookup inode %s, parent id %d\n", op.Name, op.Parent)
	if op.OpContext.Pid == 0 {
		return fuse.EINVAL
	}

	fs.lock.Lock()
	defer fs.lock.Unlock()

	parent := fs.inodes[op.Parent]
	if parent == nil {
		log.Fatalf(
			"lookup inode %s, parent id %d not found\n", op.Name, op.Parent,
		)
	}

	child, err := parent.LookupChild(op.Name)
	if err != nil {
		log.Printf("lookup inode %s, parent id %d\n", op.Name, op.Parent)
		return fuse.ENOENT
	}
	childId, ok := fs.byID[child.trelloID]
	if !ok {
		log.Fatalf(
			"lookup inode > unable to find child id for %s (%s)",
			op.Name,
			child.trelloID,
		)
	}
	childInode := fs.inodes[childId]
	op.Entry.Child = childId
	op.Entry.Attributes = childInode.attrs
	op.Entry.AttributesExpiration = time.Now().Add(365 * 24 * time.Hour)
	op.Entry.EntryExpiration = op.Entry.AttributesExpiration

	return nil
}

func (fs *trelloFS) GetInodeAttributes(
	ctx context.Context,
	op *fuseops.GetInodeAttributesOp,
) error {
	log.Printf("get inode attrs %d\n", op.Inode)
	if op.OpContext.Pid == 0 {
		return fuse.EINVAL
	}

	fs.lock.Lock()
	defer fs.lock.Unlock()
	op.Attributes = fs.inodes[op.Inode].attrs
	op.AttributesExpiration = time.Now().Add(365 * 24 * time.Hour)
	return nil
}

func (fs *trelloFS) SetInodeAttributes(
	ctx context.Context,
	op *fuseops.SetInodeAttributesOp,
) error {
	log.Printf("set inode attrs %d\n", op.Inode)
	if op.OpContext.Pid == 0 {
		return fuse.EINVAL
	}
	return fuse.EIO
}

func (fs *trelloFS) OpenDir(
	ctx context.Context,
	op *fuseops.OpenDirOp,
) error {
	log.Printf("open dir %d\n", op.Inode)
	return nil
}

func (fs *trelloFS) ReadDir(
	ctx context.Context,
	op *fuseops.ReadDirOp,
) error {
	log.Printf("read dir %d\n", op.Inode)

	fs.lock.Lock()
	defer fs.lock.Unlock()

	inode := fs.inodes[op.Inode]
	if inode == nil {
		log.Fatalf(fmt.Sprintf("unknown inode %d", op.Inode))
	}
	if inode.ShouldUpdate() {
		fmt.Printf("updating inode %s (%s) id %d\n",
			inode.name, inode.trelloID, op.Inode)
		new, _, err := inode.Update(fs.ctx)
		if err != nil {
			log.Printf("unable to update inode %d: %s\n", op.Inode, err)
			return nil
		}
		for _, ref := range new {
			log.Printf(
				"update inode %d: insert child %s (%s)\n",
				op.Inode,
				ref.Inode.name,
				ref.ID,
			)
			id := fs.insertInode(ref)
			ref.Inode.InodeID = id
		}
	}
	op.BytesRead = inode.ReadDir(op.Dst, int(op.Offset), fs.byID)
	log.Printf("read dir %d > %s (bytes read: %d) \n", op.Inode, string(op.Dst), op.BytesRead)
	return nil
}
