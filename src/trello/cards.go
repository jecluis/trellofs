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

type CardLabel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Card struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`

	ListID    string   `json:"idList"`
	BoardID   string   `json:"idBoard"`
	MemberIDs []string `json:"idMembers"`

	Labels      []CardLabel `json:"labels"`
	Due         string      `json:"due"`
	DueComplete bool        `json:"dueComplete"`
	LastActive  string      `json:"dateLastActivity"`

	Board *Board
}
