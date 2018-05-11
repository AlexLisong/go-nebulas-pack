package storage

import (
	"sync"
	"time"

	"github.com/alexlisong/go-nebulas/util/byteutils"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/tecbot/gorocksdb"
)

// RocksStorage the nodes in trie.
type RocksStorage struct {
	db          *gorocksdb.DB
	enableBatch bool
	mutex       sync.Mutex
	batchOpts   map[string]*batchOpt

	ro *gorocksdb.ReadOptions
	wo *gorocksdb.WriteOptions
}

// NewRocksStorage init a storage
func NewRocksStorage(path string) (*RocksStorage, error) {

	filter := gorocksdb.NewBloomFilter(10)
	bbto := gorocksdb.NewDefaultBlockBasedTableOptions()
	bbto.SetFilterPolicy(filter)
	bbto.SetBlockCache(gorocksdb.NewLRUCache(512 << 20))
	opts := gorocksdb.NewDefaultOptions()
	opts.SetBlockBasedTableFactory(bbto)
	opts.SetCreateIfMissing(true)
	opts.SetMaxOpenFiles(500)
	opts.SetWriteBufferSize(64 * opt.MiB) //Default: 4MB
	opts.IncreaseParallelism(4)           //flush and compaction thread

	db, err := gorocksdb.OpenDb(opts, path)
	if err != nil {
		return nil, err
	}

	return &RocksStorage{
		db:          db,
		enableBatch: false,
		batchOpts:   make(map[string]*batchOpt),
		ro:          gorocksdb.NewDefaultReadOptions(),
		wo:          gorocksdb.NewDefaultWriteOptions(),
	}, nil
}

// Get return value to the key in Storage
func (storage *RocksStorage) Get(key []byte) ([]byte, error) {

	value, err := storage.db.GetBytes(storage.ro, key)

	if err != nil {
		return nil, err
	}

	if value == nil {
		return nil, ErrKeyNotFound
	}

	return value, err
}

// Put put the key-value entry to Storage
func (storage *RocksStorage) Put(key []byte, value []byte) error {
	if storage.enableBatch {
		storage.mutex.Lock()
		defer storage.mutex.Unlock()

		storage.batchOpts[byteutils.Hex(key)] = &batchOpt{
			key:     key,
			value:   value,
			deleted: false,
		}

		return nil
	}

	return storage.db.Put(storage.wo, key, value)
}

// Del delete the key in Storage.
func (storage *RocksStorage) Del(key []byte) error {
	if storage.enableBatch {
		storage.mutex.Lock()
		defer storage.mutex.Unlock()

		storage.batchOpts[byteutils.Hex(key)] = &batchOpt{
			key:     key,
			deleted: true,
		}

		return nil
	}
	return storage.db.Delete(storage.wo, key)
}

// Close levelDB
func (storage *RocksStorage) Close() error {
	storage.db.Close()
	return nil
}

// EnableBatch enable batch write.
func (storage *RocksStorage) EnableBatch() {
	storage.enableBatch = true
}

// Flush write and flush pending batch write.
func (storage *RocksStorage) Flush() error {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	if !storage.enableBatch {
		return nil
	}

	startAt := time.Now().UnixNano()

	wb := gorocksdb.NewWriteBatch()
	defer wb.Destroy()

	bl := len(storage.batchOpts)

	for _, opt := range storage.batchOpts {
		if opt.deleted {
			wb.Delete(opt.key)
		} else {
			wb.Put(opt.key, opt.value)
		}
	}
	storage.batchOpts = make(map[string]*batchOpt)

	err := storage.db.Write(storage.wo, wb)

	endAt := time.Now().UnixNano()
	metricsRocksdbFlushTime.Update(endAt - startAt)
	metricsRocksdbFlushLen.Update(int64(bl))

	return err
}

// DisableBatch disable batch write.
func (storage *RocksStorage) DisableBatch() {
	storage.mutex.Lock()
	defer storage.mutex.Unlock()
	storage.batchOpts = make(map[string]*batchOpt)

	storage.enableBatch = false
}
