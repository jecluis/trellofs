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

	"github.com/jecluis/trellofs/src/trello"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type FSWorkspace struct {
	BaseFSNode

	Boards []*FSBoard
	ByID   map[string]*FSBoard
	ByName map[string]*FSBoard

	Workspace *trello.Workspace
}

func (node *FSWorkspace) ShouldUpdate() bool {
	return node.shouldUpdate(60.0)
}

func (node *FSWorkspace) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"update workspace %s (%s)\n",
		node.Workspace.Name, node.Workspace.ID,
	)

	boards, err := node.Workspace.GetBoards(node.Ctx)
	if err != nil {
		log.Printf(
			"error updating boards for workspace %s: %s\n",
			node.GetName(),
			err,
		)
		return nil, nil, err
	}

	log.Printf(
		"updating workspace %s (%s): %d total boards available\n",
		node.name, node.TrelloID, len(boards),
	)

	var newNodes []FSNode = make([]FSNode, 0)
	for i, board := range boards {
		if _, exists := node.ByID[board.ID]; exists {
			continue
		}

		newItem := &FSBoard{
			BaseFSNode: BaseFSNode{
				name: board.Name,
				uid:  node.uid,
				gid:  node.gid,
				NodeAttrs: fuseops.InodeAttributes{
					Mode: 0700 | os.ModeDir,
					Uid:  node.uid,
					Gid:  node.gid,
				},
				isDir:    true,
				TrelloID: board.ID,
				Ctx:      node.Ctx,
			},
			ByCardID:   make(map[string]*FSCard),
			ByCardName: make(map[string]*FSCard),
			ByListID:   make(map[string]*FSList),
			ByListName: make(map[string]*FSList),
			Board:      &boards[i],
		}
		newNodes = append(newNodes, newItem)
		node.ByID[board.ID] = newItem
		node.ByName[board.Name] = newItem
		node.Boards = append(node.Boards, newItem)
	}
	node.markUpdated()
	log.Printf(
		"updated workspace %s (%s): %d new nodes, %d total boards\n",
		node.name, node.TrelloID, len(newNodes), len(node.Boards),
	)
	return newNodes, nil, nil
}

func (node *FSWorkspace) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	for _, board := range node.Boards {
		if board.name == name {
			return board, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSWorkspace) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"read dir %s (%s) id %d, offset %d\n",
		node.GetName(),
		node.GetTrelloID(),
		node.GetNodeID(),
		offset,
	)
	var size int
	for i := offset; i < len(node.Boards); i++ {
		board := node.Boards[i]
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   board.name,
			Inode:  board.GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s\n", board.name,
			)
			break
		}
		size += tmp
	}
	return size
}
