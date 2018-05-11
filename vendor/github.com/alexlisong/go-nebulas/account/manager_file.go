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

package account

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"strings"

	"errors"
	"os"

	"github.com/alexlisong/go-nebulas/core"
	"github.com/alexlisong/go-nebulas/util"
	"github.com/alexlisong/go-nebulas/util/logging"
	"github.com/sirupsen/logrus"
)

type account struct {

	// key address
	addr *core.Address

	// key save path
	path string
}

// refreshAccounts sync key files to memory
func (m *Manager) refreshAccounts() error {
	files, err := ioutil.ReadDir(m.keydir)
	if err != nil {
		return err
	}
	var (
		accounts []*account
	)

	for _, file := range files {

		acc, err := m.loadKeyFile(file)
		if err != nil {
			// errors have been recorded
			continue
		}
		accounts = append(accounts, acc)
	}
	m.accounts = accounts
	return nil
}

func (m *Manager) loadKeyFile(file os.FileInfo) (*account, error) {
	var (
		keyJSON struct {
			Address string `json:"address"`
		}
	)

	path := filepath.Join(m.keydir, file.Name())

	if file.IsDir() || strings.HasPrefix(file.Name(), ".") || strings.HasSuffix(file.Name(), "~") {
		logging.VLog().WithFields(logrus.Fields{
			"path": path,
		}).Warn("Skipped this key file.")
		return nil, errors.New("file need skip")
	}

	raw, err := ioutil.ReadFile(path)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"err":  err,
			"path": path,
		}).Error("Failed to read the key file.")
		return nil, errors.New("failed to read the key file")
	}

	keyJSON.Address = ""
	err = json.Unmarshal(raw, &keyJSON)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"err":  err,
			"path": path,
		}).Error("Failed to parse the key file.")
		return nil, errors.New("failed to parse the key file")
	}

	addr, err := core.AddressParse(keyJSON.Address)
	if err != nil {
		logging.VLog().WithFields(logrus.Fields{
			"err":     err,
			"address": keyJSON.Address,
		}).Error("Failed to parse the address.")
		return nil, errors.New("failed to parse the address")
	}

	acc := &account{addr, path}
	return acc, nil
}

// loadFile import key to keystore in keydir
func (m *Manager) loadFile(addr *core.Address, passphrase []byte) error {
	acc, err := m.getAccount(addr)
	if err != nil {
		return err
	}

	raw, err := ioutil.ReadFile(acc.path)
	if err != nil {
		return err
	}
	_, err = m.Load(raw, passphrase)
	return err
}

func (m *Manager) exportFile(addr *core.Address, passphrase []byte, overwrite bool) (path string, err error) {
	raw, err := m.Export(addr, passphrase)
	if err != nil {
		return "", err
	}

	acc, err := m.getAccount(addr)
	// acc not found
	if err != nil {
		path = filepath.Join(m.keydir, addr.String())
	} else {
		path = acc.path
	}
	if err := util.FileWrite(path, raw, overwrite); err != nil {
		return "", err
	}
	return path, nil
}

func (m *Manager) getAccount(addr *core.Address) (*account, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, acc := range m.accounts {
		if acc.addr.Equals(addr) {
			return acc, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *Manager) updateAccount(addr *core.Address, path string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var target *account
	for _, acc := range m.accounts {
		if acc.addr.Equals(addr) {
			target = acc
			break
		}
	}
	if target != nil {
		target.path = path
	} else {
		target = &account{addr: addr, path: path}
		m.accounts = append(m.accounts, target)
	}
}
