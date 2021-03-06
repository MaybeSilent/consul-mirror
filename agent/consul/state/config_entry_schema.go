package state

import (
	"github.com/hashicorp/go-memdb"
)

const (
	tableConfigEntries = "config-entries"

	indexLink              = "link"
	indexIntentionLegacyID = "intention-legacy-id"
	indexSource            = "intention-source"
)

// configTableSchema returns a new table schema used to store global
// config entries.
func configTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: tableConfigEntries,
		Indexes: map[string]*memdb.IndexSchema{
			indexID: {
				Name:         indexID,
				AllowMissing: false,
				Unique:       true,
				Indexer: indexerSingleWithPrefix{
					readIndex:   readIndex(indexFromConfigEntryKindName),
					writeIndex:  writeIndex(indexFromConfigEntry),
					prefixIndex: prefixIndex(indexFromConfigEntryKindName),
				},
			},
			indexKind: {
				Name:         indexKind,
				AllowMissing: false,
				Unique:       false,
				Indexer: indexerSingle{
					readIndex:  readIndex(indexFromConfigEntryKindQuery),
					writeIndex: writeIndex(indexKindFromConfigEntry),
				},
			},
			indexLink: {
				Name:         indexLink,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ConfigEntryLinkIndex{},
			},
			indexIntentionLegacyID: {
				Name:         indexIntentionLegacyID,
				AllowMissing: true,
				Unique:       true,
				Indexer:      &ServiceIntentionLegacyIDIndex{},
			},
			indexSource: {
				Name:         indexSource,
				AllowMissing: true,
				Unique:       false,
				Indexer:      &ServiceIntentionSourceIndex{},
			},
		},
	}
}
