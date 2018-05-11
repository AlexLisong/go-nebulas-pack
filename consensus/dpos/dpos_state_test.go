// Copyright (C) 2017 go-nebulas authors
//
// This file is part of the go-nebulas library.
//
// the go-nebulas library is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// the go-nebulas library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with the go-nebulas library.  If not, see <http://www.gnu.org/licenses/>.
//

package dpos

import (
	"testing"

	"github.com/alexlisong/go-nebulas/consensus/pb"

	"github.com/alexlisong/go-nebulas/core"
	"github.com/alexlisong/go-nebulas/crypto/keystore"

	"github.com/stretchr/testify/assert"

	"github.com/alexlisong/go-nebulas/common/trie"
	"github.com/alexlisong/go-nebulas/storage"
	"github.com/alexlisong/go-nebulas/util/byteutils"
)

func checkDynasty(t *testing.T, consensus core.Consensus, consensusRoot *consensuspb.ConsensusRoot, storage storage.Storage) {
	consensusState, err := consensus.NewState(consensusRoot, storage, false)
	assert.Nil(t, err)
	dynasty, err := consensusState.Dynasty()
	assert.Nil(t, err)
	for i := 0; i < DynastySize-1; i++ {
		a, _ := core.AddressParseFromBytes(dynasty[i])
		assert.Equal(t, a.String(), DefaultOpenDynasty[i])
	}
}

func TestBlock_NextDynastyContext(t *testing.T) {
	neb := mockNeb(t)
	block := neb.chain.GenesisBlock()

	context, err := block.WorldState().NextConsensusState(BlockIntervalInMs / SecondInMs)
	assert.Nil(t, err)
	miners, _ := block.WorldState().Dynasty()
	assert.Equal(t, context.Proposer(), miners[1])
	// check dynasty
	consensusRoot := context.RootHash()
	assert.Nil(t, err)
	checkDynasty(t, neb.consensus, consensusRoot, neb.Storage())

	context, err = block.WorldState().NextConsensusState((BlockIntervalInMs + DynastyIntervalInMs) / SecondInMs)
	assert.Nil(t, err)
	miners, _ = block.WorldState().Dynasty()
	assert.Equal(t, context.Proposer(), miners[1])
	// check dynasty
	consensusRoot = context.RootHash()
	assert.Nil(t, err)
	checkDynasty(t, neb.consensus, consensusRoot, neb.Storage())

	context, err = block.WorldState().NextConsensusState(DynastyIntervalInMs / SecondInMs / 2)
	assert.Nil(t, err)
	miners, _ = block.WorldState().Dynasty()
	assert.Equal(t, context.Proposer(), miners[int(DynastyIntervalInMs/2/BlockIntervalInMs)%DynastySize])
	// check dynasty
	consensusRoot = context.RootHash()
	assert.Nil(t, err)
	checkDynasty(t, neb.consensus, consensusRoot, neb.Storage())

	context, err = block.WorldState().NextConsensusState((DynastyIntervalInMs*2 + DynastyIntervalInMs/3) / SecondInMs)
	assert.Nil(t, err)
	miners, _ = block.WorldState().Dynasty()
	index := int((DynastyIntervalInMs*2+DynastyIntervalInMs/3)%DynastyIntervalInMs) / int(BlockIntervalInMs) % DynastySize
	assert.Equal(t, context.Proposer(), miners[index])
	// check dynasty
	consensusRoot = context.RootHash()
	assert.Nil(t, err)
	checkDynasty(t, neb.consensus, consensusRoot, neb.Storage())

	// new block
	coinbase, err := core.AddressParseFromBytes(miners[4])
	assert.Nil(t, err)
	assert.Nil(t, neb.am.Unlock(coinbase, []byte("passphrase"), keystore.DefaultUnlockDuration))

	newBlock, _ := core.NewBlock(neb.chain.ChainID(), coinbase, neb.chain.TailBlock())
	newBlock.SetTimestamp((DynastyIntervalInMs*2 + DynastyIntervalInMs/3) / SecondInMs)
	newBlock.WorldState().SetConsensusState(context)
	assert.Equal(t, newBlock.Seal(), nil)
	assert.Nil(t, neb.am.SignBlock(coinbase, newBlock))
	newBlock, _ = mockBlockFromNetwork(newBlock)
	newBlock.LinkParentBlock(neb.chain, neb.chain.TailBlock())
	assert.Nil(t, newBlock.VerifyExecution()) //neb.chain.TailBlock(), neb.chain.ConsensusHandler()
}

func TestTraverseDynasty(t *testing.T) {
	stor, err := storage.NewMemoryStorage()
	assert.Nil(t, err)
	dynasty, err := trie.NewTrie(nil, stor, false)
	assert.Nil(t, err)
	members, err := TraverseDynasty(dynasty)
	assert.Nil(t, err)
	assert.Equal(t, members, []byteutils.Hash{})
}

func TestInitialDynastyNotEnough(t *testing.T) {
	neb := mockNeb(t)
	neb.genesis.Consensus.Dpos.Dynasty = []string{}
	chain, err := core.NewBlockChain(neb)
	assert.Nil(t, err)
	assert.Equal(t, chain.Setup(neb), core.ErrGenesisNotEqualDynastyLenInDB)
	neb.storage, _ = storage.NewMemoryStorage()
	chain, err = core.NewBlockChain(neb)
	assert.Nil(t, err)
	assert.Equal(t, chain.Setup(neb), ErrInitialDynastyNotEnough)
}

func TestNewGenesisBlock(t *testing.T) {
	conf := MockGenesisConf()
	chain := mockNeb(t).chain
	dumpConf, err := core.DumpGenesis(chain)
	assert.Nil(t, err)
	assert.Equal(t, dumpConf.Meta.ChainId, conf.Meta.ChainId)
	assert.Equal(t, dumpConf.Consensus.Dpos.Dynasty, conf.Consensus.Dpos.Dynasty)
	assert.Equal(t, dumpConf.TokenDistribution, conf.TokenDistribution)
}

func TestCheckGenesisAndDBConsense(t *testing.T) {
	conf := MockGenesisConf()
	chain := mockNeb(t).chain

	genesisDB, err := core.DumpGenesis(chain)
	assert.Nil(t, err)
	err = core.CheckGenesisConfByDB(genesisDB, conf)
	assert.Nil(t, err)

	conf4 := MockGenesisConf()
	conf4.TokenDistribution[0].Value = "1001"
	err = core.CheckGenesisConfByDB(genesisDB, conf4)
	assert.NotNil(t, err)
	assert.Equal(t, err, core.ErrGenesisNotEqualTokenInDB)

	conf1 := MockGenesisConf()
	conf1.Consensus.Dpos.Dynasty = nil
	// fmt.Printf("conf1:%v\n", conf1)
	err = core.CheckGenesisConfByDB(genesisDB, conf1)
	assert.NotNil(t, err)
	assert.Equal(t, err, core.ErrGenesisNotEqualDynastyLenInDB)

	conf2 := MockGenesisConf()
	conf2.Consensus.Dpos.Dynasty[0] = "12b"
	err = core.CheckGenesisConfByDB(genesisDB, conf2)
	assert.NotNil(t, err)
	assert.Equal(t, err, core.ErrGenesisNotEqualDynastyInDB)

	conf3 := MockGenesisConf()
	conf3.TokenDistribution = nil
	err = core.CheckGenesisConfByDB(genesisDB, conf3)
	assert.NotNil(t, err)
	assert.Equal(t, err, core.ErrGenesisNotEqualTokenLenInDB)

}
