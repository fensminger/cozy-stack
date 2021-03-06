package vfs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDownloadStoreInMemory(t *testing.T) {
	downloadStoreTTL = 100 * time.Millisecond

	domainA := "alice.cozycloud.local"
	domainB := "bob.cozycloud.local"
	store := newMemStore()

	path := "/test/random/path.txt"
	key1, err := store.AddFile(domainA, path)
	assert.NoError(t, err)

	path2, err := store.GetFile(domainB, key1)
	assert.NoError(t, err)
	assert.Zero(t, path2, "Inter-instances store leaking")

	path3, err := store.GetFile(domainA, key1)
	assert.NoError(t, err)
	assert.Equal(t, path, path3)

	time.Sleep(2 * downloadStoreTTL)

	path4, err := store.GetFile(domainA, key1)
	assert.NoError(t, err)
	assert.Zero(t, path4, "no expiration")

	a := &Archive{
		Name: "test",
		Files: []string{
			"/archive/foo.jpg",
			"/archive/bar",
		},
	}
	key2, err := store.AddArchive(domainA, a)
	assert.NoError(t, err)

	a2, err := store.GetArchive(domainA, key2)
	assert.NoError(t, err)
	assert.Equal(t, a, a2)

	time.Sleep(2 * downloadStoreTTL)

	a3, err := store.GetArchive(domainA, key2)
	assert.NoError(t, err)
	assert.Nil(t, a3, "no expiration")
}

func TestDownloadStoreInRedis(t *testing.T) {
	downloadStoreTTL = 100 * time.Millisecond

	domainA := "alice.cozycloud.local"
	domainB := "bob.cozycloud.local"
	store := GetStore()

	path := "/test/random/path.txt"
	key1, err := store.AddFile(domainA, path)
	assert.NoError(t, err)

	path2, err := store.GetFile(domainB, key1)
	assert.NoError(t, err)
	assert.Zero(t, path2, "Inter-instances store leaking")

	path3, err := store.GetFile(domainA, key1)
	assert.NoError(t, err)
	assert.Equal(t, path, path3)

	time.Sleep(2 * downloadStoreTTL)

	path4, err := store.GetFile(domainA, key1)
	assert.NoError(t, err)
	assert.Zero(t, path4, "no expiration")

	a := &Archive{
		Name: "test",
		Files: []string{
			"/archive/foo.jpg",
			"/archive/bar",
		},
	}
	key2, err := store.AddArchive(domainA, a)
	assert.NoError(t, err)

	a2, err := store.GetArchive(domainA, key2)
	assert.NoError(t, err)
	assert.Equal(t, a, a2)

	time.Sleep(2 * downloadStoreTTL)

	a3, err := store.GetArchive(domainA, key2)
	assert.NoError(t, err)
	assert.Nil(t, a3, "no expiration")
}
