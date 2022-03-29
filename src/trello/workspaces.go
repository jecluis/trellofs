/*
 * trellofs - A Trello POSIX filesystem
 * Copyright (C) 2022  Joao Eduardo Luis <joao@wipwd.dev>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 */

package trello

import (
	"encoding/json"
	"fmt"
	"log"
)

type Workspace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Desc        string `json:"desc"`
}

func GetWorkspaces(ctx *TrelloCtx) ([]Workspace, error) {

	orgsEndpoint := MakeEndpoint(
		fmt.Sprintf("/members/%s/organizations", ctx.ID),
		[]string{"id", "name", "displayName"},
	)
	orgsRaw, err := ctx.ApiGet(orgsEndpoint)
	if err != nil {
		log.Printf("error obtaining orgs: %s\n", err)
		return nil, err
	}

	fmt.Println(string(orgsRaw))

	var orgs []Workspace
	json.Unmarshal(orgsRaw, &orgs)
	return orgs, nil
}

func (workspace *Workspace) GetBoards(
	ctx *TrelloCtx,
) ([]Board, error) {

	boardsEndpoint := MakeEndpoint(
		fmt.Sprintf("/organizations/%s/boards", workspace.ID),
		[]string{"id", "name", "desc", "descData", "closed"},
	)
	boardsRaw, err := ctx.ApiGet(boardsEndpoint)
	if err != nil {
		log.Printf("error obtaining orgs: %s\n", err)
		return nil, err
	}

	// fmt.Println(string(boardsRaw))

	var boards []Board
	json.Unmarshal(boardsRaw, &boards)
	return boards, nil
}
