package global

import (
	"sync"

	"github.com/copernet/copernicus/model/block"
	"github.com/copernet/copernicus/model/blockindex"
	"github.com/copernet/copernicus/util"
)

const (
	// MaxBlockFileSize is the maximum size of a blk?????.dat file (since 0.8) */
	MaxBlockFileSize = uint32(0x8000000)
	// BlockFileChunkSize is the pre-allocation chunk size for blk?????.dat files (since 0.8)
	BlockFileChunkSize = 0x1000000
	// UndoFileChunkSize is the pre-allocation chunk size for rev?????.dat files (since 0.8) */
	UndoFileChunkSize     = 0x100000
	DefaultMaxMemPoolSize = 300
)

var (
	CsMain          = new(sync.RWMutex)
	CsLastBlockFile = new(sync.RWMutex)
	persistGlobal   *PersistGlobal
)

type (
	MapBlocksUnlinked map[*blockindex.BlockIndex][]*blockindex.BlockIndex
)

type PersistGlobal struct {
	GlobalBlockFileInfo                                  []*block.BlockFileInfo
	GlobalLastBlockFile                                  int32 //last block file no.
	GlobalLastWrite, GlobalLastFlush, GlobalLastSetChain int   // last update time
	DefaultMaxMemPoolSize                                uint
	GlobalDirtyFileInfo                                  map[int32]bool // temp for update file info
	GlobalDirtyBlockIndex                                map[util.Hash]*blockindex.BlockIndex
	GlobalTimeReadFromDisk                               int64
	GlobalTimeConnectTotal                               int64
	GlobalTimeChainState                                 int64
	GlobalTimeFlush                                      int64
	GlobalTimeCheck                                      int64
	GlobalTimeForks                                      int64
	GlobalTimePostConnect                                int64
	GlobalTimeTotal                                      int64
	GlobalBlockSequenceID                                int32
	GlobalMapBlocksUnlinked                              MapBlocksUnlinked
}

type PruneState struct {
	PruneMode       bool
	HavePruned      bool
	PruneTarget     uint64
	CheckForPruning bool
	Reindex         bool
}

func (pg *PersistGlobal) AddDirtyBlockIndex(pindex *blockindex.BlockIndex) {
	pg.GlobalDirtyBlockIndex[*pindex.GetBlockHash()] = pindex
}

func (pg *PersistGlobal) AddBlockSequenceID() {
	pg.GlobalBlockSequenceID++
}

func InitPersistGlobal() *PersistGlobal {
	cg := new(PersistGlobal)
	cg.GlobalBlockFileInfo = make([]*block.BlockFileInfo, 0, 1000)
	cg.GlobalDirtyFileInfo = make(map[int32]bool)
	cg.GlobalDirtyBlockIndex = make(map[util.Hash]*blockindex.BlockIndex)
	cg.GlobalMapBlocksUnlinked = make(MapBlocksUnlinked)
	return cg
}

func InitPruneState() *PruneState {
	ps := &PruneState{
		PruneMode:       false,
		HavePruned:      false,
		CheckForPruning: false,
		Reindex:         false,
		PruneTarget:     0,
	}
	return ps
}

func GetInstance() *PersistGlobal {
	if persistGlobal == nil {
		persistGlobal = InitPersistGlobal()
	}
	return persistGlobal
}
