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

package core

import (
	"testing"

	"time"

	"github.com/alexlisong/go-nebulas/crypto"
	"github.com/alexlisong/go-nebulas/crypto/keystore"
	"github.com/alexlisong/go-nebulas/crypto/keystore/secp256k1"
	"github.com/alexlisong/go-nebulas/util"
	"github.com/stretchr/testify/assert"
)

func TestTransactionPool_1(t *testing.T) {
	ks := keystore.DefaultKS
	priv1 := secp256k1.GeneratePrivateKey()
	pubdata1, _ := priv1.PublicKey().Encoded()
	from, _ := NewAddressFromPublicKey(pubdata1)
	ks.SetKey(from.String(), priv1, []byte("passphrase"))
	ks.Unlock(from.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key1, _ := ks.GetUnlocked(from.String())
	signature1, _ := crypto.NewSignature(keystore.SECP256K1)
	signature1.InitSign(key1.(keystore.PrivateKey))

	priv2 := secp256k1.GeneratePrivateKey()
	pubdata2, _ := priv2.PublicKey().Encoded()
	other, _ := NewAddressFromPublicKey(pubdata2)
	ks.SetKey(other.String(), priv2, []byte("passphrase"))
	ks.Unlock(other.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key2, _ := ks.GetUnlocked(other.String())
	signature2, _ := crypto.NewSignature(keystore.SECP256K1)
	signature2.InitSign(key2.(keystore.PrivateKey))

	priv3 := secp256k1.GeneratePrivateKey()
	pubdata3, _ := priv3.PublicKey().Encoded()
	other2, _ := NewAddressFromPublicKey(pubdata3)
	ks.SetKey(other2.String(), priv3, []byte("passphrase"))
	ks.Unlock(other2.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key3, _ := ks.GetUnlocked(other2.String())
	signature3, _ := crypto.NewSignature(keystore.SECP256K1)
	signature3.InitSign(key3.(keystore.PrivateKey))

	priv4 := secp256k1.GeneratePrivateKey()
	pubdata4, _ := priv4.PublicKey().Encoded()
	other3, _ := NewAddressFromPublicKey(pubdata4)
	ks.SetKey(other3.String(), priv4, []byte("passphrase"))
	ks.Unlock(other3.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key4, _ := ks.GetUnlocked(other3.String())
	signature4, _ := crypto.NewSignature(keystore.SECP256K1)
	signature4.InitSign(key4.(keystore.PrivateKey))

	gasCount, _ := util.NewUint128FromInt(2)
	heighPrice, err := TransactionGasPrice.Mul(gasCount)
	assert.Nil(t, err)

	bc := testNeb(t).chain
	txPool, _ := NewTransactionPool(3)
	txPool.setBlockChain(bc)
	txPool.setEventEmitter(bc.eventEmitter)

	gasLimit, _ := util.NewUint128FromInt(200000)
	tx1, _ := NewTransaction(bc.ChainID(), from, &Address{address: []byte("to")}, util.NewUint128(), 10, TxPayloadBinaryType, []byte("1"), TransactionGasPrice, gasLimit)
	tx2, _ := NewTransaction(bc.ChainID(), other, &Address{address: []byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("2"), heighPrice, gasLimit)
	tx3, _ := NewTransaction(bc.ChainID(), from, &Address{address: []byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("3"), TransactionGasPrice, gasLimit)

	tx4, _ := NewTransaction(bc.ChainID(), from, &Address{address: []byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("4"), TransactionGasPrice, gasLimit)
	tx5, _ := NewTransaction(bc.ChainID()+1, from, &Address{address: []byte("to")}, util.NewUint128(), 0, TxPayloadBinaryType, []byte("5"), TransactionGasPrice, gasLimit)

	tx6, _ := NewTransaction(bc.ChainID(), other2, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("6"), TransactionGasPrice, gasLimit)
	tx7, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("7"), heighPrice, gasLimit)

	tx8, _ := NewTransaction(bc.ChainID(), other3, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("8"), heighPrice, gasLimit)

	txs := []*Transaction{tx1, tx2, tx3, tx4, tx5, tx6, tx7, tx8}

	assert.Nil(t, txs[0].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[0]))
	// put dup tx, should fail
	assert.NotNil(t, txPool.Push(txs[0]))
	assert.Nil(t, txs[1].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[1]))
	assert.Nil(t, txs[2].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[2]))
	// put not signed tx, should fail
	assert.NotNil(t, txPool.Push(txs[3]))
	// push 3, full, drop 0
	assert.Equal(t, len(txPool.all), 3)
	assert.NotNil(t, txPool.all[txs[0].hash.Hex()])
	assert.Nil(t, txs[3].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[3]))
	assert.Nil(t, txPool.all[txs[0].hash.Hex()])
	assert.Equal(t, len(txPool.all), 3)
	// pop 1
	tx := txPool.Pop()
	assert.Equal(t, txs[1].data, tx.data)
	// put tx with different chainID, should fail
	assert.Nil(t, txs[4].Sign(signature1))
	assert.NotNil(t, txPool.Push(txs[4]))
	// put one new
	assert.Equal(t, len(txPool.all), 2)
	assert.Nil(t, txs[5].Sign(signature3))
	assert.Nil(t, txPool.Push(txs[5]))
	assert.Equal(t, len(txPool.all), 3)
	// put one new, full, pop 3
	assert.Equal(t, len(txPool.all), 3)
	assert.NotNil(t, txPool.all[txs[3].hash.Hex()])
	assert.Nil(t, txs[6].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[6]))
	assert.Nil(t, txPool.all[txs[3].hash.Hex()])
	assert.Equal(t, len(txPool.all), 3)

	assert.Equal(t, len(txPool.all), 3)
	assert.Nil(t, txs[7].Sign(signature4))
	assert.Nil(t, txPool.Push(txs[7]))
	assert.Equal(t, len(txPool.all), 3)

	assert.NotNil(t, txPool.Pop())
	assert.Equal(t, len(txPool.all), 2)
	assert.NotNil(t, txPool.Pop())
	assert.Equal(t, len(txPool.all), 1)
	assert.NotNil(t, txPool.Pop())
	assert.Equal(t, len(txPool.all), 0)
	assert.Equal(t, txPool.Empty(), true)
	assert.Nil(t, txPool.Pop())
}

func TestTransactionPool(t *testing.T) {
	ks := keystore.DefaultKS
	priv1 := secp256k1.GeneratePrivateKey()
	pubdata1, _ := priv1.PublicKey().Encoded()
	from, _ := NewAddressFromPublicKey(pubdata1)
	ks.SetKey(from.String(), priv1, []byte("passphrase"))
	ks.Unlock(from.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key1, _ := ks.GetUnlocked(from.String())
	signature1, _ := crypto.NewSignature(keystore.SECP256K1)
	signature1.InitSign(key1.(keystore.PrivateKey))

	priv2 := secp256k1.GeneratePrivateKey()
	pubdata2, _ := priv2.PublicKey().Encoded()
	other, _ := NewAddressFromPublicKey(pubdata2)
	ks.SetKey(other.String(), priv2, []byte("passphrase"))
	ks.Unlock(other.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key2, _ := ks.GetUnlocked(other.String())
	signature2, _ := crypto.NewSignature(keystore.SECP256K1)
	signature2.InitSign(key2.(keystore.PrivateKey))
	gasCount, _ := util.NewUint128FromInt(2)
	heighPrice, err := TransactionGasPrice.Mul(gasCount)
	assert.Nil(t, err)
	neb := testNeb(t)
	bc := neb.chain
	txPool := bc.txPool

	gasLimit, _ := util.NewUint128FromInt(200000)

	tx1, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 10, TxPayloadBinaryType, []byte("1"), TransactionGasPrice, gasLimit)
	tx2, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("2"), TransactionGasPrice, gasLimit)
	tx3, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("3"), TransactionGasPrice, gasLimit)
	tx4, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("4"), TransactionGasPrice, gasLimit)
	tx5, _ := NewTransaction(bc.ChainID()+1, from, &Address{[]byte("to")}, util.NewUint128(), 0, TxPayloadBinaryType, []byte("5"), TransactionGasPrice, gasLimit)
	tx6, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("6"), heighPrice, gasLimit)
	tx7, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("7"), heighPrice, gasLimit)

	txs := []*Transaction{tx1, tx2, tx3, tx4, tx5, tx6, tx7}

	assert.Nil(t, txs[0].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[0]))
	//put dup tx, should fail
	assert.NotNil(t, txPool.Push(txs[0]))
	assert.Nil(t, txs[1].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[1]))
	assert.Nil(t, txs[2].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[2]))
	// put not signed tx, should fail
	assert.NotNil(t, txPool.Push(txs[3]))
	// put tx with different chainID, should fail
	assert.Nil(t, txs[4].Sign(signature1))
	assert.NotNil(t, txPool.Push(txs[4]))
	// put one new, replace txs[1]
	assert.Equal(t, len(txPool.all), 3)
	assert.Nil(t, txs[6].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[6]))
	assert.Equal(t, len(txPool.all), 4)
	// get from: other, nonce: 1, data: "da"
	tx := txPool.Pop()
	assert.Equal(t, txs[6].data.Payload, tx.data.Payload)
	// put one new
	assert.Equal(t, len(txPool.all), 3)
	assert.Nil(t, txs[5].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[5]))
	assert.Equal(t, len(txPool.all), 4)
	// get 2 txs, txs[5], txs[0]
	tx = txPool.Pop()
	assert.Equal(t, txs[5].from.address, tx.from.address)
	assert.Equal(t, txs[5].Nonce(), tx.Nonce())
	assert.Equal(t, txs[5].data, tx.data)
	assert.Equal(t, txPool.Empty(), false)
	txPool.Pop()
	txPool.Pop()
	txPool.Pop()
	assert.Equal(t, txPool.Empty(), true)
	assert.Nil(t, txPool.Pop())
}

func TestGasConfig(t *testing.T) {
	txPool, _ := NewTransactionPool(3)
	txPool.SetGasConfig(nil, nil)
	assert.Equal(t, txPool.minGasPrice, TransactionGasPrice)
	assert.Equal(t, txPool.maxGasLimit, TransactionMaxGas)
	gasPrice, _ := util.NewUint128FromInt(1)
	gasLimit, _ := util.NewUint128FromInt(1)
	txPool.SetGasConfig(gasPrice, gasLimit)
	assert.Equal(t, txPool.minGasPrice, gasPrice)
	assert.Equal(t, txPool.maxGasLimit, gasLimit)
}

func TestPushTxs(t *testing.T) {
	ks := keystore.DefaultKS
	priv1 := secp256k1.GeneratePrivateKey()
	pubdata1, _ := priv1.PublicKey().Encoded()
	from, _ := NewAddressFromPublicKey(pubdata1)
	ks.SetKey(from.String(), priv1, []byte("passphrase"))
	ks.Unlock(from.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key1, _ := ks.GetUnlocked(from.String())
	signature1, _ := crypto.NewSignature(keystore.SECP256K1)
	signature1.InitSign(key1.(keystore.PrivateKey))

	priv2 := secp256k1.GeneratePrivateKey()
	pubdata2, _ := priv2.PublicKey().Encoded()
	to, _ := NewAddressFromPublicKey(pubdata2)
	ks.SetKey(to.String(), priv2, []byte("passphrase"))
	ks.Unlock(to.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key2, _ := ks.GetUnlocked(to.String())
	signature2, _ := crypto.NewSignature(keystore.SECP256K1)
	signature2.InitSign(key2.(keystore.PrivateKey))

	neb := testNeb(t)
	bc := neb.chain
	txPool := bc.txPool
	txPool.setBlockChain(bc)
	txPool.setEventEmitter(bc.eventEmitter)
	uint128Number1, _ := util.NewUint128FromInt(1)
	MaxGasPlus1, _ := TransactionMaxGas.Add(uint128Number1)
	gasPrice, _ := util.NewUint128FromInt(1000000 - 1)
	tx1, err := NewTransaction(bc.ChainID(), from, to, util.NewUint128(), 10, TxPayloadBinaryType, []byte("datadata"), gasPrice, TransactionMaxGas)
	assert.Nil(t, err)
	_, err = NewTransaction(bc.ChainID(), from, to, util.NewUint128(), 10, TxPayloadBinaryType, []byte("datadata"), TransactionGasPrice, MaxGasPlus1)
	assert.Equal(t, err, ErrInvalidGasLimit)
	txs := []*Transaction{tx1}
	assert.Equal(t, txPool.Push(txs[0]), ErrBelowGasPrice)
}

func TestTransactionPool_Pop(t *testing.T) {
	ks := keystore.DefaultKS
	priv1 := secp256k1.GeneratePrivateKey()
	pubdata1, _ := priv1.PublicKey().Encoded()
	from, _ := NewAddressFromPublicKey(pubdata1)
	ks.SetKey(from.String(), priv1, []byte("passphrase"))
	ks.Unlock(from.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key1, _ := ks.GetUnlocked(from.String())
	signature1, _ := crypto.NewSignature(keystore.SECP256K1)
	signature1.InitSign(key1.(keystore.PrivateKey))

	priv2 := secp256k1.GeneratePrivateKey()
	pubdata2, _ := priv2.PublicKey().Encoded()
	other, _ := NewAddressFromPublicKey(pubdata2)
	ks.SetKey(other.String(), priv2, []byte("passphrase"))
	ks.Unlock(other.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key2, _ := ks.GetUnlocked(other.String())
	signature2, _ := crypto.NewSignature(keystore.SECP256K1)
	signature2.InitSign(key2.(keystore.PrivateKey))

	gasCount, _ := util.NewUint128FromInt(2)
	highPrice, err := TransactionGasPrice.Mul(gasCount)
	assert.Nil(t, err)
	neb := testNeb(t)
	bc := neb.chain
	txPool := bc.txPool

	assert.Equal(t, highPrice.Cmp(TransactionGasPrice), 1)
	gasLimit, _ := util.NewUint128FromInt(200000)
	tx1, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 3, TxPayloadBinaryType, []byte("1"), TransactionGasPrice, gasLimit)
	tx2, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("2"), highPrice, gasLimit)
	tx3, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("3"), TransactionGasPrice, gasLimit)
	tx4, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("4"), TransactionGasPrice, gasLimit)
	tx5, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("5"), highPrice, gasLimit)
	txs := []*Transaction{tx1, tx2, tx3, tx4, tx5}

	assert.Nil(t, txs[0].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[0]))
	assert.Nil(t, txs[1].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[1]))
	assert.Nil(t, txs[2].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[2]))
	assert.Nil(t, txs[3].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[3]))
	assert.Nil(t, txs[4].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[4]))

	tx := txPool.Pop()
	assert.Equal(t, tx.sign, txs[4].sign)
	tx = txPool.Pop()
	assert.Equal(t, tx.sign, txs[1].sign)
	tx = txPool.Pop()
	assert.Equal(t, tx.sign, txs[3].sign)
	tx = txPool.Pop()
	assert.Equal(t, tx.sign, txs[2].sign)
	tx = txPool.Pop()
	assert.Equal(t, tx.sign, txs[0].sign)
}

func TestTransactionPoolBucketUpdateTimeAndEvict(t *testing.T) {
	ks := keystore.DefaultKS
	priv1 := secp256k1.GeneratePrivateKey()
	pubdata1, _ := priv1.PublicKey().Encoded()
	from, _ := NewAddressFromPublicKey(pubdata1)
	ks.SetKey(from.String(), priv1, []byte("passphrase"))
	ks.Unlock(from.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key1, _ := ks.GetUnlocked(from.String())
	signature1, _ := crypto.NewSignature(keystore.SECP256K1)
	signature1.InitSign(key1.(keystore.PrivateKey))

	priv2 := secp256k1.GeneratePrivateKey()
	pubdata2, _ := priv2.PublicKey().Encoded()
	other, _ := NewAddressFromPublicKey(pubdata2)
	ks.SetKey(other.String(), priv2, []byte("passphrase"))
	ks.Unlock(other.String(), []byte("passphrase"), time.Second*60*60*24*365)
	key2, _ := ks.GetUnlocked(other.String())
	signature2, _ := crypto.NewSignature(keystore.SECP256K1)
	signature2.InitSign(key2.(keystore.PrivateKey))

	gasCount, _ := util.NewUint128FromInt(2)
	highPrice, err := TransactionGasPrice.Mul(gasCount)
	assert.Nil(t, err)
	neb := testNeb(t)
	bc := neb.chain
	txPool := bc.txPool

	assert.Equal(t, highPrice.Cmp(TransactionGasPrice), 1)
	gasLimit, _ := util.NewUint128FromInt(200000)
	tx1, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 3, TxPayloadBinaryType, []byte("1"), TransactionGasPrice, gasLimit)
	tx2, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("2"), highPrice, gasLimit)
	tx3, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 2, TxPayloadBinaryType, []byte("3"), TransactionGasPrice, gasLimit)
	tx4, _ := NewTransaction(bc.ChainID(), from, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("4"), TransactionGasPrice, gasLimit)
	tx5, _ := NewTransaction(bc.ChainID(), other, &Address{[]byte("to")}, util.NewUint128(), 1, TxPayloadBinaryType, []byte("5"), highPrice, gasLimit)
	txs := []*Transaction{tx1, tx2, tx3, tx4, tx5}

	assert.Nil(t, txs[0].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[0]))
	assert.Nil(t, txs[1].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[1]))
	assert.Nil(t, txs[2].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[2]))
	assert.Nil(t, txs[3].Sign(signature1))
	assert.Nil(t, txPool.Push(txs[3]))
	assert.Nil(t, txs[4].Sign(signature2))
	assert.Nil(t, txPool.Push(txs[4]))

	// test bucket time is initialized but not updated
	assert.Equal(t, time.Since(txPool.bucketsLastUpdate[txs[0].from.address.Hex()]) < time.Second*5, true)
	assert.Equal(t, txPool.bucketsLastUpdate[txs[0].from.address.Hex()], txPool.bucketsLastUpdate[txs[2].from.address.Hex()])
	assert.Equal(t, txPool.bucketsLastUpdate[txs[0].from.address.Hex()], txPool.bucketsLastUpdate[txs[3].from.address.Hex()])
	assert.Equal(t, time.Since(txPool.bucketsLastUpdate[txs[1].from.address.Hex()]) < time.Second*5, true)
	assert.Equal(t, txPool.bucketsLastUpdate[txs[1].from.address.Hex()], txPool.bucketsLastUpdate[txs[4].from.address.Hex()])
	assert.NotNil(t, txPool.all[txs[0].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[2].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[3].hash.Hex()])

	txPool.bucketsLastUpdate[txs[0].from.address.Hex()] = time.Now().Add(time.Minute * -89)
	txPool.evictExpiredTransactions()
	assert.NotNil(t, txPool.all[txs[0].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[2].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[3].hash.Hex()])
	_, ok := txPool.buckets[txs[0].from.address.Hex()]
	assert.Equal(t, ok, true)
	_, ok = txPool.bucketsLastUpdate[txs[0].from.address.Hex()]
	assert.Equal(t, ok, true)

	txPool.bucketsLastUpdate[txs[0].from.address.Hex()] = time.Now().Add(time.Minute * -91)
	txPool.evictExpiredTransactions()
	assert.Nil(t, txPool.all[txs[0].hash.Hex()])
	assert.Nil(t, txPool.all[txs[2].hash.Hex()])
	assert.Nil(t, txPool.all[txs[3].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[1].hash.Hex()])
	assert.NotNil(t, txPool.all[txs[4].hash.Hex()])

	_, ok = txPool.buckets[txs[0].from.address.Hex()]
	assert.Equal(t, ok, false)
	_, ok = txPool.bucketsLastUpdate[txs[0].from.address.Hex()]
	assert.Equal(t, ok, false)

}
