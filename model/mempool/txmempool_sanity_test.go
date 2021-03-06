package mempool

import (
	"math"
	"testing"

	"github.com/copernet/copernicus/model/opcodes"
	"github.com/copernet/copernicus/model/outpoint"
	"github.com/copernet/copernicus/model/script"
	"github.com/copernet/copernicus/model/tx"
	"github.com/copernet/copernicus/model/txin"
	"github.com/copernet/copernicus/model/txout"
	"github.com/copernet/copernicus/util"
	"github.com/copernet/copernicus/util/amount"

	"github.com/copernet/copernicus/conf"
	"github.com/google/btree"
	"github.com/stretchr/testify/assert"
	"reflect"
)

func TestMempoolRemove(t *testing.T) {
	scriptSig := script.NewEmptyScript()
	scriptSig.PushOpCode(opcodes.OP_11)
	scriptPubkey := script.NewEmptyScript()
	scriptPubkey.PushOpCode(opcodes.OP_11)
	scriptPubkey.PushOpCode(opcodes.OP_EQUAL)

	txParent := tx.NewTx(0, 0)
	ti := txin.NewTxIn(
		outpoint.NewOutPoint(util.HashZero, 0),
		scriptSig,
		0,
	)
	txParent.AddTxIn(ti)
	for i := 0; i < 3; i++ {
		txParent.AddTxOut(txout.NewTxOut(
			amount.Amount(33000),
			scriptPubkey,
		))
	}

	txChild := make([]*tx.Tx, 3)
	for i := 0; i < 3; i++ {
		txChild[i] = tx.NewTx(0, 0)
		txChild[i].AddTxIn(
			txin.NewTxIn(
				outpoint.NewOutPoint(txParent.GetHash(), uint32(i)),
				scriptSig,
				script.SequenceFinal,
			))
		txChild[i].AddTxOut(
			txout.NewTxOut(
				amount.Amount(11000),
				scriptPubkey,
			),
		)
	}

	txGrandChild := make([]*tx.Tx, 3)
	for i := 0; i < 3; i++ {
		txGrandChild[i] = tx.NewTx(0, 0)
		txGrandChild[i].AddTxIn(
			txin.NewTxIn(
				outpoint.NewOutPoint(txChild[i].GetHash(), uint32(0)),
				scriptSig,
				script.SequenceFinal,
			))
		txGrandChild[i].AddTxOut(
			txout.NewTxOut(
				amount.Amount(11000),
				scriptPubkey,
			),
		)
	}

	mp := NewTxMempool()
	ps := mp.Size()
	tmpTxEntry := make(map[util.Hash]*TxEntry)
	if !reflect.DeepEqual(mp.GetAllTxEntry(), tmpTxEntry) {
		t.Errorf("expect zero value got %v", mp.GetAllTxEntry())
	}

	if !reflect.DeepEqual(mp.GetAllTxEntryWithoutLock(), tmpTxEntry) {
		t.Errorf("expect zero value got %v", mp.GetAllTxEntryWithoutLock())
	}

	assert.Equal(t, mp.GetPoolUsage(), int64(0))
	assert.Equal(t, mp.GetPoolAllTxSize(true), uint64(0))

	mp.RemoveTxRecursive(txParent, UNKNOWN)
	if mp.Size() != ps {
		t.Errorf("expect 0 got %d", mp.Size())
	}

	noLimit := uint64(math.MaxUint64)
	testEntryHelp := NewTestMemPoolEntry()

	// Just the parent
	ancestors, _ := mp.CalculateMemPoolAncestors(txParent, noLimit, noLimit, noLimit, noLimit, true)
	entryParent := testEntryHelp.FromTxToEntry(txParent)
	mp.AddTx(entryParent, ancestors)
	ps = mp.Size()
	mp.RemoveTxRecursive(txParent, UNKNOWN)
	if mp.Size() != ps-1 {
		t.Errorf("expect %d got %d", ps-1, mp.Size())
	}

	// Parent, children, grandchildren
	mp.AddTx(entryParent, ancestors)
	for i := 0; i < 3; i++ {
		ancestors, _ := mp.CalculateMemPoolAncestors(txChild[i], noLimit, noLimit, noLimit, noLimit, true)
		entry := testEntryHelp.FromTxToEntry(txChild[i])
		mp.AddTx(entry, ancestors)

		ancestors, _ = mp.CalculateMemPoolAncestors(txGrandChild[i], noLimit, noLimit, noLimit, noLimit, true)
		entry = testEntryHelp.FromTxToEntry(txGrandChild[i])
		mp.AddTx(entry, ancestors)
	}
	ps = mp.Size()

	txentry := mp.GetAllTxEntry()
	noLockTxEntry := mp.GetAllTxEntryWithoutLock()
	if len(txentry) != 7 || len(noLockTxEntry) != 7 {
		t.Errorf("tx entry map expected 7 got %d", len(txentry))
	}

	mp.RemoveTxRecursive(txChild[0], UNKNOWN)
	if mp.Size() != ps-2 {
		t.Errorf("expect %d got %d", ps-2, mp.Size())
	}

	// ... make sure grandchild and child are gone:
	ps = mp.Size()
	mp.RemoveTxRecursive(txGrandChild[0], UNKNOWN)
	if mp.Size() != ps {
		t.Errorf("expect %d got %d", ps, mp.Size())
	}

	ps = mp.Size()
	mp.RemoveTxRecursive(txChild[0], UNKNOWN)
	if mp.Size() != ps {
		t.Errorf("expect %d got %d", ps, mp.Size())
	}

	// Remove parent, all children/grandchildren should go:
	mp.RemoveTxRecursive(txParent, UNKNOWN)
	if mp.Size() != 0 {
		t.Errorf("expect %d got %d", 0, mp.Size())
	}

	// Add children and grandchildren, but NOT the parent (simulate the parent
	// being in a block)
	for i := 0; i < 3; i++ {
		ancestors, _ := mp.CalculateMemPoolAncestors(txChild[i], noLimit, noLimit, noLimit, noLimit, true)
		entry := testEntryHelp.FromTxToEntry(txChild[i])
		mp.AddTx(entry, ancestors)

		ancestors, _ = mp.CalculateMemPoolAncestors(txGrandChild[i], noLimit, noLimit, noLimit, noLimit, true)
		entry = testEntryHelp.FromTxToEntry(txGrandChild[i])
		mp.AddTx(entry, ancestors)
	}

	// Now remove the parent, as might happen if a block-re-org occurs but the
	// parent cannot be put into the mempool (maybe because it is non-standard):
	ps = mp.Size()
	mp.RemoveTxRecursive(txParent, UNKNOWN)
	if mp.Size() != ps-6 {
		t.Errorf("expect %d got %d", ps-6, mp.Size())
	}
}

func TestMempoolOrphan(t *testing.T) {
	scriptSig := script.NewEmptyScript()
	scriptSig.PushOpCode(opcodes.OP_11)
	scriptPubkey := script.NewEmptyScript()
	scriptPubkey.PushOpCode(opcodes.OP_11)
	scriptPubkey.PushOpCode(opcodes.OP_EQUAL)

	txParent := tx.NewTx(0, 0)
	ti := txin.NewTxIn(
		outpoint.NewOutPoint(util.HashZero, 0),
		scriptSig,
		0,
	)
	txParent.AddTxIn(ti)
	for i := 0; i < 3; i++ {
		txParent.AddTxOut(txout.NewTxOut(
			amount.Amount(33000),
			scriptPubkey,
		))
	}

	mp := NewTxMempool()
	hash := txParent.GetHash()

	for _, ok := range []bool{true, false} {
		mp.AddOrphanTx(txParent, 0x01)
		mp.EraseOrphanTx(hash, ok)
	}

	tmpOrphanTx := make(map[util.Hash]OrphanTx)
	assert.Equal(t, mp.OrphanTransactions, tmpOrphanTx)

	mp.AddOrphanTx(txParent, 0x01)
	numEvicted := mp.RemoveOrphansByTag(0x01)
	assert.Equal(t, numEvicted, 1)

}

func TestMempoolAncestorIndexing(t *testing.T) {
	scriptSig := script.NewEmptyScript()
	scriptSig.PushOpCode(opcodes.OP_11)
	scriptPubkey := script.NewEmptyScript()
	scriptPubkey.PushOpCode(opcodes.OP_11)
	scriptPubkey.PushOpCode(opcodes.OP_EQUAL)

	noLimit := uint64(math.MaxUint64)
	testEntryHelp := NewTestMemPoolEntry()
	mp := NewTxMempool()

	/* 3rd highest fee */
	tx1 := tx.NewTx(0, 0)
	tx1.AddTxOut(txout.NewTxOut(
		amount.Amount(10*util.COIN),
		scriptPubkey,
	))
	ancestors, _ := mp.CalculateMemPoolAncestors(tx1, noLimit, noLimit, noLimit, noLimit, true)
	entry1 := testEntryHelp.SetFee(10000).FromTxToEntry(tx1)
	mp.AddTx(entry1, ancestors)

	/* highest fee */
	tx2 := tx.NewTx(0, 0)
	tx2.AddTxOut(txout.NewTxOut(
		amount.Amount(2*util.COIN),
		scriptPubkey,
	))
	tx2Size := tx2.EncodeSize()
	ancestors, _ = mp.CalculateMemPoolAncestors(tx2, noLimit, noLimit, noLimit, noLimit, true)
	entry2 := testEntryHelp.SetFee(20000).FromTxToEntry(tx2)
	mp.AddTx(entry2, ancestors)

	/* lowest fee */
	tx3 := tx.NewTx(0, 0)
	tx3.AddTxOut(txout.NewTxOut(
		amount.Amount(5*util.COIN),
		scriptPubkey,
	))
	ancestors, _ = mp.CalculateMemPoolAncestors(tx3, noLimit, noLimit, noLimit, noLimit, true)
	entry3 := testEntryHelp.SetFee(0).FromTxToEntry(tx3)
	mp.AddTx(entry3, ancestors)

	/*  2nd highest fee */
	tx4 := tx.NewTx(0, 0)
	tx4.AddTxOut(txout.NewTxOut(
		amount.Amount(7*util.COIN),
		scriptPubkey,
	))
	ancestors, _ = mp.CalculateMemPoolAncestors(tx4, noLimit, noLimit, noLimit, noLimit, true)
	entry4 := testEntryHelp.SetFee(15000).FromTxToEntry(tx4)
	mp.AddTx(entry4, ancestors)

	/* equal fee rate to tx1, but newer */
	tx5 := tx.NewTx(0, 0)
	tx5.AddTxOut(txout.NewTxOut(
		amount.Amount(11*util.COIN),
		scriptPubkey,
	))
	ancestors, _ = mp.CalculateMemPoolAncestors(tx5, noLimit, noLimit, noLimit, noLimit, true)
	entry5 := testEntryHelp.SetFee(10000).FromTxToEntry(tx5)
	mp.AddTx(entry5, ancestors)

	assert.Equal(t, mp.Size(), 5, "mempool size should equal 5")

	sortedOrder := make([]util.Hash, 6)
	sortedOrder[0] = tx2.GetHash() //20000
	sortedOrder[1] = tx4.GetHash() //15000

	tx1hash := tx1.GetHash()
	tx5hash := tx5.GetHash()

	if tx1hash.Cmp(&tx5hash) < 0 {
		sortedOrder[2] = tx1.GetHash()
		sortedOrder[3] = tx5.GetHash()
	} else {
		sortedOrder[2] = tx5.GetHash()
		sortedOrder[3] = tx1.GetHash()
	}

	sortedOrder[4] = tx3.GetHash() //0

	index := 0
	mp.txByAncestorFeeRateSort.Ascend(func(i btree.Item) bool {
		entry := i.(*EntryAncestorFeeRateSort)
		if entry.Tx.GetHash() != sortedOrder[index] {
			t.Errorf("the sort by fee is error, index : %d, expect hash : %s, actual hash is : %v\n",
				index, sortedOrder[index], entry.Tx.GetHash())
			return true
		}
		index++
		return true
	})

	/* low fee parent with high fee child */
	/* tx6 (0) -> tx7 (high) */
	tx6 := tx.NewTx(0, 0)
	tx6.AddTxOut(txout.NewTxOut(
		amount.Amount(20*util.COIN),
		scriptPubkey,
	))
	tx6Size := tx6.EncodeSize()
	ancestors, _ = mp.CalculateMemPoolAncestors(tx6, noLimit, noLimit, noLimit, noLimit, true)
	entry6 := testEntryHelp.SetFee(0).FromTxToEntry(tx6)
	mp.AddTx(entry6, ancestors)

	tx3hash := tx3.GetHash()
	tx6hash := tx6.GetHash()

	if tx3hash.Cmp(&tx6hash) < 0 {
		sortedOrder[4] = tx3hash
		sortedOrder[5] = tx6hash
	} else {
		sortedOrder[4] = tx6hash
		sortedOrder[5] = tx3hash
	}

	index = 0
	mp.txByAncestorFeeRateSort.Ascend(func(i btree.Item) bool {
		entry := i.(*EntryAncestorFeeRateSort)
		if entry.Tx.GetHash() != sortedOrder[index] {
			t.Errorf("the sort by fee is error, index : %d, expect hash : %s, actual hash is : %v\n",
				index, sortedOrder[index], entry.Tx.GetHash())
			return true
		}
		index++
		return true
	})

	tx7 := tx.NewTx(0, 0)
	tx7.AddTxIn(txin.NewTxIn(
		outpoint.NewOutPoint(tx6hash, 0),
		scriptSig,
		0,
	))
	tx7.AddTxOut(txout.NewTxOut(
		amount.Amount(10*util.COIN),
		scriptPubkey,
	))
	tx7Size := tx7.EncodeSize()
	ancestors, _ = mp.CalculateMemPoolAncestors(tx7, noLimit, noLimit, noLimit, noLimit, true)
	/* set the fee to just below tx2's feerate when including ancestor */
	fee := 20000/tx2Size*(tx7Size+tx6Size) - 1
	entry7 := testEntryHelp.SetFee(amount.Amount(fee)).FromTxToEntry(tx7)
	mp.AddTx(entry7, ancestors)
	assert.Equal(t, mp.Size(), 7, "mempool size should equal 7")
	tmpOrder := make([]util.Hash, 7)
	tmpOrder[0] = sortedOrder[0]
	tmpOrder[1] = tx7.GetHash()
	copy(tmpOrder[2:], sortedOrder[1:])
	sortedOrder = tmpOrder

	index = 0
	mp.txByAncestorFeeRateSort.Ascend(func(i btree.Item) bool {
		entry := i.(*EntryAncestorFeeRateSort)
		if entry.Tx.GetHash() != sortedOrder[index] {
			t.Errorf("the sort by fee is error, index : %d, expect hash : %s, actual hash is : %v\n",
				index, sortedOrder[index], entry.Tx.GetHash())
			return false
		}
		index++
		return true
	})
	/* after tx6 is mined, tx7 should move up in the sort */
	vtx := []*tx.Tx{tx6}
	mp.RemoveTxSelf(vtx)
	sortedOrder = append(sortedOrder[:1], sortedOrder[2:]...)

	if tx3hash.Cmp(&tx6hash) < 0 {
		sortedOrder = sortedOrder[:len(sortedOrder)-1]
	} else {
		sortedOrder = append(sortedOrder[:4], sortedOrder[5:]...)
	}

	sortedOrder = append([]util.Hash{tx7.GetHash()}, sortedOrder...)

	index = 0
	mp.txByAncestorFeeRateSort.Ascend(func(i btree.Item) bool {
		entry := i.(*EntryAncestorFeeRateSort)
		h := entry.Tx.GetHash()
		if entry.Tx.GetHash() != sortedOrder[index] {
			t.Errorf("the sort by ancestor fee is error, index : %d, expect hash : %s, actual hash is : %v\n",
				index, sortedOrder[index], &h)
			return false
		}
		index++
		return true
	})
}

func TestTxMempool_GetMinFee(t *testing.T) {
	mp := NewTxMempool()
	mp.rollingMinimumFeeRate = 10
	mp.blockSinceLastRollingFeeBump = false
	res := mp.GetMinFee(1000)
	assert.Equal(t, res, *util.NewFeeRate(mp.rollingMinimumFeeRate))

	mp.blockSinceLastRollingFeeBump = true
	mp.lastRollingFeeUpdate = 1540260957
	res = mp.GetMinFee(1000)
	assert.Equal(t, res, *util.NewFeeRate(1))

	mp.usageSize = 1000
	mp.rollingMinimumFeeRate = 10
	mp.lastRollingFeeUpdate = 10
	res = mp.GetMinFee(1000)
	assert.Equal(t, res, *util.NewFeeRate(1))

	mp1 := NewTxMempool()
	mp1.rollingMinimumFeeRate = 100
	mp1.blockSinceLastRollingFeeBump = true
	mp1.usageSize = 1000
	mp1.incrementalRelayFee = *util.NewFeeRate(1000)
	res1 := mp1.GetMinFee(1000)
	assert.Equal(t, res1, *util.NewFeeRate(0))

	mp2 := NewTxMempool()
	conf.Cfg = conf.InitConfig(nil)
	tmpFeeRate := mp2.GetMinFee(conf.Cfg.Mempool.MaxPoolSize)
	feeRate := mp2.GetMinFeeRate()
	assert.Equal(t, tmpFeeRate, feeRate)
}
