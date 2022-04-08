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

type FSList struct {
	BaseFSNode

	Cards  []*FSCard
	ByID   map[string]*FSCard
	ByName map[string]*FSCard

	BoardNode *FSBoard
	List      *trello.List
}

func (node *FSList) ShouldUpdate() bool {
	return node.shouldUpdate(30.0)
}

func (node *FSList) Update() ([]FSNode, []FSNode, error) {
	node.Lock()
	defer node.Unlock()

	boardNode := node.BoardNode

	log.Printf(
		"update cards for list %s (%s) on board %s (%s)\n",
		node.GetName(), node.GetTrelloID(),
		boardNode.GetName(), boardNode.GetTrelloID(),
	)

	cards, err := node.List.GetCards(node.Ctx)
	if err != nil {
		log.Printf(
			"error upating cards for list %s (%s) on board %s (%s): %s\n",
			node.GetName(), node.GetTrelloID(),
			boardNode.GetName(), boardNode.GetTrelloID(),
			err,
		)
		return nil, nil, err
	}

	log.Printf(
		"updating cards for list %s (%s) on board %s (%s)\n",
		node.GetName(), node.GetTrelloID(),
		boardNode.GetName(), boardNode.GetTrelloID(),
	)

	var newNodes []FSNode = make([]FSNode, 0)
	for _, card := range cards {
		var newCard *FSCard = nil
		if _, exists := boardNode.ByCardID[card.ID]; exists {
			newCard = boardNode.ByCardID[card.ID]
			log.Printf(
				"reusing card on board %s (%s) for list %s (%s): %s (%s)\n",
				boardNode.GetName(), boardNode.GetTrelloID(),
				node.GetName(), node.GetTrelloID(),
				newCard.GetName(), newCard.GetTrelloID(),
			)
		} else {
			newCard = &FSCard{
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
			log.Printf(
				"new card %s (%s) on list %s (%s) for board %s (%s)\n",
				newCard.GetName(), newCard.GetTrelloID(),
				node.GetName(), node.GetTrelloID(),
				boardNode.GetName(), boardNode.GetTrelloID(),
			)
		}
		if _, exists := node.ByID[card.ID]; !exists {
			node.Cards = append(node.Cards, newCard)
			node.ByID[card.ID] = newCard
			node.ByName[card.Name] = newCard
			boardNode.Cards = append(boardNode.Cards, newCard)
			boardNode.ByCardID[card.ID] = newCard
			boardNode.ByCardName[card.Name] = newCard
		}
	}
	node.markUpdated()
	log.Printf(
		"updated cards for list %s (%s) on board %s (%s): %d new nodes, %d total cards\n",
		node.GetName(), node.GetTrelloID(),
		boardNode.GetName(), boardNode.GetTrelloID(),
		len(newNodes), len(boardNode.Cards),
	)

	return newNodes, nil, nil
}

func (node *FSList) LookupChild(name string) (FSNode, error) {
	node.Lock()
	defer node.Unlock()

	for _, card := range node.Cards {
		if card.GetName() == name {
			return card, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSList) ReadDir(dst []byte, offset int) int {
	node.Lock()
	defer node.Unlock()

	boardNode := node.BoardNode

	log.Printf(
		"read dir %s/%s (%s) id %d, offset %d\n",
		boardNode.GetName(),
		node.GetName(), node.GetTrelloID(), node.GetNodeID(), offset,
	)
	var size int
	for i := offset; i < len(node.Cards); i++ {
		card := node.Cards[i]
		tmp := fuseutil.WriteDirent(dst[size:], fuseutil.Dirent{
			Name:   card.GetName(),
			Inode:  card.GetNodeID(),
			Type:   fuseutil.DT_Directory,
			Offset: fuseops.DirOffset(i + 1),
		})
		if tmp == 0 {
			log.Printf(
				"read dir > no more space to write dirent for %s/%s (%s)\n",
				boardNode.GetName(),
				node.GetName(), node.GetTrelloID(),
			)
			break
		}
		log.Printf(
			"read dir %s/%s id %d: wrote direntry for %s (%s) id %d\n",
			boardNode.GetName(), node.GetName(), node.GetNodeID(),
			card.GetName(), card.GetTrelloID(), card.GetNodeID(),
		)
		size += tmp
	}
	return size
}
