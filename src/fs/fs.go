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
	log.Printf("lookup inode %s\n", op.Name)
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
	op.BytesRead = inode.ReadDir(op.Dst, int(op.Offset))
	return nil
}
