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
	"encoding/json"
	"fmt"

	"github.com/alexlisong/go-nebulas/util"
)

// CallPayload carry function call information
type CallPayload struct {
	Function string
	Args     string
}

// LoadCallPayload from bytes
func LoadCallPayload(bytes []byte) (*CallPayload, error) {
	payload := &CallPayload{}
	if err := json.Unmarshal(bytes, payload); err != nil {
		return nil, ErrInvalidArgument
	}
	return NewCallPayload(payload.Function, payload.Args)
}

// NewCallPayload with function & args
func NewCallPayload(function, args string) (*CallPayload, error) {

	if PublicFuncNameChecker.MatchString(function) == false {
		return nil, ErrInvalidCallFunction
	}

	if err := CheckContractArgs(args); err != nil {
		return nil, ErrInvalidArgument
	}

	return &CallPayload{
		Function: function,
		Args:     args,
	}, nil
}

// ToBytes serialize payload
func (payload *CallPayload) ToBytes() ([]byte, error) {
	return json.Marshal(payload)
}

// BaseGasCount returns base gas count
func (payload *CallPayload) BaseGasCount() *util.Uint128 {
	base, _ := util.NewUint128FromInt(60)
	return base
}

// Execute the call payload in tx, call a function
func (payload *CallPayload) Execute(limitedGas *util.Uint128, tx *Transaction, block *Block, ws WorldState) (*util.Uint128, string, error) {
	if block == nil || tx == nil {
		return util.NewUint128(), "", ErrNilArgument
	}

	// payloadGasLimit <= 0, v8 engine not limit the execution instructions
	if limitedGas.Cmp(util.NewUint128()) <= 0 {
		return util.NewUint128(), "", ErrOutOfGasLimit
	}

	// contract address is tx.to.
	contract, err := CheckContract(tx.to, ws)
	if err != nil {
		return util.NewUint128(), "", err
	}

	birthTx, err := GetTransaction(contract.BirthPlace(), ws)
	if err != nil {
		return util.NewUint128(), "", err
	}
	/* // useless owner.
	owner, err := ws.GetOrCreateUserAccount(birthTx.from.Bytes())
	if err != nil {
		return util.NewUint128(), "", err
	} */
	deploy, err := LoadDeployPayload(birthTx.data.Payload) // ToConfirm: move deploy payload in ctx.
	if err != nil {
		return util.NewUint128(), "", err
	}

	engine, err := block.nvm.CreateEngine(block, tx, contract, ws)
	if err != nil {
		return util.NewUint128(), "", err
	}
	defer engine.Dispose()

	if err := engine.SetExecutionLimits(limitedGas.Uint64(), DefaultLimitsOfTotalMemorySize); err != nil {
		return util.NewUint128(), "", err
	}

	result, exeErr := engine.Call(deploy.Source, deploy.SourceType, payload.Function, payload.Args)
	gasCout := engine.ExecutionInstructions()
	instructions, err := util.NewUint128FromInt(int64(gasCout))
	if err != nil {
		return util.NewUint128(), "", err
	}
	if exeErr != nil && exeErr == ErrExecutionFailed && len(result) > 0 {
		exeErr = fmt.Errorf("Call: %s", result)
	}
	return instructions, result, exeErr
}
