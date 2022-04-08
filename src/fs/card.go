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
	"io"
	"log"
	"trellofs/trello"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type FSCardMetaFile struct {
	BaseFSNode

	contents []byte

	Card *trello.Card
}

func (node *FSCardMetaFile) ShouldUpdate() bool {
	return false
}

func (node *FSCardMetaFile) Update() ([]FSNode, []FSNode, error) {
	return nil, nil, fuse.EINVAL
}

func (node *FSCardMetaFile) LookupChild(name string) (FSNode, error) {
	return nil, fuse.ENOENT
}

func (node *FSCardMetaFile) ReadDir(dst []byte, offset int) int {
	return 0
}

func (node *FSCardMetaFile) ReadAt(dst []byte, offset int64) (int, error) {

	log.Printf(
		"read file %s/%s meta %s, offset %d, len %d\n",
		node.Card.Board.Name,
		node.Card.Name,
		node.GetName(),
		offset, len(node.contents),
	)

	if offset > int64(len(node.contents)) {
		return 0, io.EOF
	}

	n := copy(dst, node.contents[offset:])
	if n < len(dst) {
		return n, io.EOF
	}

	return n, nil
}

type FSCard struct {
	BaseFSNode

	MetaFiles []*FSCardMetaFile
	ByName    map[string]*FSCardMetaFile
	ByID      map[string]*FSCardMetaFile
	Card      *trello.Card
}

func (node *FSCard) ShouldUpdate() bool {
	return node.shouldUpdate(30.0)
}

func (node *FSCard) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	board := node.Card.Board
	log.Printf(
		"update meta for card %s (%s) on board %s (%s)\n",
		node.GetName(), node.GetTrelloID(),
		board.Name, board.ID,
	)

	var newNodes []FSNode = make([]FSNode, 0)
	meta := getMeta(*node.Card)
	for _, entry := range meta {
		log.Printf(
			"card meta name: %s, value: %s\n",
			entry.Name, string(entry.Contents),
		)
		if _, exists := node.ByName[entry.Name]; exists {
			continue
		}
		trelloID := fmt.Sprintf("%s/_meta/%s", node.GetTrelloID(), entry.Name)
		metaFile := &FSCardMetaFile{
			BaseFSNode: BaseFSNode{
				name: entry.Name,
				uid:  node.uid,
				gid:  node.gid,
				NodeAttrs: fuseops.InodeAttributes{
					Mode:  0600,
					Nlink: 1,
					Uid:   node.uid,
					Gid:   node.gid,
					Size:  uint64(len(entry.Contents)),
				},
				isDir:    false,
				TrelloID: trelloID,
			},
			contents: entry.Contents,
			Card:     node.Card,
		}
		newNodes = append(newNodes, metaFile)
		node.MetaFiles = append(node.MetaFiles, metaFile)
		node.ByName[entry.Name] = metaFile
		node.ByID[trelloID] = metaFile
	}

	return newNodes, nil, nil
}

func (node *FSCard) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	for _, entry := range node.MetaFiles {
		if entry.GetName() == name {
			return entry, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSCard) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"read dir %s/%s (%s), offset %d\n",
		node.Card.Board.Name,
		node.GetName(), node.GetTrelloID(),
		offset,
	)
	var size int
	for i := offset; i < len(node.MetaFiles); i++ {
		entry := node.MetaFiles[i]
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   entry.GetName(),
			Inode:  entry.GetNodeID(),
			Type:   fuseutil.DT_File,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s/%s (%s)\n",
				node.Card.Board.Name, node.GetName(), node.GetTrelloID(),
			)
			break
		}
		log.Printf(
			"read dir %s/%s id %d: wrote direntry for %s (%s) id %d\n",
			node.Card.Board.Name, node.GetName(), node.GetNodeID(),
			entry.GetName(), entry.GetTrelloID(), entry.GetNodeID(),
		)
		size += tmp
	}
	return size
}
