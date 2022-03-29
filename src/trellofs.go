/*
 * trellofs - A Trello POSIX filesystem
 * Copyright (C) 2022  Joao Eduardo Luis <joao@wipwd.dev>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 */
package main

import (
	"context"
	"flag"
	"log"
	"os/user"
	"strconv"
	"trellofs/config"
	"trellofs/fs"
	"trellofs/trello"

	"github.com/jacobsa/fuse"
)

var fMountPoint = flag.String("mount", "", "Path to Mount point.")
var fConfigFile = flag.String("config", "", "Path to config file.")

func main() {

	flag.Parse()

	if *fMountPoint == "" {
		log.Fatalf("Must provide mount point via '--mount'")
	} else if *fConfigFile == "" {
		log.Fatalf("Must provide config file via '--config'")
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}

	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		panic(err)
	}
	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		panic(err)
	}

	config, err := config.ReadConfig(*fConfigFile)
	if err != nil {
		panic(err)
	}

	trelloCtx := trello.Trello(config.ID, config.Key, config.Token)
	trelloFS, err := fs.NewTrelloFS(uint32(uid), uint32(gid), trelloCtx)
	if err != nil {
		panic(err)
	}

	cfg := &fuse.MountConfig{
		DisableWritebackCaching: true,
		ReadOnly:                true, // eventually make read/write
	}

	mfs, err := fuse.Mount(*fMountPoint, trelloFS, cfg)
	if err != nil {
		log.Fatalf("error mounting %s: %v", *fMountPoint, err)
	}

	if err = mfs.Join(context.Background()); err != nil {
		log.Fatalf("error waiting for filesystem: %v", err)
	}
}
