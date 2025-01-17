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

package stream

import (
	"io"

	"go.uber.org/multierr"

	"github.com/apache/skywalking-banyandb/api/common"
	databasev1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/database/v1"
	streamv1 "github.com/apache/skywalking-banyandb/api/proto/banyandb/stream/v1"
	"github.com/apache/skywalking-banyandb/banyand/tsdb"
	"github.com/apache/skywalking-banyandb/pkg/index"
	"github.com/apache/skywalking-banyandb/pkg/partition"
)

type callbackFn func()
type indexMessage struct {
	localWriter tsdb.Writer
	blockCloser io.Closer
	value       *streamv1.ElementValue
	cb          callbackFn
}

func (s *stream) bootIndexGenerator() {
	go func() {
		for {
			m, more := <-s.indexCh
			if !more {
				return
			}
			var err error
			for _, ruleIndex := range s.indexRuleIndex {
				rule := ruleIndex.rule
				switch rule.GetLocation() {
				case databasev1.IndexRule_LOCATION_SERIES:
					err = multierr.Append(err, writeLocalIndex(m.localWriter, ruleIndex, m.value))
				case databasev1.IndexRule_LOCATION_GLOBAL:
					err = multierr.Append(err, s.writeGlobalIndex(ruleIndex, m.localWriter.ItemID(), m.value))
				}
			}
			err = multierr.Append(err, m.blockCloser.Close())
			if err != nil {
				s.l.Error().Err(err).Msg("encounter some errors when generating indices")
			}
			if m.cb != nil {
				m.cb()
			}
		}
	}()
}

//TODO: should listen to pipeline in a distributed cluster
func (s *stream) writeGlobalIndex(ruleIndex indexRule, ref tsdb.GlobalItemID, value *streamv1.ElementValue) error {
	val, _, err := getIndexValue(ruleIndex, value)
	if err != nil {
		return err
	}
	indexShardID, err := partition.ShardID(val, s.schema.ShardNum)
	if err != nil {
		return err
	}
	shard, err := s.db.Shard(common.ShardID(indexShardID))
	if err != nil {
		return err
	}
	builder := shard.Index().WriterBuilder()
	indexWriter, err := builder.
		GlobalItemID(ref).
		Time(value.GetTimestamp().AsTime()).
		Build()
	if err != nil {
		return err
	}
	rule := ruleIndex.rule
	switch rule.GetType() {
	case databasev1.IndexRule_TYPE_INVERTED:
		return indexWriter.WriteInvertedIndex(index.Field{
			Key: index.FieldKey{
				IndexRuleID: rule.GetMetadata().GetId(),
			},
			Term: val,
		})
	case databasev1.IndexRule_TYPE_TREE:
		return indexWriter.WriteLSMIndex(index.Field{
			Key: index.FieldKey{
				IndexRuleID: rule.GetMetadata().GetId(),
			},
			Term: val,
		})
	}
	return err
}

func writeLocalIndex(writer tsdb.Writer, ruleIndex indexRule, value *streamv1.ElementValue) (err error) {
	val, _, err := getIndexValue(ruleIndex, value)
	if err != nil {
		return err
	}
	rule := ruleIndex.rule
	switch rule.GetType() {
	case databasev1.IndexRule_TYPE_INVERTED:
		return writer.WriteInvertedIndex(index.Field{
			Key: index.FieldKey{
				IndexRuleID: rule.GetMetadata().GetId(),
			},
			Term: val,
		})
	case databasev1.IndexRule_TYPE_TREE:
		return writer.WriteLSMIndex(index.Field{
			Key: index.FieldKey{
				IndexRuleID: rule.GetMetadata().GetId(),
			},
			Term: val,
		})
	}
	return err
}
