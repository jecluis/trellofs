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

type Board struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Desc     string `json:"desc"`
	DescData string `json:"descData"`
	Closed   bool   `json:"closed"`
}

type List struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Closed bool   `json:"closed"`
	Board  *Board
}

func (board *Board) GetCards(ctx *TrelloCtx) ([]Card, error) {

	endpoint := MakeEndpoint(
		fmt.Sprintf("/boards/%s/cards", board.ID), nil,
	)
	cardsRaw, err := ctx.ApiGet(endpoint)
	if err != nil {
		log.Printf(
			"error obtaining cards for board: %s (%s)",
			board.Name,
			board.ID,
		)
		return nil, err
	}
	var cards []Card
	json.Unmarshal(cardsRaw, &cards)
	for _, c := range cards {
		c.Board = board
	}

	log.Println(string(cardsRaw))
	return cards, nil
}

func (board *Board) GetLists(
	client *TrelloCtx,
) ([]List, error) {

	endpoint := MakeEndpoint(
		fmt.Sprintf("/boards/%s/lists", board.ID),
		nil,
	)
	listsRaw, err := client.ApiGet(endpoint)
	if err != nil {
		log.Printf("error obtaining orgs: %s\n", err)
		return nil, err
	}

	var lists []List
	json.Unmarshal(listsRaw, &lists)
	for _, l := range lists {
		l.Board = board
	}
	return lists, nil
}

func (list *List) GetCards(
	client *TrelloCtx,
) ([]Card, error) {

	endpoint := MakeEndpoint(
		fmt.Sprintf("/lists/%s/cards", list.ID),
		nil,
	)
	cardsRaw, err := client.ApiGet(endpoint)
	if err != nil {
		log.Printf(
			"error obtaining cards for list %s (%s)",
			list.Name, list.ID,
		)
		return nil, err
	}

	var cards []Card
	json.Unmarshal(cardsRaw, &cards)
	for _, c := range cards {
		c.Board = list.Board
	}
	return cards, nil
}
