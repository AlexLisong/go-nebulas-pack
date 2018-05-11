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

package rpc

import (
	"errors"

	"github.com/alexlisong/go-nebulas/storage"
	"github.com/alexlisong/go-nebulas/util/logging"
	"github.com/sirupsen/logrus"

	"encoding/json"

	"github.com/gogo/protobuf/proto"
	"github.com/alexlisong/go-nebulas/core"
	"github.com/alexlisong/go-nebulas/core/pb"
	"github.com/alexlisong/go-nebulas/net"
	"github.com/alexlisong/go-nebulas/rpc/pb"
	"github.com/alexlisong/go-nebulas/util"
	"github.com/alexlisong/go-nebulas/util/byteutils"
	"golang.org/x/net/context"
)

//the max number of block can be dumped once
const maxDumpBlockCount = 10

// APIService implements the RPC API service interface.
type APIService struct {
	server GRPCServer
}

// GetNebState is the RPC API handler.
func (s *APIService) GetNebState(ctx context.Context, req *rpcpb.NonParamsRequest) (*rpcpb.GetNebStateResponse, error) {

	neb := s.server.Neblet()

	tail := neb.BlockChain().TailBlock()
	lib := neb.BlockChain().LIB()

	resp := &rpcpb.GetNebStateResponse{}
	resp.ChainId = neb.BlockChain().ChainID()
	resp.Tail = tail.Hash().String()
	resp.Lib = lib.Hash().String()
	resp.Height = tail.Height()
	resp.Synchronized = neb.IsActiveSyncing()
	resp.ProtocolVersion = net.NebProtocolID
	resp.Version = neb.Config().App.Version

	return resp, nil
}

// GetAccountState is the RPC API handler.
func (s *APIService) GetAccountState(ctx context.Context, req *rpcpb.GetAccountStateRequest) (*rpcpb.GetAccountStateResponse, error) {

	neb := s.server.Neblet()

	addr, err := core.AddressParse(req.Address)
	if err != nil {
		metricsAccountStateFailed.Mark(1)
		return nil, err
	}

	block := neb.BlockChain().TailBlock()
	if req.Height > 0 {
		block = neb.BlockChain().GetBlockOnCanonicalChainByHeight(req.Height)
		if block == nil {
			metricsAccountStateFailed.Mark(1)
			return nil, errors.New("block not found")
		}
	}

	acc, err := block.GetAccount(addr.Bytes())
	if err != nil {
		return nil, err
	}

	metricsAccountStateSuccess.Mark(1)
	return &rpcpb.GetAccountStateResponse{Balance: acc.Balance().String(), Nonce: acc.Nonce(), Type: uint32(addr.Type())}, nil
}

// Call is the RPC API handler.
func (s *APIService) Call(ctx context.Context, req *rpcpb.TransactionRequest) (*rpcpb.CallResponse, error) {
	neb := s.server.Neblet()
	tx, err := parseTransaction(neb, req)
	if err != nil {
		return nil, err
	}

	result, err := neb.BlockChain().SimulateTransactionExecution(tx)
	if err != nil {
		return nil, err
	}

	errMsg := ""
	if result.Err != nil {
		errMsg = result.Err.Error()
	}

	return &rpcpb.CallResponse{
		Result:      result.Msg,
		ExecuteErr:  errMsg,
		EstimateGas: result.GasUsed.String(),
	}, nil
}

func parseTransaction(neb core.Neblet, reqTx *rpcpb.TransactionRequest) (*core.Transaction, error) {
	fromAddr, err := core.AddressParse(reqTx.From)
	if err != nil {
		return nil, err
	}
	toAddr, err := core.AddressParse(reqTx.To)
	if err != nil {
		return nil, err
	}

	value, err := util.NewUint128FromString(reqTx.Value)
	if err != nil {
		return nil, errors.New("invalid value")
	}
	gasPrice, err := util.NewUint128FromString(reqTx.GasPrice)
	if err != nil {
		return nil, errors.New("invalid gasPrice")
	}
	gasLimit, err := util.NewUint128FromString(reqTx.GasLimit)
	if err != nil {
		return nil, errors.New("invalid gasLimit")
	}
	var (
		payloadType string
		payload     []byte
	)

	if reqTx.Contract != nil {
		if len(reqTx.Contract.Source) > 0 && len(reqTx.Contract.Function) == 0 { // TODO: reqTx.DeployContract, reqTx.CallContract
			payloadType = core.TxPayloadDeployType
			payloadObj, err := core.NewDeployPayload(reqTx.Contract.Source, reqTx.Contract.SourceType, reqTx.Contract.Args)
			if err != nil {
				return nil, err
			}
			if payload, err = payloadObj.ToBytes(); err != nil {
				return nil, err
			}
		} else if len(reqTx.Contract.Source) == 0 && len(reqTx.Contract.Function) > 0 {
			payloadType = core.TxPayloadCallType
			callpayload, err := core.NewCallPayload(reqTx.Contract.Function, reqTx.Contract.Args)
			if err != nil {
				return nil, err
			}

			if payload, err = callpayload.ToBytes(); err != nil {
				return nil, err
			}
		} else {
			return nil, errors.New("invalid contract")
		}
	} else {
		payloadType = core.TxPayloadBinaryType
		if payload, err = core.NewBinaryPayload(reqTx.Binary).ToBytes(); err != nil {
			return nil, err
		}
	}

	tx, err := core.NewTransaction(neb.BlockChain().ChainID(), fromAddr, toAddr, value, reqTx.Nonce, payloadType, payload, gasPrice, gasLimit)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func handleTransactionResponse(neb core.Neblet, tx *core.Transaction) (resp *rpcpb.SendTransactionResponse, err error) {
	defer func() {
		if err != nil {
			metricsSendTxFailed.Mark(1)
		} else {
			metricsSendTxSuccess.Mark(1)
		}
	}()

	err = tx.VerifyIntegrity(neb.BlockChain().ChainID())
	if err != nil {
		return nil, err
	}

	tailBlock := neb.BlockChain().TailBlock()
	acc, err := tailBlock.GetAccount(tx.From().Bytes())
	if err != nil {
		return nil, err
	}

	if tx.Nonce() <= acc.Nonce() {
		return nil, errors.New("transaction's nonce is invalid, should bigger than the from's nonce")
	}

	if tx.Type() == core.TxPayloadDeployType {
		if !tx.From().Equals(tx.To()) {
			return nil, core.ErrContractTransactionAddressNotEqual
		}
	} else if tx.Type() == core.TxPayloadCallType {
		if _, err := tailBlock.CheckContract(tx.To()); err != nil {
			return nil, err
		}
	}

	// push and broadcast tx
	if err := neb.BlockChain().TransactionPool().PushAndBroadcast(tx); err != nil {
		return nil, err
	}

	var contract string
	if tx.Type() == core.TxPayloadDeployType {
		addr, err := core.NewContractAddressFromData(tx.From().Bytes(), byteutils.FromUint64(tx.Nonce()))
		if err != nil {
			return nil, err
		}
		contract = addr.String()
	}

	return &rpcpb.SendTransactionResponse{Txhash: tx.Hash().String(), ContractAddress: contract}, nil
}

// SendRawTransaction submit the signed transaction raw data to txpool
func (s *APIService) SendRawTransaction(ctx context.Context, req *rpcpb.SendRawTransactionRequest) (*rpcpb.SendTransactionResponse, error) {

	// Validate and sign the tx, then submit it to the tx pool.
	neb := s.server.Neblet()

	pbTx := new(corepb.Transaction)
	if err := proto.Unmarshal(req.GetData(), pbTx); err != nil {
		metricsSendTxFailed.Mark(1)
		return nil, err
	}
	tx := new(core.Transaction)
	if err := tx.FromProto(pbTx); err != nil {
		metricsSendTxFailed.Mark(1)
		return nil, err
	}

	return handleTransactionResponse(neb, tx)
}

// GetBlockByHash get block info by the block hash
func (s *APIService) GetBlockByHash(ctx context.Context, req *rpcpb.GetBlockByHashRequest) (*rpcpb.BlockResponse, error) {

	neb := s.server.Neblet()

	bhash, err := byteutils.FromHex(req.GetHash())
	if err != nil {
		return nil, err
	}
	block := neb.BlockChain().GetBlockOnCanonicalChainByHash(bhash)

	return s.toBlockResponse(block, req.FullFillTransaction)
}

// GetBlockByHeight get block info by the block hash
func (s *APIService) GetBlockByHeight(ctx context.Context, req *rpcpb.GetBlockByHeightRequest) (*rpcpb.BlockResponse, error) {

	neb := s.server.Neblet()

	block := neb.BlockChain().GetBlockOnCanonicalChainByHeight(req.Height)

	return s.toBlockResponse(block, req.FullFillTransaction)
}

func (s *APIService) toBlockResponse(block *core.Block, fullFillTransaction bool) (*rpcpb.BlockResponse, error) {
	if block == nil {
		return nil, errors.New("block not found")
	}
	neb := s.server.Neblet()
	lib := neb.BlockChain().LIB()

	isFinality := false
	if lib.Height() > block.Height() {
		isFinality = true
	}
	resp := &rpcpb.BlockResponse{
		Hash:          block.Hash().String(),
		ParentHash:    block.ParentHash().String(),
		Height:        block.Height(),
		Coinbase:      block.Coinbase().String(),
		Timestamp:     block.Timestamp(),
		ChainId:       block.ChainID(),
		StateRoot:     block.StateRoot().String(),
		TxsRoot:       block.TxsRoot().String(),
		EventsRoot:    block.EventsRoot().String(),
		ConsensusRoot: block.ConsensusRoot(),
		Miner:         byteutils.Hash(block.ConsensusRoot().Proposer).Base58(),
		IsFinality:    isFinality,
	}

	// add block transactions
	txs := []*rpcpb.TransactionResponse{}
	for _, v := range block.Transactions() {
		var tx *rpcpb.TransactionResponse
		if fullFillTransaction {
			tx, _ = s.toTransactionResponse(v)
		} else {
			tx = &rpcpb.TransactionResponse{Hash: v.Hash().String()}
		}
		txs = append(txs, tx)
	}
	resp.Transactions = txs

	return resp, nil
}

// LatestIrreversibleBlock is the RPC API handler.
func (s *APIService) LatestIrreversibleBlock(ctx context.Context, req *rpcpb.NonParamsRequest) (*rpcpb.BlockResponse, error) {

	neb := s.server.Neblet()
	block := neb.BlockChain().LIB()

	return s.toBlockResponse(block, false)
}

// GetTransactionReceipt get transaction info by the transaction hash
func (s *APIService) GetTransactionReceipt(ctx context.Context, req *rpcpb.GetTransactionByHashRequest) (*rpcpb.TransactionResponse, error) {

	neb := s.server.Neblet()
	hash, err := byteutils.FromHex(req.GetHash())
	if err != nil {
		return nil, err
	}
	tx, err := neb.BlockChain().GetTransaction(hash)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}

	// if tx is nil, check it in transaction pool.
	if tx == nil {
		tx = neb.BlockChain().TransactionPool().GetTransaction(hash) // TODO: @roy @fengzi make tx pending when collecttxs
		if tx == nil {
			return nil, errors.New("transaction not found")
		}
	}

	return s.toTransactionResponse(tx)
}

func (s *APIService) toTransactionResponse(tx *core.Transaction) (*rpcpb.TransactionResponse, error) {
	var (
		status  int32
		gasUsed string
	)
	neb := s.server.Neblet()
	event, err := neb.BlockChain().TailBlock().FetchExecutionResultEvent(tx.Hash())
	if err != nil && err != core.ErrNotFoundTransactionResultEvent {
		return nil, err
	}

	if event != nil {
		txEvent := core.TransactionEvent{}
		err := json.Unmarshal([]byte(event.Data), &txEvent)
		if err != nil {
			return nil, err
		}
		status = int32(txEvent.Status)
		gasUsed = txEvent.GasUsed
	} else {
		status = core.TxExecutionPendding
	}

	resp := &rpcpb.TransactionResponse{
		ChainId:   tx.ChainID(),
		Hash:      tx.Hash().String(),
		From:      tx.From().String(),
		To:        tx.To().String(),
		Value:     tx.Value().String(),
		Nonce:     tx.Nonce(),
		Timestamp: tx.Timestamp(),
		Type:      tx.Type(),
		Data:      tx.Data(),
		GasPrice:  tx.GasPrice().String(),
		GasLimit:  tx.GasLimit().String(),
		Status:    status,
		GasUsed:   gasUsed,
	}

	if tx.Type() == core.TxPayloadDeployType {
		contractAddr, err := tx.GenerateContractAddress()
		if err != nil {
			return nil, err
		}
		resp.ContractAddress = contractAddr.String()
	}
	return resp, nil
}

// Subscribe ..
func (s *APIService) Subscribe(req *rpcpb.SubscribeRequest, gs rpcpb.ApiService_SubscribeServer) error {

	neb := s.server.Neblet()

	eventSub := core.NewEventSubscriber(1024, req.Topics)
	neb.EventEmitter().Register(eventSub)
	defer neb.EventEmitter().Deregister(eventSub)

	var err error
	for {
		select {
		case <-gs.Context().Done():
			return gs.Context().Err()
		case event := <-eventSub.EventChan():
			err = gs.Send(&rpcpb.SubscribeResponse{Topic: event.Topic, Data: event.Data})
			if err != nil {
				return err
			}
		}
	}
}

// GetGasPrice get gas price from chain.
func (s *APIService) GetGasPrice(ctx context.Context, req *rpcpb.NonParamsRequest) (*rpcpb.GasPriceResponse, error) {
	neb := s.server.Neblet()
	gasPrice := neb.BlockChain().GasPrice()
	return &rpcpb.GasPriceResponse{GasPrice: gasPrice.String()}, nil
}

// EstimateGas Compute the smart contract gas consumption.
func (s *APIService) EstimateGas(ctx context.Context, req *rpcpb.TransactionRequest) (*rpcpb.GasResponse, error) {
	neb := s.server.Neblet()
	tx, err := parseTransaction(neb, req)
	if err != nil {
		return nil, err
	}

	result, err := neb.BlockChain().SimulateTransactionExecution(tx)
	if err != nil {
		return nil, err
	}

	errMsg := ""
	if result.Err != nil {
		errMsg = result.Err.Error()
	}
	return &rpcpb.GasResponse{Gas: result.GasUsed.String(), Err: errMsg}, nil
}

// GetEventsByHash return events by tx hash.
func (s *APIService) GetEventsByHash(ctx context.Context, req *rpcpb.HashRequest) (*rpcpb.EventsResponse, error) {
	neb := s.server.Neblet()

	if len(req.Hash) == 0 {
		return nil, errors.New("please input valid hash")
	}

	txhash, err := byteutils.FromHex(req.Hash)
	if err != nil {
		return nil, err
	}

	tailBlock := neb.BlockChain().TailBlock()
	tx, err := tailBlock.GetTransaction(txhash)
	if err != nil {
		return nil, err
	}

	result, err := tailBlock.FetchEvents(tx.Hash())
	if err != nil {
		return nil, err
	}

	events := make([]*rpcpb.Event, len(result))
	for idx, v := range result {
		event := &rpcpb.Event{Topic: v.Topic, Data: v.Data}
		events[idx] = event
	}

	return &rpcpb.EventsResponse{Events: events}, nil
}

// GetDynasty is the RPC API handler.
func (s *APIService) GetDynasty(ctx context.Context, req *rpcpb.ByBlockHeightRequest) (*rpcpb.GetDynastyResponse, error) {
	neb := s.server.Neblet()

	block := neb.BlockChain().TailBlock()
	if req.Height > 0 {
		block = neb.BlockChain().GetBlockOnCanonicalChainByHeight(req.Height)
		if block == nil {
			return nil, errors.New("block not found")
		}
	}

	miners, err := block.Dynasty()
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, v := range miners {
		addr, err := core.AddressParseFromBytes(v)
		if err != nil {
			logging.VLog().WithFields(logrus.Fields{
				"miner": v.Base58(),
				"block": block,
			}).Debug("Failed to parse miner's bytes into address")
			return nil, err
		}
		result = append(result, addr.String())
	}
	return &rpcpb.GetDynastyResponse{Miners: result}, nil
}
