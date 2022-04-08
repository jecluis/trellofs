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
	"reflect"
)

type MetaEntry struct {
	Name     string
	Contents []byte
}

func getMeta(item interface{}) []MetaEntry {
	var entries []MetaEntry

	v := reflect.ValueOf(item)

	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		field := v.Type().Field(i)

		log.Printf(
			"meta > field %d, name: %s, type: %s\n",
			i, field.Name, field.Type.Kind(),
		)

		var contentStr string = ""
		fieldVal := v.Field(i).Interface()
		unknown := false
		switch field.Type.Name() {
		case "string":
			contentStr = fieldVal.(string)
			break
		case "bool":
			b := fieldVal.(bool)
			if b {
				contentStr = "true"
			} else {
				contentStr = "false"
			}
			break
		case "[]string":
			arr := fieldVal.([]string)
			for _, entry := range arr {
				contentStr += fmt.Sprintf("%s\n", entry)
			}
			break
		default:
			log.Printf(
				"meta > field %d, name: %s, type %s unknown\n",
				i, field.Name, field.Type.Kind(),
			)
			unknown = true
			break
		}

		if unknown {
			continue
		}

		entries = append(entries, MetaEntry{
			Name:     field.Name,
			Contents: []byte(contentStr),
		})
	}

	return entries
}
