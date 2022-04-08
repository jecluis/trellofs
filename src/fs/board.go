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

type FSBoardCardsDirMeta struct {
	BaseFSNode

	BoardNode *FSBoard
}

func (node *FSBoardCardsDirMeta) ShouldUpdate() bool {
	return node.shouldUpdate(30.0)
}

func (node *FSBoardCardsDirMeta) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	boardNode := node.BoardNode

	log.Printf(
		"update cards for board %s (%s) id %d\n",
		boardNode.GetName(), boardNode.GetTrelloID(), boardNode.GetNodeID(),
	)

	board := boardNode.Board
	cards, err := board.GetCards(node.Ctx)
	if err != nil {
		log.Printf(
			"error updating cars for board %s (%s) id %d\n",
			boardNode.GetName(), boardNode.GetTrelloID(), boardNode.GetNodeID(),
		)
		return nil, nil, err
	}

	var newNodes []FSNode = make([]FSNode, 0)
	for _, card := range cards {
		log.Printf("==> card %s board nil: %t\n", card.Name, card.Board == nil)
		if _, exists := boardNode.ByCardID[card.ID]; exists {
			continue
		}

		newCard := &FSCard{
			BaseFSNode: BaseFSNode{
				name: card.Name,
				uid:  node.uid,
				gid:  node.gid,
				NodeAttrs: fuseops.InodeAttributes{
					Mode: 0700 | os.ModeDir,
					Uid:  node.uid,
					Gid:  node.gid,
				},
				isDir:    true,
				TrelloID: card.ID,
				Ctx:      node.Ctx,
			},
			Card:   &card,
			ByName: make(map[string]*FSCardMetaFile),
			ByID:   make(map[string]*FSCardMetaFile),
		}
		newNodes = append(newNodes, newCard)
		boardNode.Cards = append(boardNode.Cards, newCard)
		boardNode.ByCardID[card.ID] = newCard
		boardNode.ByCardName[card.Name] = newCard

		log.Printf(
			"new card on board %s (%s): %s (%s)\n",
			boardNode.GetName(), boardNode.GetTrelloID(),
			newCard.GetName(), newCard.GetTrelloID(),
		)
	}
	node.markUpdated()
	log.Printf(
		"updated cards for board %s (%s): %d new nodes, %d total cards\n",
		boardNode.GetName(), boardNode.GetTrelloID(),
		len(newNodes), len(boardNode.Cards),
	)

	return newNodes, nil, nil
}

func (node *FSBoardCardsDirMeta) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	for _, card := range node.BoardNode.Cards {
		if card.GetName() == name {
			return card, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSBoardCardsDirMeta) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"read dir %s/%s (%s) id %d, offset %d\n",
		node.BoardNode.GetName(),
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), offset,
	)
	var size int
	for i := offset; i < len(node.BoardNode.Cards); i++ {
		card := node.BoardNode.Cards[i]
		log.Printf("-> card ptr null: %t\n", card.Card == nil)
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   card.GetName(),
			Inode:  card.GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s (%s)\n",
				node.BoardNode.GetName(), node.BoardNode.GetTrelloID(),
			)
			break
		}
		log.Printf(
			"read dir %s/%s id %d: wrote direntry for %s (%s) id %d\n",
			node.BoardNode.GetName(), node.GetName(), node.GetNodeID(),
			card.GetName(), card.GetTrelloID(), card.GetNodeID(),
		)
		size += tmp
	}
	return size
}

type FSBoardListsDirMeta struct {
	BaseFSNode

	BoardNode *FSBoard
}

func (node *FSBoardListsDirMeta) ShouldUpdate() bool {
	return node.shouldUpdate(60.0)
}

func (node *FSBoardListsDirMeta) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"update lists for board %s (%s)\n",
		node.BoardNode.GetName(),
		node.BoardNode.GetTrelloID(),
	)

	board := node.BoardNode.Board
	lists, err := board.GetLists(node.BoardNode.Ctx)
	if err != nil {
		log.Printf(
			"error updating lists for board %s (%s)\n",
			node.BoardNode.GetName(),
			node.BoardNode.GetTrelloID(),
		)
		return nil, nil, err
	}

	log.Printf(
		"updating lists for board %s (%s)\n",
		node.BoardNode.GetName(),
		node.BoardNode.GetTrelloID(),
	)

	var newNodes []FSNode = make([]FSNode, 0)
	for _, list := range lists {
		if _, exists := node.BoardNode.ByListID[list.ID]; exists {
			continue
		}

		newList := &FSList{
			BaseFSNode: BaseFSNode{
				name: list.Name,
				uid:  node.uid,
				gid:  node.gid,
				NodeAttrs: fuseops.InodeAttributes{
					Mode: 0700 | os.ModeDir,
					Uid:  node.uid,
					Gid:  node.gid,
				},
				isDir:    true,
				TrelloID: list.ID,
				Ctx:      node.BoardNode.Ctx,
			},
			ByID:      make(map[string]*FSCard),
			ByName:    make(map[string]*FSCard),
			BoardNode: node.BoardNode,
			List:      &list,
		}
		newNodes = append(newNodes, newList)
		node.BoardNode.Lists = append(node.BoardNode.Lists, newList)
		node.BoardNode.ByListID[list.ID] = newList
		node.BoardNode.ByListName[list.Name] = newList

		log.Printf(
			"new list %s (%s) on board %s (%s)\n",
			newList.GetName(), newList.GetTrelloID(),
			node.BoardNode.GetName(), node.BoardNode.GetTrelloID(),
		)
	}
	node.markUpdated()
	log.Printf(
		"updated lists for board %s (%s): %d new nodes, %d total lists\n",
		node.BoardNode.GetName(), node.BoardNode.GetTrelloID(),
		len(newNodes), len(node.BoardNode.Lists),
	)

	return newNodes, nil, nil
}

func (node *FSBoardListsDirMeta) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	for _, list := range node.BoardNode.Lists {
		if list.GetName() == name {
			return list, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSBoardListsDirMeta) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"read dir %s/%s (%s) id %d, offset %d\n",
		node.BoardNode.GetName(),
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), offset,
	)
	var size int
	for i := offset; i < len(node.BoardNode.Lists); i++ {
		list := node.BoardNode.Lists[i]
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   list.GetName(),
			Inode:  list.GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s (%s)\n",
				node.BoardNode.GetName(), node.BoardNode.GetTrelloID(),
			)
			break
		}
		log.Printf(
			"read dir %s/%s id %d: wrote direntry for %s (%s) id %d\n",
			node.BoardNode.GetName(), node.GetName(), node.GetNodeID(),
			list.GetName(), list.GetTrelloID(), list.GetNodeID(),
		)
		size += tmp
	}
	return size
}

type FSBoard struct {
	BaseFSNode

	MetaCardsDir *FSBoardCardsDirMeta
	MetaListsDir *FSBoardListsDirMeta

	Cards      []*FSCard
	ByCardID   map[string]*FSCard
	ByCardName map[string]*FSCard

	Lists      []*FSList
	ByListID   map[string]*FSList
	ByListName map[string]*FSList

	Board *trello.Board
}

func (node *FSBoard) ShouldUpdate() bool {
	return node.shouldUpdate(30.0)
}

func (node *FSBoard) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	log.Printf(
		"update board %s (%s)\n",
		node.Board.Name, node.Board.ID,
	)

	var newNodes []FSNode = make([]FSNode, 0)
	if node.MetaCardsDir != nil && node.MetaListsDir != nil {
		return newNodes, nil, nil
	}

	node.MetaCardsDir = &FSBoardCardsDirMeta{
		BaseFSNode: BaseFSNode{
			name: "cards",
			uid:  node.uid,
			gid:  node.gid,
			NodeAttrs: fuseops.InodeAttributes{
				Mode: 0700 | os.ModeDir,
				Uid:  node.uid,
				Gid:  node.gid,
			},
			isDir:    true,
			TrelloID: fmt.Sprintf("%s/cards", node.GetTrelloID()),
			Ctx:      node.Ctx,
		},
		BoardNode: node,
	}
	node.MetaListsDir = &FSBoardListsDirMeta{
		BaseFSNode: BaseFSNode{
			name: "lists",
			uid:  node.uid,
			gid:  node.gid,
			NodeAttrs: fuseops.InodeAttributes{
				Mode: 0700 | os.ModeDir,
				Uid:  node.uid,
				Gid:  node.gid,
			},
			isDir:    true,
			TrelloID: fmt.Sprintf("%s/lists", node.GetTrelloID()),
			Ctx:      node.Ctx,
		},
		BoardNode: node,
	}
	newNodes = append(newNodes, node.MetaCardsDir, node.MetaListsDir)
	node.markUpdated()
	log.Printf(
		"updated board %s (%s)", node.Board.Name, node.Board.ID,
	)
	return newNodes, nil, nil
}

func (node *FSBoard) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	var err error = fuse.ENOENT
	var child FSNode = nil

	log.Printf(
		"board %s (%s) id %d lookup child %s\n",
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), name,
	)

	if name == "lists" {
		child = node.MetaListsDir
		err = nil
	} else if name == "cards" {
		child = node.MetaCardsDir
		err = nil
	}
	return child, err
}

func (node *FSBoard) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	fmt.Printf(
		"read dir board %s (%s) id %d, offset %d\n",
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), offset,
	)

	var entries []FSNode = make([]FSNode, 2)
	entries[0] = node.MetaCardsDir
	entries[1] = node.MetaListsDir

	var size int
	for i := offset; i < len(entries); i++ {
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   entries[i].GetName(),
			Inode:  entries[i].GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir board > no more space to write dirent for %s\n",
				entries[i].GetName(),
			)
			break
		}
		size += tmp
	}
	return size
}
