// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package inverted

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/apache/skywalking-banyandb/api/common"
	"github.com/apache/skywalking-banyandb/pkg/convert"
	"github.com/apache/skywalking-banyandb/pkg/index"
)

var ErrFieldAbsent = errors.New("field doesn't exist")

type fieldHashID uint64

type fieldMap struct {
	repo  map[fieldHashID]*fieldValue
	mutex sync.RWMutex
}

func newFieldMap(initialSize int) *fieldMap {
	return &fieldMap{
		repo: make(map[fieldHashID]*fieldValue, initialSize),
	}
}

func (fm *fieldMap) createKey(key []byte) *fieldValue {
	result := &fieldValue{
		key:   key,
		value: newPostingMap(),
	}
	fm.repo[fieldHashID(convert.Hash(key))] = result
	return result
}

func (fm *fieldMap) get(key []byte) (*fieldValue, bool) {
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.getWithoutLock(key)
}

func (fm *fieldMap) getWithoutLock(key []byte) (*fieldValue, bool) {
	v, ok := fm.repo[fieldHashID(convert.Hash(key))]
	return v, ok
}

func (fm *fieldMap) put(fv index.Field, id common.ItemID) error {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()
	pm, ok := fm.getWithoutLock(fv.Key)
	if !ok {
		pm = fm.createKey(fv.Key)
	}
	return pm.value.put(fv.Term, id)
}

type fieldValue struct {
	key   []byte
	value *postingMap
}