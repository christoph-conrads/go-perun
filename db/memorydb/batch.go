// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package memorydb

import (
	"github.com/pkg/errors"
)

type Batch struct {
	db      *Database
	writes  map[string][]byte
	deletes map[string]struct{}
	bytes   uint
}

func (this *Batch) Put(key []byte, value []byte) error {
	skey := string(key)
	delete(this.deletes, skey)
	oldValue, exists := this.writes[skey]
	if exists {
		this.bytes -= uint(len(oldValue))
	}
	this.bytes += uint(len(value))
	this.writes[skey] = value
	return nil
}

func (this *Batch) Delete(key []byte) error {
	skey := string(key)
	oldValue, exists := this.writes[skey]
	if exists {
		this.bytes -= uint(len(oldValue))
		delete(this.writes, skey)
	}

	this.deletes[skey] = struct{}{}
	return nil
}

func (this *Batch) Len() uint {
	return uint(len(this.writes))
}

func (this *Batch) ValueSize() uint {
	return this.bytes
}

func (this *Batch) Write() error {
	for key := range this.writes {
		err := this.db.Put([]byte(key), this.writes[key])
		if err != nil {
			return errors.Wrap(err, "Failed to put entry.")
		}
	}

	for key := range this.deletes {
		err := this.db.Delete([]byte(key))
		if err != nil {
			return errors.Wrap(err, "Failed to delete entry.")
		}
	}
	return nil
}

func (this *Batch) Reset() {
	this.writes = nil
	this.deletes = nil
	this.bytes = 0
}