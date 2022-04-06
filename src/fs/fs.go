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

type FSNodeType int16

const (
	FSN_UNKNOWN FSNodeType = iota
	FSN_META
	FSN_CARD
	FSN_LIST
	FSN_BOARD
	FSN_WORKSPACE
	FSN_ROOT
)

type FSNode interface {
	Lock()
	Unlock()

	ShouldUpdate() bool
	Update() ([]FSNode, []FSNode, error) // (new, removed, error)
	GetName() string
	GetNodeType() FSNodeType
	GetTrelloID() string
	GetNodeID() fuseops.InodeID
	GetNodeAttrs() fuseops.InodeAttributes
	SetNodeID(fuseops.InodeID)

	LookupChild(string) (FSNode, error)

	ReadDir([]byte, int) int
}

type BaseFSNode struct {
	lock sync.Mutex

	name string

	uid uint32
	gid uint32

	NodeID    fuseops.InodeID
	NodeAttrs fuseops.InodeAttributes

	isDir    bool
	NodeType FSNodeType
	TrelloID string

	lastUpdate time.Time

	Ctx *trello.TrelloCtx
}

func (base *BaseFSNode) Lock() {
	base.lock.Lock()
}

func (base *BaseFSNode) Unlock() {
	base.lock.Unlock()
}

func (base *BaseFSNode) GetName() string {
	return base.name
}

func (base *BaseFSNode) GetNodeID() fuseops.InodeID {
	return base.NodeID
}

func (base *BaseFSNode) GetNodeAttrs() fuseops.InodeAttributes {
	return base.NodeAttrs
}

func (base *BaseFSNode) GetNodeType() FSNodeType {
	return base.NodeType
}

func (base *BaseFSNode) GetTrelloID() string {
	return base.TrelloID
}

func (base *BaseFSNode) SetNodeID(id fuseops.InodeID) {
	base.NodeID = id
}

func (base *BaseFSNode) getLastUpdated() time.Time {
	return base.lastUpdate
}

func (base *BaseFSNode) markUpdated() {
	base.lastUpdate = time.Now()
}

func (base *BaseFSNode) shouldUpdate(interval float64) bool {
	base.Lock()
	defer base.Unlock()
	delta := time.Since(base.lastUpdate)
	secs := delta.Seconds()
	return secs >= interval
}

type FSCard struct {
	BaseFSNode

	Card *trello.Card
}

func (node *FSCard) ShouldUpdate() bool {
	return node.shouldUpdate(30.0)
}

func (node *FSCard) Update() ([]FSNode, []FSNode, error) {
	return nil, nil, fuse.ENOENT
}

func (node *FSCard) LookupChild(name string) (FSNode, error) {
	return nil, fuse.ENOENT
}

func (node *FSCard) ReadDir(dst []byte, offset int) int {
	return 0
}

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
	return nil, nil, fuse.ENOENT
}

func (node *FSList) LookupChild(name string) (FSNode, error) {
	return nil, fuse.ENOENT
}

func (node *FSList) ReadDir(dst []byte, offset int) int {
	return 0
}

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
				NodeType: FSN_CARD,
				TrelloID: card.ID,
				Ctx:      node.Ctx,
			},
			Card: &card,
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
				NodeType: FSN_LIST,
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
	node.lock.Lock()
	defer node.lock.Unlock()

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
			NodeType: FSN_META,
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
			NodeType: FSN_META,
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
	node.lock.Lock()
	defer node.lock.Unlock()

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
	node.lock.Lock()
	defer node.lock.Unlock()

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
	node.lock.Lock()
	defer node.lock.Unlock()

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

		newAttrs := fuseops.InodeAttributes{
			Mode: 0700 | os.ModeDir,
			Uid:  node.uid,
			Gid:  node.gid,
		}
		newItem := &FSBoard{
			BaseFSNode: BaseFSNode{
				name:      board.Name,
				uid:       node.uid,
				gid:       node.gid,
				NodeAttrs: newAttrs,
				isDir:     true,
				NodeType:  FSN_BOARD,
				TrelloID:  board.ID,
				Ctx:       node.Ctx,
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
	node.lock.Lock()
	defer node.lock.Unlock()

	for _, board := range node.Boards {
		if board.name == name {
			return board, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *FSWorkspace) ReadDir(dst []byte, offset int) int {
	node.lock.Lock()
	defer node.lock.Unlock()

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

	node.lock.Lock()
	defer node.lock.Unlock()

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

		newAttrs := fuseops.InodeAttributes{
			Mode: 0700 | os.ModeDir,
			Uid:  node.uid,
			Gid:  node.gid,
		}
		newItem := &FSWorkspace{
			BaseFSNode: BaseFSNode{
				name:      ws.Name,
				uid:       node.uid,
				gid:       node.gid,
				NodeAttrs: newAttrs,
				isDir:     true,
				NodeType:  FSN_WORKSPACE,
				TrelloID:  ws.ID,
				Ctx:       node.Ctx,
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

	node.lock.Lock()
	defer node.lock.Unlock()

	for _, workspace := range node.workspaces {
		if workspace.GetName() == name {
			return workspace, nil
		}
	}
	return nil, fuse.ENOENT
}

func (node *TrelloTreeRoot) ReadDir(dst []byte, offset int) int {
	node.lock.Lock()
	defer node.lock.Unlock()

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
			NodeType:  FSN_ROOT,
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
