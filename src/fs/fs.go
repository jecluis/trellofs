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
	"errors"
	"io"
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

	Root *TrelloTreeRoot

	uid uint32
	gid uint32

	lock sync.Mutex

	inodes     []FSNode
	freeInodes []fuseops.InodeID
	byID       map[string]fuseops.InodeID

	Clock timeutil.Clock

	ctx *trello.TrelloCtx
}

func (fs *trelloFS) initRoot() FSNode {

	rootAttrs := fuseops.InodeAttributes{
		Mode: 0700 | os.ModeDir,
		Uid:  fs.uid,
		Gid:  fs.gid,
	}
	fs.Root = &TrelloTreeRoot{
		BaseFSNode: BaseFSNode{
			name:      "/",
			uid:       fs.uid,
			gid:       fs.gid,
			NodeID:    fuseops.RootInodeID,
			NodeAttrs: rootAttrs,
			isDir:     true,
			TrelloID:  "rootID",
			Ctx:       fs.ctx,
		},
		byID:   make(map[string]*FSWorkspace),
		byName: make(map[string]*FSWorkspace),
	}
	return fs.Root
}

func NewTrelloFS(
	uid uint32,
	gid uint32,
	ctx *trello.TrelloCtx,
) (fuse.Server, error) {
	fs := &trelloFS{
		uid:    uid,
		gid:    gid,
		inodes: make([]FSNode, fuseops.RootInodeID+1),
		byID:   make(map[string]fuseops.InodeID),
		Clock:  timeutil.RealClock(),
		ctx:    ctx,
	}
	fs.inodes[fuseops.RootInodeID] = fs.initRoot()
	return fuseutil.NewFileSystemServer(fs), nil
}

func (fs *trelloFS) refreshNode(node FSNode) {

	if !node.ShouldUpdate() {
		return
	}
	log.Printf(
		"refreshing node id %d, %s (%s)\n",
		node.GetNodeID(), node.GetName(), node.GetTrelloID(),
	)
	add, rm, err := node.Update()

	if err != nil {
		log.Printf(
			"error updating node %s (%s) id %d\n",
			node.GetName(),
			node.GetTrelloID(),
			node.GetNodeID(),
		)
		return
	}

	for _, n := range add {
		numFree := len(fs.freeInodes)
		id := fuseops.InodeID(len(fs.inodes))
		if numFree > 0 {
			id = fs.freeInodes[numFree-1]
			log.Printf(
				"refresh > reuse id %d for %s (%s)\n",
				id, n.GetName(), n.GetTrelloID(),
			)
			fs.freeInodes = fs.freeInodes[:numFree-1]
			fs.inodes[id] = n
		} else {
			fs.inodes = append(fs.inodes, n)
		}
		fs.byID[n.GetTrelloID()] = id
		n.SetNodeID(id)
		log.Printf(
			"added new node %s (%s) id %d\n",
			n.GetName(),
			n.GetTrelloID(),
			n.GetNodeID(),
		)
	}

	for _, n := range rm {
		log.Printf(
			"not implemented: remove node %s (%s) id %d\n",
			n.GetName(),
			n.GetTrelloID(),
			n.GetNodeID(),
		)
	}

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
		return fuse.ENOENT
	}

	fs.refreshNode(parent)

	child, err := parent.LookupChild(op.Name)
	if err != nil {
		log.Printf(
			"lookup inode %s, parent id %d, not found\n",
			op.Name, op.Parent,
		)
		return fuse.ENOENT
	}
	op.Entry.Child = child.GetNodeID()
	op.Entry.Attributes = child.GetNodeAttrs()
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
	op.Attributes = fs.inodes[op.Inode].GetNodeAttrs()
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
	log.Printf("read dir > id %d\n", op.Inode)

	fs.lock.Lock()
	defer fs.lock.Unlock()

	parent := fs.inodes[op.Inode]
	if parent == nil {
		log.Printf("read dir > failed to find parent inode %d\n", op.Inode)
		return fuse.ENOENT
	}
	log.Printf(
		"read dir > id %d, %s (%s)\n",
		parent.GetNodeID(), parent.GetName(), parent.GetTrelloID(),
	)

	fs.refreshNode(parent)
	op.BytesRead = parent.ReadDir(op.Dst, int(op.Offset))

	log.Printf(
		"read dir %d > %s (bytes read: %d)\n",
		op.Inode,
		string(op.Dst),
		op.BytesRead,
	)
	return nil
}

func (fs *trelloFS) OpenFile(
	ctx context.Context,
	op *fuseops.OpenFileOp,
) error {
	log.Printf("open file > id %d\n", op.Inode)
	return nil
}

func (fs *trelloFS) ReadFile(
	ctx context.Context,
	op *fuseops.ReadFileOp,
) error {
	log.Printf("read file > id %d\n", op.Inode)

	if op.OpContext.Pid == 0 {
		return fuse.EINVAL
	}

	fs.lock.Lock()
	defer fs.lock.Unlock()

	if int(op.Inode) >= len(fs.inodes) {
		panic(errors.New("Inode does not exist"))
	}

	node := fs.inodes[op.Inode]
	bytes, err := node.ReadAt(op.Dst, op.Offset)

	log.Printf(
		"read file > read %s (%s) id %d, bytes: %d\n",
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), bytes,
	)
	op.BytesRead = bytes
	if err == io.EOF {
		return nil
	}
	return err
}
