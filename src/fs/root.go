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

	"github.com/jecluis/trellofs/src/trello"

	"github.com/jacobsa/fuse"
	"github.com/jacobsa/fuse/fuseops"
	"github.com/jacobsa/fuse/fuseutil"
)

type TrelloTreeRoot struct {
	BaseFSNode

	workspaces []*FSWorkspace
	byID       map[string]*FSWorkspace
	byName     map[string]*FSWorkspace
}

func (node *TrelloTreeRoot) ShouldUpdate() bool {
	return node.shouldUpdate(60.0)
}

func (node *TrelloTreeRoot) Update() ([]FSNode, []FSNode, error) {

	node.Lock()
	defer node.Unlock()

	workspaces, err := trello.GetWorkspaces(node.Ctx)
	if err != nil {
		log.Printf("error updating workspaces for root node: %s\n", err)
		return nil, nil, err
	}

	var newNodes []FSNode = make([]FSNode, 0)
	for i, ws := range workspaces {
		if _, exists := node.byID[ws.ID]; exists {
			continue
		}

		newItem := &FSWorkspace{
			BaseFSNode: BaseFSNode{
				name: ws.Name,
				uid:  node.uid,
				gid:  node.gid,
				NodeAttrs: fuseops.InodeAttributes{
					Mode: 0700 | os.ModeDir,
					Uid:  node.uid,
					Gid:  node.gid,
				},
				isDir:    true,
				TrelloID: ws.ID,
				Ctx:      node.Ctx,
			},
			ByID:      make(map[string]*FSBoard),
			ByName:    make(map[string]*FSBoard),
			Workspace: &workspaces[i],
		}
		newNodes = append(newNodes, newItem)
		node.byID[ws.ID] = newItem
		node.byName[ws.Name] = newItem
		node.workspaces = append(node.workspaces, newItem)
		log.Printf(
			"update root: workspace %s (%s)\n",
			ws.Name, ws.ID,
		)
	}
	for _, ws := range node.workspaces {
		log.Printf(
			"debug > workspace for root: %s (%s)\n",
			ws.GetName(), ws.GetTrelloID(),
		)
	}
	node.markUpdated()
	return newNodes, nil, nil
}

func (node *TrelloTreeRoot) LookupChild(name string) (FSNode, error) {

	node.Lock()
	defer node.Unlock()

	for _, workspace := range node.workspaces {
		if workspace.GetName() == name {
			return workspace, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *TrelloTreeRoot) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	fmt.Printf(
		"read dir %s (%s) id %d, offset %d\n",
		node.GetName(),
		node.GetTrelloID(),
		node.GetNodeID(),
		offset,
	)
	var size int
	for i := offset; i < len(node.workspaces); i++ {
		ws := node.workspaces[i]
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   ws.name,
			Inode:  ws.GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s\n", ws.name,
			)
			break
		}
		size += tmp
	}
	return size
}
